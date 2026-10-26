package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// NoticeLog is the read side of the disclosure log, for the dashboard's
// notifications page.
type NoticeLog interface {
	ListNotifications(ctx context.Context, f store.NotificationFilter) ([]store.NotificationRow, int, error)
	GetNotification(ctx context.Context, id entity.ID) (entity.Notification, error)
	NotificationCounts(ctx context.Context) ([]store.Count, error)
	ListFindingsByIDs(ctx context.Context, ids []entity.ID) ([]entity.Finding, error)
}

// WithNoticeLog enables the notifications routes. info is the same operator
// identity the notifier signs with, so a previewed mail matches a sent one.
func (s *Server) WithNoticeLog(l NoticeLog, info notify.DisclosureInfo) *Server {
	s.notices = l
	s.discloseInfo = info
	return s
}

func (s *Server) noticeRoutes(mux *http.ServeMux) {
	if s.notices == nil {
		return
	}
	mux.HandleFunc("GET /v1/notifications", s.handleListNotifications)
	mux.HandleFunc("GET /v1/notifications/{id}/message", s.handleNotificationMessage)
	mux.HandleFunc("POST /v1/notifications/{id}/resend", s.handleResendNotification)
}

type notificationResponse struct {
	ID         string     `json:"id"`
	Repository repoRef    `json:"repository"`
	Recipient  string     `json:"recipient"`
	Findings   int        `json:"findings"`
	State      string     `json:"state"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	SentAt     *time.Time `json:"sent_at,omitempty"`
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	switch entity.NotificationState(state) {
	case "", entity.NotificationPending, entity.NotificationSent, entity.NotificationFailed, entity.NotificationSuppressed:
	default:
		s.writeError(w, http.StatusBadRequest, errors.New("unknown state"))
		return
	}

	ctx := r.Context()
	rows, total, err := s.notices.ListNotifications(ctx, store.NotificationFilter{
		State: state, Query: q.Get("q"), Limit: intParam(r, "limit"), Offset: intParam(r, "offset"),
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	counts, err := s.notices.NotificationCounts(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]notificationResponse, 0, len(rows))
	for _, n := range rows {
		out = append(out, notificationResponse{
			ID: n.ID.String(),
			Repository: repoRef{ID: n.RepositoryID.String(), Owner: n.RepoOwner, Name: n.RepoName,
				URL: "https://github.com/" + n.RepoOwner + "/" + n.RepoName},
			Recipient: n.Recipient, Findings: n.Findings, State: n.State, Error: n.Error,
			CreatedAt: n.CreatedAt, SentAt: n.SentAt,
		})
	}
	byState := make([]countResponse, 0, len(counts))
	for _, c := range counts {
		byState = append(byState, countResponse{Key: c.Key, Count: c.Count})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out, "total": total, "by_state": byState})
}

// handleNotificationMessage re-renders a disclosure from the findings it
// covered. The mail itself is not stored, so this reflects the findings as they
// are now: enrichment that landed after the send shows up here.
func (s *Server) handleNotificationMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, ok := s.loadNotification(w, r)
	if !ok {
		return
	}
	repo, err := s.store.GetRepository(ctx, n.RepositoryID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	findings, err := s.notices.ListFindingsByIDs(ctx, n.FindingIDs)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	m := notify.RenderDisclosure(repo, findings, s.discloseInfo)
	writeJSON(w, http.StatusOK, map[string]string{
		"to": n.Recipient, "subject": m.Subject, "text": m.Body, "html": m.HTML,
	})
}

// handleResendNotification queues a failed disclosure again, to the same
// recipient about the same findings. Only a failed one: a sent mail is the
// one-per-repository promise kept, and a suppressed one is the recipient's
// wish (plan.md §0.3).
func (s *Server) handleResendNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, ok := s.loadNotification(w, r)
	if !ok {
		return
	}
	if n.State != entity.NotificationFailed {
		s.writeError(w, http.StatusConflict, errors.New("only a failed notification can be resent"))
		return
	}
	on, err := s.store.NotificationsEnabled(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !on {
		s.writeError(w, http.StatusConflict, errors.New("notifications are turned off"))
		return
	}
	if err := s.queue.EnqueueNotify(ctx, dispatch.NotifyRequest{
		RepositoryID: n.RepositoryID, Recipient: n.Recipient, FindingIDs: n.FindingIDs,
	}); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("disclosure resend queued", "notification", n.ID.String(), "recipient", n.Recipient)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": n.ID.String(), "state": "queued"})
}

func (s *Server) loadNotification(w http.ResponseWriter, r *http.Request) (entity.Notification, bool) {
	id := entity.ID(r.PathValue("id"))
	if !validID(id) {
		s.writeError(w, http.StatusBadRequest, errors.New("invalid notification id"))
		return entity.Notification{}, false
	}
	n, err := s.notices.GetNotification(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("notification not found"))
		return entity.Notification{}, false
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return entity.Notification{}, false
	}
	return n, true
}
