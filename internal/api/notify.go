package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// pickRecipient chooses the most frequent real committer email across the
// findings, ignoring GitHub's noreply privacy addresses and anything that does
// not look like an address. It returns "" when there is no usable email, which
// the caller surfaces as a validation error.
func pickRecipient(findings []entity.Finding) string {
	count := map[string]int{}
	var best string
	for _, f := range findings {
		e := strings.ToLower(strings.TrimSpace(f.AuthorEmail))
		if e == "" || strings.Contains(e, "noreply") || !strings.Contains(e, "@") || !strings.Contains(e, ".") {
			continue
		}
		count[e]++
		if count[e] > count[best] {
			best = e
		}
	}
	return best
}

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

	open, err := s.store.ListOpenFindingsByRepository(ctx, repoID)
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

	// With no recipient given, fall back to the email of whoever committed the
	// leaked credentials — the person this notice is for, from data the scan
	// already has. GitHub's privacy addresses are not real inboxes, so they are
	// not used. The operator can always pass an explicit recipient instead.
	if req.Recipient == "" {
		req.Recipient = pickRecipient(open)
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
