// Package api is the REST surface (plan.md §8) and, until it is split out, the
// provider registry endpoints (plan.md §5.5).
//
// Handlers live here, not in cmd/api, so they can be tested with httptest and
// so cmd/api stays a declaration of what the binary is.
//
// Files:
//
//	api.go     Server, routing, JSON helpers
//	scans.go   scan submission, status, findings
//	notify.go  responsible-disclosure requests
//	catalog.go dashboard browsing (overview, scans, findings, repositories,
//	           candidates) and finding triage
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/libs/buildinfo"
	"github.com/mservice/gitscanner/libs/entity"
)

// Store is the database surface the API depends on, named here so the handlers
// are testable against a fake without pulling in pgx.
type Store interface {
	CreateTarget(ctx context.Context, t entity.Target) (entity.Target, error)
	GetTarget(ctx context.Context, targetID entity.ID) (entity.Target, error)
	CreateScanJob(ctx context.Context, targetID entity.ID) (entity.ScanJob, error)
	DeleteScanJob(ctx context.Context, jobID entity.ID) error
	UpsertRepository(ctx context.Context, r entity.Repository) (entity.Repository, error)
	GetScanJob(ctx context.Context, jobID entity.ID) (entity.ScanJob, error)
	GetRepository(ctx context.Context, repoID entity.ID) (entity.Repository, error)
	ListFindings(ctx context.Context, jobID entity.ID) ([]entity.Finding, error)
	ListOpenFindingsByRepository(ctx context.Context, repoID entity.ID) ([]entity.Finding, error)
	SetReposTotal(ctx context.Context, jobID entity.ID, n int) error
	SetScanState(ctx context.Context, jobID entity.ID, to entity.ScanState, errMsg string) error
}

// TaskQueue dispatches scan work. Satisfied by the rabbit-backed dispatcher.
type TaskQueue interface {
	EnqueueScan(ctx context.Context, job entity.ScanJob, repo entity.Repository) error
	EnqueueDiscovery(ctx context.Context, req dispatch.DiscoveryRequest) error
	EnqueueNotify(ctx context.Context, req dispatch.NotifyRequest) error
}

// Control is the pause/resume flag surface, backed by Redis (plan.md §8).
type Control interface {
	RequestPause(ctx context.Context, jobID string) error
	ClearPause(ctx context.Context, jobID string) error
}

// Server holds the API dependencies and builds the router.
type Server struct {
	store   Store
	queue   TaskQueue
	control   Control
	catalog   Catalog             // optional: the dashboard's browsing and triage routes
	providers []scan.ProviderInfo // the detection catalogue the dashboard lists
	log       *slog.Logger
}

// WithProviders registers the providers the detection engine can attribute a
// finding to, so the dashboard can list them. The list is derived from the
// active ruleset (scan.Ruleset.Providers), which keeps it in lockstep with what
// the engine detects rather than a copy that drifts.
func (s *Server) WithProviders(ps []scan.ProviderInfo) *Server {
	s.providers = ps
	return s
}

// NewServer wires the API. Either dependency may be nil in a degraded mode, but
// the scan routes require both.
func NewServer(store Store, queue TaskQueue, control Control, log *slog.Logger) *Server {
	return &Server{store: store, queue: queue, control: control, log: log}
}

// Routes returns every route the api serves besides the probes.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", s.handleVersion)
	mux.HandleFunc("GET /v1/providers", s.handleListProviders)
	mux.HandleFunc("POST /v1/scans/repo", s.handleScanRepo)
	mux.HandleFunc("POST /v1/scans/user", s.handleScanUser)
	mux.HandleFunc("POST /v1/scans/org", s.handleScanOrg)
	mux.HandleFunc("POST /v1/scans/random", s.handleScanRandom)
	mux.HandleFunc("GET /v1/scans/{id}", s.handleGetScan)
	mux.HandleFunc("DELETE /v1/scans/{id}", s.handleDeleteScan)
	mux.HandleFunc("GET /v1/scans/{id}/findings", s.handleListFindings)
	mux.HandleFunc("POST /v1/scans/{id}/pause", s.handlePause)
	mux.HandleFunc("POST /v1/scans/{id}/resume", s.handleResume)
	mux.HandleFunc("POST /v1/scans/{id}/cancel", s.handleCancel)
	mux.HandleFunc("POST /v1/scans/{id}/rescan", s.handleRescan)
	mux.HandleFunc("POST /v1/repositories/{id}/disclose", s.handleDisclose)
	s.catalogRoutes(mux)
	return mux
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Service string `json:"service"`
		buildinfo.Info
	}{Service: "api", Info: buildinfo.Get()})
}

type providerResponse struct {
	Slug     string `json:"slug"`
	Severity string `json:"severity"`
	Rules    int    `json:"rules"`
}

// handleListProviders returns the detection catalogue — the providers a finding
// can be attributed to — so the dashboard can show what is supported. An
// unconfigured server (WithProviders never called) returns an empty list rather
// than a 404, so the page renders cleanly in a degraded deployment.
func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) {
	out := make([]providerResponse, 0, len(s.providers))
	for _, p := range s.providers {
		out = append(out, providerResponse{Slug: p.Slug, Severity: p.Severity, Rules: p.Rules})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError returns a JSON error body. Client mistakes (4xx) are returned
// verbatim; a 5xx is logged and replaced with a generic message so an internal
// detail does not leak to the caller.
func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	msg := err.Error()
	if status >= 500 {
		if s.log != nil {
			s.log.Error("request failed", "status", status, "error", err)
		}
		msg = "internal error"
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
