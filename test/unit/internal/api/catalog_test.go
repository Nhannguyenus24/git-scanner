package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/mservice/gitscanner/internal/api"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// fakeCatalog records the filter it was asked for and the triage it applied.
type fakeCatalog struct {
	filter     store.FindingFilter
	sealed     []byte // returned on the row when the filter asks to reveal
	statuses   map[entity.ID]entity.FindingStatus
	jobs       []store.JobSummary
	repoFilter store.RepositoryFilter
	page       store.Page
	scanFilter store.ScanFilter
}

func (c *fakeCatalog) ListScanJobs(_ context.Context, f store.ScanFilter) ([]store.JobSummary, int, error) {
	c.page, c.scanFilter = f.Page, f
	return c.jobs, len(c.jobs), nil
}

func (c *fakeCatalog) SearchFindings(_ context.Context, f store.FindingFilter) ([]store.FindingRow, int, error) {
	c.filter = f
	row := store.FindingRow{ID: "f1", RepositoryID: "r1", RepoOwner: "o", RepoName: "r", Severity: "high", MaskedPreview: "sk_t…7890"}
	if f.Reveal && c.sealed != nil {
		row.Fingerprint, row.Sealed = "fp1", c.sealed
	}
	return []store.FindingRow{row}, 7, nil
}

func (c *fakeCatalog) SetFindingStatus(_ context.Context, id entity.ID, s entity.FindingStatus) error {
	if _, ok := c.statuses[id]; !ok {
		return store.ErrNotFound
	}
	c.statuses[id] = s
	return nil
}

func (c *fakeCatalog) ListRepositories(_ context.Context, f store.RepositoryFilter) ([]store.RepositorySummary, int, error) {
	c.repoFilter = f
	return []store.RepositorySummary{{ID: "r1", Owner: "acme", Name: "web", Open: 2, Critical: 1}}, 126102, nil
}

func (c *fakeCatalog) RepositorySummaryByID(_ context.Context, id entity.ID) (store.RepositorySummary, error) {
	if id != knownRepo {
		return store.RepositorySummary{}, store.ErrNotFound
	}
	return store.RepositorySummary{ID: id, Owner: "acme", Name: "web"}, nil
}

func (c *fakeCatalog) GetOverview(context.Context) (store.Overview, error) {
	return store.Overview{Findings: 3}, nil
}

func (c *fakeCatalog) ListProviderCandidates(_ context.Context, p store.Page) ([]entity.ProviderCandidate, int, error) {
	c.page = p
	return []entity.ProviderCandidate{{ID: "c1"}}, 824, nil
}

func (c *fakeCatalog) FindingsTimeline(context.Context) ([]store.TimePoint, error) {
	return []store.TimePoint{{Period: "2025-01", Findings: 2}}, nil
}
func (c *fakeCatalog) FindingsByAppType(context.Context) ([]store.Count, error) {
	return []store.Count{{Key: "node", Count: 3}}, nil
}
func (c *fakeCatalog) FindingsByRule(_ context.Context, _ int) ([]store.Count, error) {
	return []store.Count{{Key: "aws-access-key-id", Count: 1}}, nil
}
func (c *fakeCatalog) LiveProbeCounts(context.Context) ([]store.Count, error) {
	return []store.Count{{Key: "live", Count: 1}}, nil
}
func (c *fakeCatalog) NotificationCounts(context.Context) ([]store.Count, error) {
	return []store.Count{{Key: "sent", Count: 1}}, nil
}

const knownRepo = entity.ID("01a16d46-dc0a-7979-899f-fae3af3961b3")

const knownFinding = entity.ID("01a16d46-dc0a-7979-899f-fae3af3961b2")

func newCatalogServer(t *testing.T) (*api.Server, *fakeCatalog) {
	t.Helper()
	cat := &fakeCatalog{
		statuses: map[entity.ID]entity.FindingStatus{knownFinding: entity.FindingOpen},
		jobs: []store.JobSummary{{
			ScanJob:    entity.ScanJob{ID: "j1", State: entity.ScanCompleted, ReposTotal: 2, ReposScanned: 2},
			TargetKind: "user", TargetValue: "octocat",
		}},
	}
	srv := api.NewServer(newFakeStore(), &fakeQueue{}, &fakeControl{}, slog.New(slog.NewTextHandler(io.Discard, nil))).
		WithCatalog(cat)
	return srv, cat
}

