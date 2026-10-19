package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
	findings, err := s.store.ListFindings(r.Context(), entity.ID(r.PathValue("id")))
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
	writeJSON(w, http.StatusOK, map[string]any{"findings": out})
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
