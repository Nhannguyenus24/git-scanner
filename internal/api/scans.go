package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// scanRepoRequest submits one repository for scanning.
type scanRepoRequest struct {
	URL string `json:"url"`
}

type scanJobResponse struct {
	ID       string  `json:"id"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`

	ReposTotal    int    `json:"repos_total"`
	ReposScanned  int    `json:"repos_scanned"`
	BlobsScanned  int    `json:"blobs_scanned"`
	FindingsCount int    `json:"findings_count"`
	Error         string `json:"error,omitempty"`
}

// handleScanRepo validates a repository URL, records the target, job and
// repository, and enqueues one scan task (plan.md §8).
func (s *Server) handleScanRepo(w http.ResponseWriter, r *http.Request) {
	var req scanRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	target := entity.Target{Kind: entity.TargetRepo, Value: req.URL}
	if err := target.Validate(); err != nil {
		// Validation is a security boundary (the URL reaches git clone), so a
		// bad URL is a client error returned verbatim.
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	owner, name, err := github.ParseRepoURL(req.URL)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	target, err = s.store.CreateTarget(ctx, target)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	job, err := s.store.CreateScanJob(ctx, target.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	repo, err := s.store.UpsertRepository(ctx, entity.Repository{
		TargetID: target.ID, Owner: owner, Name: name, CloneURL: req.URL,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	// A single repo means the job's total is known up front, so completion can
	// fire when this one repo finishes (plan.md §8).
	if err := s.store.SetReposTotal(ctx, job.ID, 1); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.queue.EnqueueScan(ctx, job, repo); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	job.ReposTotal = 1
	s.log.Info("scan queued", "job", job.ID.String(), "url", req.URL)
	writeJSON(w, http.StatusAccepted, toScanResponse(job))
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.GetScanJob(r.Context(), entity.ID(r.PathValue("id")))
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toScanResponse(job))
}

func (s *Server) handleListFindings(w http.ResponseWriter, r *http.Request) {
	page := pageParam(r)
	findings, total, err := s.store.ListFindingsPage(r.Context(), entity.ID(r.PathValue("id")), page)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Shape the response so it carries the masked preview but never anything
	// that could reconstruct the secret.
	out := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		out = append(out, map[string]any{
			"id":                  f.ID.String(),
			"rule_id":             f.RuleID,
			"provider":            f.ProviderSlug,
			"severity":            f.Severity.String(),
			"masked_preview":      f.MaskedPreview,
			"file_path":           f.FilePath,
			"line_number":         f.LineNumber,
			"blob_oid":            f.BlobOID,
			"reachable_from_head": f.ReachableFromHEAD,
			"status":              string(f.Status),
		})
	}
	paged(w, "findings", out, total, page)
}

// handleDeleteScan removes a scan job and the findings it recorded (the DB
// cascades). It is the dashboard's "delete scan result" action. The repository
// and target survive, so a later scan of the same target still has them.
func (s *Server) handleDeleteScan(w http.ResponseWriter, r *http.Request) {
	id := entity.ID(r.PathValue("id"))
	err := s.store.DeleteScanJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("scan deleted", "job", id.String())
	w.WriteHeader(http.StatusNoContent)
}

// handleRescan starts a fresh scan of an existing job's target. It creates a new
// job rather than reviving the old one, so the history of both runs is kept, and
// re-dispatches the same kind of work: a single repository is re-enqueued
// directly, a user or org is re-enumerated through discovery.
func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	old, err := s.store.GetScanJob(ctx, entity.ID(r.PathValue("id")))
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("scan not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	target, err := s.store.GetTarget(ctx, old.TargetID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Everything that can reject the request is checked before the job exists:
	// a job created and then refused would sit queued forever, since nothing
	// was dispatched to move it.
	var owner, name string
	switch target.Kind {
	case entity.TargetRepo:
		if owner, name, err = github.ParseRepoURL(target.Value); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
	case entity.TargetUser, entity.TargetOrg:
	default:
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("cannot rescan target kind %q", target.Kind))
		return
	}

	job, err := s.store.CreateScanJob(ctx, target.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	switch target.Kind {
	case entity.TargetRepo:
		repo, rerr := s.store.UpsertRepository(ctx, entity.Repository{
			TargetID: target.ID, Owner: owner, Name: name, CloneURL: target.Value,
		})
		if rerr != nil {
			s.writeError(w, http.StatusInternalServerError, rerr)
			return
		}
		if err := s.store.SetReposTotal(ctx, job.ID, 1); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := s.queue.EnqueueScan(ctx, job, repo); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		job.ReposTotal = 1
	case entity.TargetUser, entity.TargetOrg:
		// include_forks is not stored on the target, so a rescan uses the default
		// (forks excluded); submit a new scan from the dialog to change it.
		if err := s.queue.EnqueueDiscovery(ctx, dispatch.DiscoveryRequest{
			JobID: job.ID, TargetID: target.ID, Kind: string(target.Kind), Value: target.Value,
		}); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	s.log.Info("rescan queued", "job", job.ID.String(), "from", old.ID.String(), "kind", string(target.Kind))
	writeJSON(w, http.StatusAccepted, toScanResponse(job))
}

func toScanResponse(j entity.ScanJob) scanJobResponse {
	return scanJobResponse{
		ID: j.ID.String(), State: string(j.State), Progress: j.Progress(),
		ReposTotal: j.ReposTotal, ReposScanned: j.ReposScanned,
		BlobsScanned: j.BlobsScanned, FindingsCount: j.FindingsCount, Error: j.Error,
	}
}

// decodeJSON reads a JSON body, rejecting unknown fields so a typo in a request
// is reported rather than silently ignored.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}
