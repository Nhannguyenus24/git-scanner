package api

import (
	"errors"
	"net/http"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

type scanTargetRequest struct {
	Name         string `json:"name"` // GitHub login (user or org)
	IncludeForks bool   `json:"include_forks"`
}

// handleScanUser starts a discovery scan over a user's repositories (plan.md §8).
func (s *Server) handleScanUser(w http.ResponseWriter, r *http.Request) {
	s.startDiscovery(w, r, entity.TargetUser)
}

// handleScanOrg starts a discovery scan over an org's repositories.
func (s *Server) handleScanOrg(w http.ResponseWriter, r *http.Request) {
	s.startDiscovery(w, r, entity.TargetOrg)
}

func (s *Server) startDiscovery(w http.ResponseWriter, r *http.Request, kind entity.TargetKind) {
	var req scanTargetRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	target := entity.Target{Kind: kind, Value: req.Name}
	if err := target.Validate(); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	target, err := s.store.CreateTarget(ctx, target)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	job, err := s.store.CreateScanJob(ctx, target.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.queue.EnqueueDiscovery(ctx, dispatch.DiscoveryRequest{
		JobID: job.ID, TargetID: target.ID, Kind: string(kind),
		Value: req.Name, IncludeForks: req.IncludeForks,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.log.Info("discovery queued", "job", job.ID.String(), "kind", string(kind), "target", req.Name)
	writeJSON(w, http.StatusAccepted, toScanResponse(job))
}

// handlePause asks a running job to stop at its next checkpoint. It sets the
// Redis flag the workers poll; the state flips to paused when a worker observes
// it, so the response reports "pause requested" rather than an immediate stop.
func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.control.RequestPause(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("pause requested", "job", id)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "pause": "requested"})
}

// handleResume clears the pause flag and re-enqueues discovery from the stored
// cursor (plan.md §8). In-flight scan tasks for already-discovered repos were
// never cancelled, so only enumeration needs restarting.
func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	id := entity.ID(r.PathValue("id"))
	ctx := r.Context()

	job, err := s.store.GetScanJob(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if job.State != entity.ScanPaused {
		s.writeError(w, http.StatusConflict, errors.New("scan is not paused"))
		return
	}

	if err := s.control.ClearPause(ctx, job.ID.String()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SetScanState(ctx, id, entity.ScanRunning, ""); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.queue.EnqueueDiscovery(ctx, dispatch.DiscoveryRequest{
		JobID: id, TargetID: job.TargetID, Cursor: job.Cursor,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("scan resumed", "job", id.String())
	writeJSON(w, http.StatusAccepted, toScanResponse(job))
}

// handleCancel moves a job to cancelled. Workers see the terminal state and
// stop enqueuing; in-flight tasks finish or dead-letter harmlessly.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := entity.ID(r.PathValue("id"))
	if err := s.store.SetScanState(r.Context(), id, entity.ScanCancelled, "cancelled by operator"); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
			return
		}
		s.writeError(w, http.StatusConflict, err)
		return
	}
	s.log.Info("scan cancelled", "job", id.String())
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id.String(), "state": "cancelled"})
}
