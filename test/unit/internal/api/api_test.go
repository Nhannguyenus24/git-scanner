package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/api"
	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// fakeStore is an in-memory Store for the handler tests.
type fakeStore struct {
	jobs     map[entity.ID]entity.ScanJob
	findings map[entity.ID][]entity.Finding
	repos    map[entity.ID]entity.Repository
	targets  map[entity.ID]entity.Target
	// openFindings is keyed by repository id, for the disclosure handler.
	openFindings map[entity.ID][]entity.Finding
	// notifyOff is inverted so the zero value leaves disclosure enabled.
	notifyOff bool
}

func (s *fakeStore) NotificationsEnabled(context.Context) (bool, error) { return !s.notifyOff, nil }
func (s *fakeStore) SetNotificationsEnabled(_ context.Context, on bool) error {
	s.notifyOff = !on
	return nil
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		jobs:         map[entity.ID]entity.ScanJob{},
		findings:     map[entity.ID][]entity.Finding{},
		repos:        map[entity.ID]entity.Repository{},
		targets:      map[entity.ID]entity.Target{},
		openFindings: map[entity.ID][]entity.Finding{},
	}
}

func (s *fakeStore) CreateTarget(_ context.Context, t entity.Target) (entity.Target, error) {
	t.ID = id.New()
	s.targets[t.ID] = t
	return t, nil
}
func (s *fakeStore) GetTarget(_ context.Context, targetID entity.ID) (entity.Target, error) {
	t, ok := s.targets[targetID]
	if !ok {
		return entity.Target{}, store.ErrNotFound
	}
	return t, nil
}
func (s *fakeStore) DeleteScanJob(_ context.Context, jobID entity.ID) error {
	if _, ok := s.jobs[jobID]; !ok {
		return store.ErrNotFound
	}
	delete(s.jobs, jobID)
	delete(s.findings, jobID)
	return nil
}
func (s *fakeStore) CreateScanJob(_ context.Context, targetID entity.ID) (entity.ScanJob, error) {
	j := entity.ScanJob{ID: id.New(), TargetID: targetID, State: entity.ScanQueued}
	s.jobs[j.ID] = j
	return j, nil
}
func (s *fakeStore) UpsertRepository(_ context.Context, r entity.Repository) (entity.Repository, error) {
	r.ID = id.New()
	return r, nil
}
func (s *fakeStore) GetScanJob(_ context.Context, jobID entity.ID) (entity.ScanJob, error) {
	j, ok := s.jobs[jobID]
	if !ok {
		return entity.ScanJob{}, store.ErrNotFound
	}
	return j, nil
}
func (s *fakeStore) ListFindings(_ context.Context, jobID entity.ID) ([]entity.Finding, error) {
	return s.findings[jobID], nil
}
func (s *fakeStore) GetRepository(_ context.Context, repoID entity.ID) (entity.Repository, error) {
	r, ok := s.repos[repoID]
	if !ok {
		return entity.Repository{}, store.ErrNotFound
	}
	return r, nil
}
func (s *fakeStore) ListOpenFindingsByRepository(_ context.Context, repoID entity.ID) ([]entity.Finding, error) {
	return s.openFindings[repoID], nil
}
func (s *fakeStore) SetReposTotal(_ context.Context, jobID entity.ID, n int) error {
	j := s.jobs[jobID]
	j.ReposTotal = n
	s.jobs[jobID] = j
	return nil
}
func (s *fakeStore) SetScanState(_ context.Context, jobID entity.ID, to entity.ScanState, _ string) error {
	j, ok := s.jobs[jobID]
	if !ok {
		return store.ErrNotFound
	}
	j.State = to
	s.jobs[jobID] = j
	return nil
}

type fakeQueue struct {
	scans, discoveries, notifies int
	lastNotify                  dispatch.NotifyRequest
}

func (q *fakeQueue) EnqueueScan(context.Context, entity.ScanJob, entity.Repository) error {
	q.scans++
	return nil
}
func (q *fakeQueue) EnqueueDiscovery(context.Context, dispatch.DiscoveryRequest) error {
	q.discoveries++
	return nil
}
func (q *fakeQueue) EnqueueNotify(_ context.Context, req dispatch.NotifyRequest) error {
	q.notifies++
	q.lastNotify = req
	return nil
}

type fakeControl struct{ paused, cleared int }

func (c *fakeControl) RequestPause(context.Context, entity.ID) error { c.paused++; return nil }
func (c *fakeControl) ClearPause(context.Context, entity.ID) error   { c.cleared++; return nil }

