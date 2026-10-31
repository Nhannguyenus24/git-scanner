package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mservice/gitscanner/internal/progress"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// ProgressHub is the live-progress surface the events route needs. Satisfied
// by *progress.Hub.
type ProgressHub interface {
	Subscribe(jobID string, after int64) ([]progress.Event, <-chan progress.Event, func())
	Done() <-chan struct{}
}

// WithProgress enables GET /v1/scans/{id}/events. hub feeds the streams; pub,
// when set, lets the api announce what it does itself — a cancel — so every
// replica's streams see it, not only this one's.
func (s *Server) WithProgress(hub ProgressHub, pub *progress.Publisher) *Server {
	s.hub = hub
	s.progress = pub
	return s
}

// heartbeat keeps an idle stream from being closed by a proxy that drops
// connections with no traffic, and lets the server notice a client that left.
const heartbeat = 15 * time.Second

// handleScanEvents streams a job's live progress as Server-Sent Events.
//
// Each event is a progress.Event as JSON, with its sequence number as the SSE
// id, so a browser that reconnects sends Last-Event-ID and resumes from the
// hub's replay instead of starting over. The stream opens with the recent
// events of the job, so a page opened mid-scan shows the step every repository
// is in. It ends with an "end" event once the job has ended.
//
// The job row's counters stay authoritative; this is the narration around them.
func (s *Server) handleScanEvents(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		s.writeError(w, http.StatusNotFound, errors.New("live progress is not enabled"))
		return
	}
	id := entity.ID(r.PathValue("id"))
	job, err := s.store.GetScanJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if q := r.URL.Query().Get("after"); q != "" {
		after, _ = strconv.ParseInt(q, 10, 64)
	}
	replay, events, stop := s.hub.Subscribe(id.String(), after)
	defer stop()

	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // nginx would otherwise hold events back
	w.WriteHeader(http.StatusOK)

	// retry tells the browser how long to wait before reconnecting on its own.
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
		return
	}
	for _, ev := range replay {
		if !writeEvent(w, ev) {
			return
		}
	}
	if job.State.Terminal() {
		writeEnd(w, string(job.State))
		_ = rc.Flush()
		return
	}
	if err := rc.Flush(); err != nil {
		return
	}

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.hub.Done():
			return // shutting down; the browser reconnects to another replica
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case ev := <-events:
			if !writeEvent(w, ev) {
				return
			}
			if ev.Stage == progress.StageJob {
				writeEnd(w, ev.Status)
				_ = rc.Flush()
				return
			}
			if rc.Flush() != nil {
				return
			}
		}
	}
}

func writeEvent(w http.ResponseWriter, ev progress.Event) bool {
	b, err := json.Marshal(ev)
	if err != nil {
		return true // skip the one event rather than end the stream
	}
	_, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.Seq, b)
	return err == nil
}

func writeEnd(w http.ResponseWriter, state string) {
	_, _ = fmt.Fprintf(w, "event: end\ndata: {\"state\":%q}\n\n", state)
}
