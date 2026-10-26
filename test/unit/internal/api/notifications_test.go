package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

type fakeNotices struct {
	byID     map[entity.ID]entity.Notification
	lastList store.NotificationFilter
}

func (f *fakeNotices) ListNotifications(_ context.Context, flt store.NotificationFilter) ([]store.NotificationRow, int, error) {
	f.lastList = flt
	var out []store.NotificationRow
	for _, n := range f.byID {
		out = append(out, store.NotificationRow{ID: n.ID, RepositoryID: n.RepositoryID, RepoOwner: "octocat",
			RepoName: "demo", Recipient: n.Recipient, Findings: len(n.FindingIDs), State: string(n.State), CreatedAt: time.Now()})
	}
	return out, len(out), nil
}
func (f *fakeNotices) GetNotification(_ context.Context, nID entity.ID) (entity.Notification, error) {
	n, ok := f.byID[nID]
	if !ok {
		return entity.Notification{}, store.ErrNotFound
	}
	return n, nil
}
func (f *fakeNotices) NotificationCounts(context.Context) ([]store.Count, error) {
	return []store.Count{{Key: "sent", Count: 1}}, nil
}
func (f *fakeNotices) ListFindingsByIDs(context.Context, []entity.ID) ([]entity.Finding, error) {
	return []entity.Finding{{RuleID: "x", Severity: entity.SeverityHigh, FilePath: ".env", MaskedPreview: "ab…cd"}}, nil
}

func noticeServer(t *testing.T, state entity.NotificationState) (*fakeNotices, entity.Notification, *fakeQueue, func(method, path string) (int, string)) {
	t.Helper()
	srv, st, q := newServer(t)
	repoID := id.New()
	st.repos[repoID] = entity.Repository{Owner: "octocat", Name: "demo"}
	n := entity.Notification{ID: id.New(), RepositoryID: repoID, Recipient: "owner@example.com",
		FindingIDs: []entity.ID{id.New()}, State: state}
	notices := &fakeNotices{byID: map[entity.ID]entity.Notification{n.ID: n}}
	srv.WithNoticeLog(notices, notify.DisclosureInfo{Signature: "-- Tester"})
	return notices, n, q, func(method, path string) (int, string) {
		rec := do(srv, method, path, "")
		return rec.Code, rec.Body.String()
	}
}

func TestListNotifications(t *testing.T) {
	notices, _, _, call := noticeServer(t, entity.NotificationSent)

	code, body := call(http.MethodGet, "/v1/notifications?state=sent&q=octo&limit=10")
	if code != http.StatusOK {
		t.Fatalf("status = %d, body %s", code, body)
	}
	var resp struct {
		Notifications []struct {
			Recipient string `json:"recipient"`
			State     string `json:"state"`
		} `json:"notifications"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || resp.Notifications[0].Recipient != "owner@example.com" {
		t.Errorf("resp = %+v", resp)
	}
	if notices.lastList.State != "sent" || notices.lastList.Query != "octo" || notices.lastList.Limit != 10 {
		t.Errorf("filter = %+v", notices.lastList)
	}

	if code, _ := call(http.MethodGet, "/v1/notifications?state=bogus"); code != http.StatusBadRequest {
		t.Errorf("bogus state = %d, want 400", code)
	}
}

func TestNotificationMessageRendersTheMail(t *testing.T) {
	_, n, _, call := noticeServer(t, entity.NotificationSent)
	code, body := call(http.MethodGet, "/v1/notifications/"+n.ID.String()+"/message")
	if code != http.StatusOK {
		t.Fatalf("status = %d, body %s", code, body)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	if m["to"] != "owner@example.com" || !strings.Contains(m["subject"], "octocat/demo") ||
		!strings.Contains(m["text"], "-- Tester") || !strings.HasPrefix(m["html"], "<!doctype html>") {
		t.Errorf("message = %+v", m)
	}
	if code, _ := call(http.MethodGet, "/v1/notifications/"+id.New().String()+"/message"); code != http.StatusNotFound {
		t.Errorf("unknown id = %d, want 404", code)
	}
}

// Only a failed mail may be resent: a sent one is the one-per-repository
// promise kept, a suppressed one is the recipient's wish.
func TestResendOnlyFailed(t *testing.T) {
	_, sent, q, call := noticeServer(t, entity.NotificationSent)
	if code, _ := call(http.MethodPost, "/v1/notifications/"+sent.ID.String()+"/resend"); code != http.StatusConflict {
		t.Errorf("resend sent = %d, want 409", code)
	}
	if q.notifies != 0 {
		t.Error("a sent notification was queued again")
	}

	_, failed, q, call := noticeServer(t, entity.NotificationFailed)
	if code, body := call(http.MethodPost, "/v1/notifications/"+failed.ID.String()+"/resend"); code != http.StatusAccepted {
		t.Fatalf("resend failed = %d, body %s", code, body)
	}
	if q.lastNotify.Recipient != "owner@example.com" || len(q.lastNotify.FindingIDs) != 1 {
		t.Errorf("queued %+v", q.lastNotify)
	}
}
