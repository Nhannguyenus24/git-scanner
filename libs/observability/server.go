package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server is the health listener. Services without their own HTTP surface still
// run one, because Kubernetes probes and KEDA-driven rollouts depend on it.
type Server struct {
	srv    *http.Server
	log    *slog.Logger
	health *Health
}

// NewServer builds the probe listener. extra, when non-nil, serves every path
// the probes do not, so a service can mount its routes on the same port.
func NewServer(addr string, log *slog.Logger, health *Health, extra http.Handler) *Server {
	s := &Server{log: log, health: health}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleLive)
	mux.HandleFunc("GET /readyz", s.handleReady)
	if extra != nil {
		mux.Handle("/", extra)
	}

	s.srv = &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// corsMiddleware adds permissive CORS headers so browser-based API explorers
// (e.g. the GitScanner API artifact on claude.ai) can reach a locally running
// service without a proxy. All origins are allowed because this server is
// expected to run on localhost during development only.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleLive answers liveness. It checks nothing on purpose: a dependency
// outage must make a pod unready, never get it restarted in a loop.
func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DefaultCheckTimeout)
	defer cancel()

	if err := s.health.Check(ctx); err != nil {
		if !errors.Is(err, ErrNotReady) {
			s.log.Warn("readiness check failed", "error", err)
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// Handler exposes the routing for tests.
func (s *Server) Handler() http.Handler { return s.srv.Handler }

// Start binds the port, then serves in the background until Shutdown.
//
// Binding happens before Start returns so a taken port fails startup. Binding
// inside the goroutine would leave a process that reports itself ready but
// that no probe or client can reach.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("observability: %w", err)
	}

	s.log.Info("http listener started", "addr", ln.Addr().String())
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("http listener failed", "error", err)
		}
	}()
	return nil
}

// Shutdown marks the service unready first, so the endpoint drops out of load
// balancing before the listener closes, then stops the listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.health.SetReady(false)
	return s.srv.Shutdown(ctx)
}
