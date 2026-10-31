package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mservice/gitscanner/internal/share"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// ShareStore is the persistence the share-link routes need. Satisfied by
// *store.DB.
type ShareStore interface {
	RepositoryAuthorized(ctx context.Context, repoID entity.ID) (bool, error)
	CreateShareLink(ctx context.Context, l entity.ShareLink, tokenHash []byte) error
	ListShareLinks(ctx context.Context, repoID entity.ID) ([]entity.ShareLink, error)
	RevokeShareLink(ctx context.Context, id entity.ID) error
	OpenShareLink(ctx context.Context, tokenHash []byte, now time.Time) (entity.ShareLink, error)
	ListFindingsByRepository(ctx context.Context, repoID entity.ID) ([]entity.Finding, error)
}

// WithShares enables share links: creating, listing and revoking them, and the
// read-only report a link opens.
func (s *Server) WithShares(st ShareStore) *Server {
	s.shares = st
	return s
}

func (s *Server) shareRoutes(mux *http.ServeMux) {
	if s.shares == nil {
		return
	}
	mux.HandleFunc("POST /v1/repositories/{id}/shares", s.handleCreateShare)
	mux.HandleFunc("GET /v1/repositories/{id}/shares", s.handleListShares)
	mux.HandleFunc("DELETE /v1/shares/{id}", s.handleRevokeShare)
	mux.HandleFunc("GET /v1/shared/{token}", s.handleSharedReport)
	mux.HandleFunc("GET /v1/shared/{token}/report", s.handleSharedReportFile)
}

type createShareRequest struct {
	Detail          string `json:"detail"`
	ExpiresInDays   int    `json:"expires_in_days"`
	IncludeResolved bool   `json:"include_resolved"`
}

type shareResponse struct {
	ID              string     `json:"id"`
	RepositoryID    string     `json:"repository_id"`
	Detail          string     `json:"detail"`
	IncludeResolved bool       `json:"include_resolved"`
	State           string     `json:"state"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	Views           int        `json:"views"`
	LastViewedAt    *time.Time `json:"last_viewed_at,omitempty"`
	// Token is present only in the response that creates the link: it is not
	// stored, so this is the one time it can be shown.
	Token string `json:"token,omitempty"`
}

func toShareResponse(l entity.ShareLink, now time.Time) shareResponse {
	return shareResponse{
		ID: l.ID.String(), RepositoryID: l.RepositoryID.String(), Detail: string(l.Detail),
		IncludeResolved: l.IncludeResolved, State: string(l.State(now)),
		CreatedAt: l.CreatedAt, ExpiresAt: l.ExpiresAt, RevokedAt: l.RevokedAt,
		Views: l.Views, LastViewedAt: l.LastViewedAt,
	}
}

// handleCreateShare makes a link to one repository's results.
//
// Refused for a repository that only a random sample ever reached: sharing is
// a form of disclosure, and plan.md §0.1 keeps those samples away from it.
func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var req createShareRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	detail, err := entity.ParseShareDetail(req.Detail)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	days := req.ExpiresInDays
	if days == 0 {
		days = entity.ShareDefaultDays
	}
	if days < 1 || days > entity.ShareMaxDays {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("expires_in_days must be between 1 and %d", entity.ShareMaxDays))
		return
	}

	ctx := r.Context()
	repoID := entity.ID(r.PathValue("id"))
	repo, err := s.store.GetRepository(ctx, repoID)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("repository not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	ok, err := s.shares.RepositoryAuthorized(ctx, repoID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		s.writeError(w, http.StatusConflict, errors.New(
			"this repository was only reached by a random sample; scan it as a repository, user or org target before sharing its results"))
		return
	}

	token, hash, err := share.NewToken()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now().UTC()
	link := entity.ShareLink{
		ID: id.New(), RepositoryID: repoID, Detail: detail, IncludeResolved: req.IncludeResolved,
		CreatedAt: now, ExpiresAt: now.Add(time.Duration(days) * 24 * time.Hour),
	}
	if err := s.shares.CreateShareLink(ctx, link, hash); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	// The token is never logged: it is the credential to the report.
	s.log.Info("share link created", "repo", repo.FullName(), "share", link.ID.String(),
		"detail", string(detail), "expires_at", link.ExpiresAt.Format(time.RFC3339))

	resp := toShareResponse(link, now)
	resp.Token = token
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListShares(w http.ResponseWriter, r *http.Request) {
	links, err := s.shares.ListShareLinks(r.Context(), entity.ID(r.PathValue("id")))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	out := make([]shareResponse, 0, len(links))
	for _, l := range links {
		out = append(out, toShareResponse(l, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": out})
}

func (s *Server) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	id := entity.ID(r.PathValue("id"))
	if err := s.shares.RevokeShareLink(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, errors.New("share link not found"))
			return
		}
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("share link revoked", "share", id.String())
	w.WriteHeader(http.StatusNoContent)
}

// errLinkGone is the one answer for a token that is malformed, unknown, expired
// or revoked, so a caller learns nothing by trying tokens.
var errLinkGone = errors.New("this link does not exist or is no longer valid")

// openReport resolves a token to its report, or writes the error and returns
// false.
func (s *Server) openReport(w http.ResponseWriter, r *http.Request) (share.Report, bool) {
	// The report names where credentials sit in someone's history: never cache
	// it, never send its URL on as a referrer, never index it.
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow")

	hash, err := share.ParseToken(r.PathValue("token"))
	if err != nil {
		s.writeError(w, http.StatusNotFound, errLinkGone)
		return share.Report{}, false
	}
	ctx := r.Context()
	now := time.Now().UTC()
	link, err := s.shares.OpenShareLink(ctx, hash, now)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errLinkGone)
		return share.Report{}, false
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return share.Report{}, false
	}
	repo, err := s.store.GetRepository(ctx, link.RepositoryID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return share.Report{}, false
	}
	findings, err := s.shares.ListFindingsByRepository(ctx, link.RepositoryID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return share.Report{}, false
	}
	s.log.Info("share link opened", "share", link.ID.String(), "repo", repo.FullName(), "views", link.Views)
	return share.Build(repo, findings, link, now), true
}

// handleSharedReport is what the shared page renders.
func (s *Server) handleSharedReport(w http.ResponseWriter, r *http.Request) {
	if rep, ok := s.openReport(w, r); ok {
		writeJSON(w, http.StatusOK, rep)
	}
}

// handleSharedReportFile serves the report as Markdown (a download) or HTML (a
// page to read or print to PDF).
func (s *Server) handleSharedReportFile(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "md" && format != "html" {
		s.writeError(w, http.StatusBadRequest, errors.New(`format must be "md" or "html"`))
		return
	}
	rep, ok := s.openReport(w, r)
	if !ok {
		return
	}
	name := "secret-scan-" + filenameSafe(rep.Repo) + "-" + rep.GeneratedAt.Format("20060102")
	if format == "md" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.md"`)
		_, _ = w.Write([]byte(share.Markdown(rep)))
		return
	}
	page, err := share.HTML(rep)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// The page needs nothing from anywhere; say so, so a browser enforces it.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'")
	h.Set("Content-Disposition", `inline; filename="`+name+`.html"`)
	_, _ = w.Write([]byte(page))
}

func filenameSafe(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			b[i] = '-'
		}
	}
	return string(b)
}