// The dashboard lists jobs from the server; before this route it only knew the
// ids it had submitted itself, so scans from anywhere else never appeared.
func TestListScansCarriesTarget(t *testing.T) {
	srv, _ := newCatalogServer(t)
	rec := do(srv, http.MethodGet, "/v1/scans", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var resp struct {
		Scans []struct {
			ID     string `json:"id"`
			State  string `json:"state"`
			Target struct{ Kind, Value string }
		}
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Scans) != 1 || resp.Scans[0].Target.Value != "octocat" || resp.Scans[0].State != "completed" {
		t.Errorf("scans = %+v", resp.Scans)
	}
}

func TestSearchFindingsPassesFilter(t *testing.T) {
	srv, cat := newCatalogServer(t)
	rec := do(srv, http.MethodGet,
		"/v1/findings?severity=critical&provider=unmapped&status=open&q=env&limit=25&offset=50", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	want := store.FindingFilter{Severity: "critical", Provider: "unmapped", Status: "open", Query: "env", Limit: 25, Offset: 50}
	if cat.filter != want {
		t.Errorf("filter = %+v, want %+v", cat.filter, want)
	}
	var resp struct {
		Total    int `json:"total"`
		Findings []struct {
			Repository struct{ Owner, Name string }
		}
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Total != 7 || len(resp.Findings) != 1 || resp.Findings[0].Repository.Name != "r" {
		t.Errorf("response = %+v", resp)
	}
}

// A bad enum or id would otherwise reach SQL as a filter that matches nothing,
// or a uuid cast error reported as a 500.
func TestSearchFindingsRejectsBadFilters(t *testing.T) {
	srv, _ := newCatalogServer(t)
	for _, q := range []string{"severity=urgent", "status=fixed", "repository_id=x", "scan_id=1-2-3"} {
		if rec := do(srv, http.MethodGet, "/v1/findings?"+q, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func TestUpdateFindingStatus(t *testing.T) {
	srv, cat := newCatalogServer(t)
	path := "/v1/findings/" + string(knownFinding)

	if rec := do(srv, http.MethodPatch, path, `{"status":"false_positive"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if cat.statuses[knownFinding] != entity.FindingFalsePositive {
		t.Errorf("stored status = %s", cat.statuses[knownFinding])
	}

	cases := []struct {
		path, body string
		code       int
	}{
		{path, `{"status":"fixed"}`, http.StatusBadRequest},
		{path, `{"state":"open"}`, http.StatusBadRequest},
		{"/v1/findings/not-a-uuid", `{"status":"open"}`, http.StatusBadRequest},
		{"/v1/findings/00000000-0000-0000-0000-000000000000", `{"status":"open"}`, http.StatusNotFound},
	}
	for _, c := range cases {
		if rec := do(srv, http.MethodPatch, c.path, c.body); rec.Code != c.code {
			t.Errorf("%s %s: status = %d, want %d", c.path, c.body, rec.Code, c.code)
		}
	}
}

func TestCatalogRoutesNeedACatalog(t *testing.T) {
	srv, _, _ := newServer(t)
	if rec := do(srv, http.MethodGet, "/v1/overview", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 without a catalog", rec.Code)
	}
	srv, _ = newCatalogServer(t)
	for _, p := range []string{"/v1/overview", "/v1/repositories", "/v1/candidates"} {
		if rec := do(srv, http.MethodGet, p, ""); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d", p, rec.Code)
		}
	}
}

func TestAnalyticsRoute(t *testing.T) {
	srv, _ := newCatalogServer(t)
	rec := do(srv, http.MethodGet, "/v1/analytics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var resp struct {
		Timeline []struct {
			Period   string
			Findings int
		}
		ByAppType []struct {
			Key   string
			Count int
		} `json:"by_app_type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Timeline) != 1 || resp.Timeline[0].Period != "2025-01" || len(resp.ByAppType) != 1 {
		t.Errorf("analytics response = %+v", resp)
	}
}

type pageMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func TestRepositoriesArePagedFilteredAndSortedOnTheServer(t *testing.T) {
	srv, cat := newCatalogServer(t)
	rec := do(srv, http.MethodGet, "/v1/repositories?q=acme&only_open=true&sort=recent&limit=25&offset=50", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	want := store.RepositoryFilter{Query: "acme", OnlyOpen: true, Sort: "recent", Page: store.Page{Limit: 25, Offset: 50}}
	if cat.repoFilter != want {
		t.Errorf("filter %+v, want %+v", cat.repoFilter, want)
	}
	var got struct {
		pageMeta
		Repositories []struct{ ID string } `json:"repositories"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Total != 126102 || got.Limit != 25 || got.Offset != 50 || len(got.Repositories) != 1 {
		t.Errorf("body %s", rec.Body.String())
	}

	if rec := do(srv, http.MethodGet, "/v1/repositories?sort=stars", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown sort: status %d, want 400", rec.Code)
	}
}

func TestPageBoundsAreReportedAsApplied(t *testing.T) {
	srv, _ := newCatalogServer(t)
	for path, want := range map[string]pageMeta{
		"/v1/candidates":                    {Total: 824, Limit: store.DefaultPageSize},
		"/v1/candidates?limit=100000":       {Total: 824, Limit: store.MaxPageSize},
		"/v1/candidates?limit=-3&offset=-9": {Total: 824, Limit: store.DefaultPageSize},
		"/v1/scans?limit=10&offset=20":      {Total: 1, Limit: 10, Offset: 20},
	} {
		rec := do(srv, http.MethodGet, path, "")
		var got pageMeta
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != http.StatusOK || got != want {
			t.Errorf("%s: %d %+v, want %+v", path, rec.Code, got, want)
		}
	}
}

func TestGetRepository(t *testing.T) {
	srv, _ := newCatalogServer(t)
	if rec := do(srv, http.MethodGet, "/v1/repositories/"+knownRepo.String(), ""); rec.Code != http.StatusOK {
		t.Errorf("known: %d", rec.Code)
	}
	if rec := do(srv, http.MethodGet, "/v1/repositories/01a16d46-dc0a-7979-899f-fae3af396000", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", rec.Code)
	}
	if rec := do(srv, http.MethodGet, "/v1/repositories/not-a-uuid", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed: %d", rec.Code)
	}
}

func TestListScansFilters(t *testing.T) {
	srv, cat := newCatalogServer(t)
	if rec := do(srv, http.MethodGet, "/v1/scans?state=done&id="+knownRepo.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if cat.scanFilter.Group != "done" || cat.scanFilter.ID != knownRepo {
		t.Errorf("filter %+v", cat.scanFilter)
	}
	for _, q := range []string{"state=running", "id=nope"} {
		if rec := do(srv, http.MethodGet, "/v1/scans?"+q, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
		}
	}
}
