package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mservice/gitscanner/internal/api"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// fakeStore is an in-memory Store for the handler tests.
type fakeStore struct {
	jobs     map[entity.ID]entity.ScanJob
	findings map[entity.ID][]entity.Finding
}

func newFakeStore() *fakeStore {
	return &fakeStore{jobs: map[entity.ID]entity.ScanJob{}, findings: map[entity.ID][]entity.Finding{}}
}

func (s *fakeStore) CreateTarget(_ context.Context, t entity.Target) (entity.Target, error) {
	t.ID = id.New()
	return t, nil
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

type fakeQueue struct{ enqueued int }

func (q *fakeQueue) EnqueueScan(context.Context, entity.ScanJob, entity.Repository) error {
	q.enqueued++
	return nil
}

func newServer(t *testing.T) (*api.Server, *fakeStore, *fakeQueue) {
	t.Helper()
	st, q := newFakeStore(), &fakeQueue{}
	return api.NewServer(st, q, slog.New(slog.NewTextHandler(io.Discard, nil))), st, q
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
	if q.enqueued != 1 {
		t.Errorf("enqueued = %d, want 1", q.enqueued)
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
	if q.enqueued != 0 {
		t.Errorf("a bad request was enqueued (%d)", q.enqueued)
	}
}

func TestGetScanNotFound(t *testing.T) {
	srv, _, _ := newServer(t)
	rec := do(srv, http.MethodGet, "/v1/scans/"+id.New().String(), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