func newServer(t *testing.T) (*api.Server, *fakeStore, *fakeQueue) {
	t.Helper()
	st, q := newFakeStore(), &fakeQueue{}
	srv := api.NewServer(st, q, &fakeControl{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv, st, q
}

func do(srv *api.Server, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestVersion(t *testing.T) {
	srv, _, _ := newServer(t)
	rec := do(srv, http.MethodGet, "/v1/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["service"] != "api" || body["version"] == "" {
		t.Errorf("body = %v", body)
	}
}

func TestScanRepoAcceptsAndEnqueues(t *testing.T) {
	srv, _, q := newServer(t)
	rec := do(srv, http.MethodPost, "/v1/scans/repo", `{"url":"https://github.com/owner/repo.git"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if q.scans != 1 {
		t.Errorf("scans = %d, want 1", q.scans)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["state"] != "queued" {
		t.Errorf("state = %v, want queued", resp["state"])
	}
}

func TestScanRepoRejectsBadInput(t *testing.T) {
	srv, _, q := newServer(t)
	cases := []string{
		`{"url":"ssh://git@github.com/a/b.git"}`,      // dangerous transport
		`{"url":"https://gitlab.com/a/b.git"}`,        // not github
		`{"url":"https://github.com/onlyowner"}`,      // no repo
		`{"url":"https://user:x@github.com/a/b.git"}`, // embedded credentials
		`{"nope":"x"}`, // unknown field
		`not json`,
	}
	for _, c := range cases {
		rec := do(srv, http.MethodPost, "/v1/scans/repo", c)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("input %q: status = %d, want 400", c, rec.Code)
		}
	}
	if q.scans != 0 {
		t.Errorf("a bad request was enqueued (%d)", q.scans)
	}
}

func TestGetScanNotFound(t *testing.T) {
	srv, _, _ := newServer(t)
	rec := do(srv, http.MethodGet, "/v1/scans/"+id.New().String(), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestScanUserEnqueuesDiscovery(t *testing.T) {
	srv, _, q := newServer(t)
	rec := do(srv, http.MethodPost, "/v1/scans/user", `{"name":"octocat"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if q.discoveries != 1 {
		t.Errorf("discoveries = %d, want 1", q.discoveries)
	}
}

func TestScanUserRejectsBadLogin(t *testing.T) {
	srv, _, _ := newServer(t)
	rec := do(srv, http.MethodPost, "/v1/scans/user", `{"name":"-bad-"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPauseResumeCancel(t *testing.T) {
	srv, st, q := newServer(t)
	// seed a running job
	tgt, _ := st.CreateTarget(context.Background(), entity.Target{Kind: entity.TargetUser, Value: "octocat"})
	job, _ := st.CreateScanJob(context.Background(), tgt.ID)
	_ = st.SetScanState(context.Background(), job.ID, entity.ScanRunning, "")

	if rec := do(srv, http.MethodPost, "/v1/scans/"+job.ID.String()+"/pause", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("pause status = %d", rec.Code)
	}
	// resume requires paused state
	if rec := do(srv, http.MethodPost, "/v1/scans/"+job.ID.String()+"/resume", ""); rec.Code != http.StatusConflict {
		t.Errorf("resume while running = %d, want 409", rec.Code)
	}
	_ = st.SetScanState(context.Background(), job.ID, entity.ScanPaused, "")
	if rec := do(srv, http.MethodPost, "/v1/scans/"+job.ID.String()+"/resume", ""); rec.Code != http.StatusAccepted {
		t.Errorf("resume status = %d", rec.Code)
	}
	if q.discoveries != 1 {
		t.Errorf("resume did not re-enqueue discovery (%d)", q.discoveries)
	}
	if rec := do(srv, http.MethodPost, "/v1/scans/"+job.ID.String()+"/cancel", ""); rec.Code != http.StatusAccepted {
		t.Errorf("cancel status = %d", rec.Code)
	}
}

type fakeOwners struct {
	email string
	asked string
}

func (o *fakeOwners) OwnerEmail(_ context.Context, owner, _ string) (string, error) {
	o.asked = owner
	return o.email, nil
}

// With no recipient in the request, the mail goes to the repository's owner —
// not to whoever committed the value, who on a team repository is a member.
func TestDiscloseWritesToTheOwner(t *testing.T) {
	srv, st, q := newServer(t)
	owners := &fakeOwners{email: "owner@example.com"}
	srv.WithOwnerLookup(owners)
	repoID := id.New()
	st.repos[repoID] = entity.Repository{Owner: "octocat", Name: "demo"}
	st.openFindings[repoID] = []entity.Finding{{ID: id.New(), AuthorEmail: "member@example.com"}}

	rec := do(srv, http.MethodPost, "/v1/repositories/"+repoID.String()+"/disclose", `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if owners.asked != "octocat" || q.lastNotify.Recipient != "owner@example.com" {
		t.Errorf("asked %q, recipient %q; want owner octocat and owner@example.com", owners.asked, q.lastNotify.Recipient)
	}
}

// An owner with no published address is reported, not guessed at.
func TestDiscloseRejectsWhenOwnerHasNoEmail(t *testing.T) {
	srv, st, q := newServer(t)
	srv.WithOwnerLookup(&fakeOwners{})
	repoID := id.New()
	st.repos[repoID] = entity.Repository{Owner: "a", Name: "b"}
	st.openFindings[repoID] = []entity.Finding{{ID: id.New(), AuthorEmail: "member@example.com"}}
	if rec := do(srv, http.MethodPost, "/v1/repositories/"+repoID.String()+"/disclose", `{}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if q.lastNotify.Recipient != "" {
		t.Errorf("queued to %q", q.lastNotify.Recipient)
	}
}

func TestDeleteScan(t *testing.T) {
	srv, st, _ := newServer(t)
	// Submit a repo scan, then delete the job it created.
	rec := do(srv, http.MethodPost, "/v1/scans/repo", `{"url":"https://github.com/owner/repo.git"}`)
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	jobID := resp["id"].(string)

	rec = do(srv, http.MethodDelete, "/v1/scans/"+jobID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body)
	}
	if _, ok := st.jobs[entity.ID(jobID)]; ok {
		t.Error("job still present after delete")
	}
	// Deleting again is a 404.
	if rec = do(srv, http.MethodDelete, "/v1/scans/"+jobID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", rec.Code)
	}
}

func TestRescanRepoCreatesNewJobAndEnqueues(t *testing.T) {
	srv, _, q := newServer(t)
	rec := do(srv, http.MethodPost, "/v1/scans/repo", `{"url":"https://github.com/owner/repo.git"}`)
	var first map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	firstID := first["id"].(string)

	rec = do(srv, http.MethodPost, "/v1/scans/"+firstID+"/rescan", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("rescan status = %d, body %s", rec.Code, rec.Body)
	}
	var second map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second["id"] == firstID || second["id"] == "" {
		t.Errorf("rescan should create a new job id, got %v (first %v)", second["id"], firstID)
	}
	if second["state"] != "queued" {
		t.Errorf("state = %v, want queued", second["state"])
	}
	if q.scans != 2 { // the original submit plus the rescan
		t.Errorf("scans enqueued = %d, want 2", q.scans)
	}
}

func TestRescanUserEnqueuesDiscovery(t *testing.T) {
	srv, _, q := newServer(t)
	rec := do(srv, http.MethodPost, "/v1/scans/user", `{"name":"octocat","include_forks":false}`)
	var first map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &first)

	rec = do(srv, http.MethodPost, "/v1/scans/"+first["id"].(string)+"/rescan", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("rescan status = %d, body %s", rec.Code, rec.Body)
	}
	if q.discoveries != 2 {
		t.Errorf("discoveries = %d, want 2", q.discoveries)
	}
}

func TestRescanNotFound(t *testing.T) {
	srv, _, _ := newServer(t)
	if rec := do(srv, http.MethodPost, "/v1/scans/"+id.New().String()+"/rescan", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// The switch is checked before anything is queued, so turning mail off cannot
// be undone by a request already in flight at the api.
func TestDiscloseRefusedWhenNotificationsOff(t *testing.T) {
	srv, st, q := newServer(t)
	repoID := id.New()
	st.repos[repoID] = entity.Repository{Owner: "octocat", Name: "demo"}
	st.openFindings[repoID] = []entity.Finding{{ID: id.New(), AuthorEmail: "dev@example.com"}}
	st.notifyOff = true

	if rec := do(srv, http.MethodPost, "/v1/repositories/"+repoID.String()+"/disclose", `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if q.lastNotify.Recipient != "" {
		t.Errorf("a disclosure was queued while notifications were off")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	srv, st, _ := newServer(t)

	if rec := do(srv, http.MethodPut, "/v1/settings", `{"notifications_enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, body %s", rec.Code, rec.Body)
	}
	if !st.notifyOff {
		t.Fatal("switch not turned off")
	}
	rec := do(srv, http.MethodGet, "/v1/settings", "")
	if !strings.Contains(rec.Body.String(), `"notifications_enabled":false`) {
		t.Errorf("get body = %s", rec.Body)
	}

	// A body without the field must not flip the switch either way.
	if rec := do(srv, http.MethodPut, "/v1/settings", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty put status = %d, want 400", rec.Code)
	}
}
