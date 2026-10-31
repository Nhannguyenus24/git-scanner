package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// OwnerLookup finds the address of the account that owns a repository.
// Satisfied by the GitHub client.
type OwnerLookup interface {
	OwnerEmail(ctx context.Context, owner, repo string) (string, error)
}

// WithOwnerLookup enables recipient resolution for disclosures that do not
// name one. Without it such requests are rejected.
func (s *Server) WithOwnerLookup(o OwnerLookup) *Server {
	s.owners = o
	return s
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

	// Checked here as well as in the notifier, so an operator who forgot the
	// switch learns it from the response rather than from a silent drop.
	on, err := s.store.NotificationsEnabled(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !on {
		s.writeError(w, http.StatusConflict, errors.New("notifications are turned off"))
		return
	}

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
	var foreign *foreignFindingError
	if errors.As(err, &foreign) {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(findingIDs) == 0 {
		s.writeError(w, http.StatusConflict, errors.New("repository has no open findings to disclose"))
		return
	}

	// With no recipient given, write to the repository's owner: the one person
	// who can rotate the credential and rewrite the history. The committer is
	// not used — on a team repository that is a member, who may have no rights
	// over either. The operator can always pass an explicit recipient instead.
	if req.Recipient == "" && s.owners != nil {
		email, err := s.owners.OwnerEmail(ctx, repo.Owner, repo.Name)
		// Throttling is the operator's to fix (a token, or a wait), so it is
		// reported as such rather than hidden behind a generic 5xx.
		var rl *github.RateLimitError
		if errors.As(err, &rl) {
			s.writeError(w, http.StatusTooManyRequests, fmt.Errorf(
				"GitHub rate limit reached looking up the owner of %s; retry in %s or set GITHUB_TOKEN",
				repo.FullName(), rl.RetryAfter.Round(time.Minute)))
			return
		}
		if err != nil {
			s.writeError(w, http.StatusBadGateway, fmt.Errorf("look up owner of %s: %w", repo.FullName(), err))
			return
		}
		if email == "" {
			s.writeError(w, http.StatusUnprocessableEntity,
				fmt.Errorf("%s publishes no email on GitHub; pass a recipient explicitly", repo.Owner))
			return
		}
		req.Recipient = email
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

// foreignFindingError names a requested finding that is not an open finding of
// the repository being disclosed.
type foreignFindingError struct{ id string }

func (e *foreignFindingError) Error() string {
	return fmt.Sprintf("finding %q is not an open finding of this repository", e.id)
}

// discloseFindings resolves which findings the mail covers: the caller's list
// when given, otherwise every open finding for the repository.
//
// A caller's list is checked against the repository's open findings. The mail
// goes to this repository's owner, so an id from another repository would
// describe someone else's leak to them.
func (s *Server) discloseFindings(ctx context.Context, repoID entity.ID, requested []string) ([]entity.ID, error) {
	open, err := s.store.ListOpenFindingsByRepository(ctx, repoID)
	if err != nil {
		return nil, err
	}

	if len(requested) > 0 {
		owned := make(map[entity.ID]bool, len(open))
		for _, f := range open {
			owned[f.ID] = true
		}
		out := make([]entity.ID, 0, len(requested))
		seen := make(map[entity.ID]bool, len(requested))
		for _, id := range requested {
			fid := entity.ID(id)
			if !owned[fid] {
				return nil, &foreignFindingError{id: id}
			}
			if !seen[fid] {
				seen[fid] = true
				out = append(out, fid)
			}
		}
		return out, nil
	}

	out := make([]entity.ID, 0, len(open))
	for _, f := range open {
		out = append(out, f.ID)
	}
	return out, nil
}
