package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// discloseRequest asks for one disclosure mail about a repository. FindingIDs
// is optional: omitted, the disclosure covers every open finding the repository
// has, which is what an operator almost always means.
type discloseRequest struct {
	Recipient  string   `json:"recipient"`
	FindingIDs []string `json:"finding_ids"`
}

// handleDisclose queues a responsible-disclosure mail (plan.md §0.3).
//
// Queueing is all this does. Whether the mail is actually sent depends on the
// suppression list, the shared rate cap and the one-per-repository constraint,
// all of which the notifier applies — so a 202 here means "accepted for
// consideration", not "sent".
func (s *Server) handleDisclose(w http.ResponseWriter, r *http.Request) {
	var req discloseRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	repoID := entity.ID(r.PathValue("id"))
	ctx := r.Context()

	repo, err := s.store.GetRepository(ctx, repoID)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("repository not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	findingIDs, err := s.discloseFindings(ctx, repoID, req.FindingIDs)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(findingIDs) == 0 {
		s.writeError(w, http.StatusConflict, errors.New("repository has no open findings to disclose"))
		return
	}

	// Validate the whole notification rather than the address alone, so the API
	// rejects exactly what the notifier would drop.
	n := entity.Notification{RepositoryID: repoID, Recipient: req.Recipient, FindingIDs: findingIDs}
	if err := n.Validate(); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.queue.EnqueueNotify(ctx, dispatch.NotifyRequest{
		RepositoryID: repoID, Recipient: req.Recipient, FindingIDs: findingIDs,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.log.Info("disclosure queued",
		"repo", repo.FullName(), "recipient", req.Recipient, "findings", len(findingIDs))

	writeJSON(w, http.StatusAccepted, map[string]any{
		"repository_id": repoID.String(),
		"recipient":     req.Recipient,
		"findings":      len(findingIDs),
		"state":         "queued",
	})
}

// discloseFindings resolves which findings the mail covers: the caller's list
// when given, otherwise every open finding for the repository.
func (s *Server) discloseFindings(ctx context.Context, repoID entity.ID, requested []string) ([]entity.ID, error) {
	if len(requested) > 0 {
		out := make([]entity.ID, 0, len(requested))
		for _, id := range requested {
			out = append(out, entity.ID(id))
		}
		return out, nil
	}

	open, err := s.store.ListOpenFindingsByRepository(ctx, repoID)
	if err != nil {
		return nil, err
	}
	out := make([]entity.ID, 0, len(open))
	for _, f := range open {
		out = append(out, f.ID)
	}
	return out, nil
}
