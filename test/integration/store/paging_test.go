//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// Four repositories under one unique owner, so the filter isolates them from
// whatever else the database holds: risk order follows the 10/5/2/1 weights
// (three highs, 15, outrank one critical, 10), pages do not overlap,
// the total counts every match, and only_open drops the clean one.
func TestListRepositoriesPagesFiltersAndSorts(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	target, err := db.CreateTarget(ctx, entity.Target{Kind: entity.TargetUser, Value: "octocat"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateScanJob(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), `DELETE FROM targets WHERE id = $1`, target.ID)
	})

	owner := "paging-" + id.New().String()[:8]
	// name -> severities of its open findings
	repos := map[string][]entity.Severity{
		"a-one-critical": {entity.SeverityCritical},                                       // weight 10
		"b-three-high":   {entity.SeverityHigh, entity.SeverityHigh, entity.SeverityHigh}, // weight 15
		"c-one-low":      {entity.SeverityLow},                                            // weight 1
		"d-clean":        nil,
	}
	ids := map[string]entity.ID{}
	for name, sevs := range repos {
		r, err := db.UpsertRepository(ctx, entity.Repository{
			TargetID: target.ID, Owner: owner, Name: name,
			CloneURL: "https://github.com/" + owner + "/" + name + ".git", DefaultBranch: "main",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = r.ID
		for _, sev := range sevs {
			if _, err := db.InsertFinding(ctx, entity.Finding{
				ID: id.New(), ScanJobID: job.ID, RepositoryID: r.ID, RuleID: "r", Fingerprint: "pg-" + id.New().String(),
				MaskedPreview: "sk_l…9c1d", BlobOID: "b", CommitSHA: "c", FilePath: "f", Severity: sev,
				Status: entity.FindingOpen, Metadata: map[string]string{},
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	names := func(rs []store.RepositorySummary) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Name
		}
		return out
	}
	list := func(f store.RepositoryFilter) ([]string, int) {
		t.Helper()
		if f.Query == "" {
			f.Query = owner
		}
		rs, total, err := db.ListRepositories(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return names(rs), total
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if got, total := list(store.RepositoryFilter{}); total != 4 || !eq(got, []string{"b-three-high", "a-one-critical", "c-one-low", "d-clean"}) {
		t.Errorf("risk order: %v (total %d)", got, total)
	}
	p1, _ := list(store.RepositoryFilter{Page: store.Page{Limit: 2}})
	p2, total := list(store.RepositoryFilter{Page: store.Page{Limit: 2, Offset: 2}})
	if !eq(append(p1, p2...), []string{"b-three-high", "a-one-critical", "c-one-low", "d-clean"}) || total != 4 {
		t.Errorf("pages %v + %v (total %d)", p1, p2, total)
	}
	if got, total := list(store.RepositoryFilter{OnlyOpen: true, Sort: "name"}); total != 3 || !eq(got, []string{"a-one-critical", "b-three-high", "c-one-low"}) {
		t.Errorf("only open by name: %v (total %d)", got, total)
	}
	// A LIKE wildcard in the search is literal.
	if _, total := list(store.RepositoryFilter{Query: "%"}); total != 0 {
		t.Errorf("'%%' matched %d repositories", total)
	}

	one, err := db.RepositorySummaryByID(ctx, ids["b-three-high"])
	if err != nil || one.High != 3 || one.Open != 3 {
		t.Errorf("by id: %+v, %v", one, err)
	}
	if _, err := db.RepositorySummaryByID(ctx, id.New()); err != store.ErrNotFound {
		t.Errorf("missing id: %v", err)
	}

	if js, n, err := db.ListScanJobs(ctx, store.ScanFilter{ID: job.ID, Group: "active"}); err != nil || n != 1 || len(js) != 1 {
		t.Errorf("queued job under active: %d of %d, %v", len(js), n, err)
	}
	if _, n, err := db.ListScanJobs(ctx, store.ScanFilter{ID: job.ID, Group: "done"}); err != nil || n != 0 {
		t.Errorf("queued job under done: %d, %v", n, err)
	}

	page, total, err := db.ListFindingsPage(ctx, job.ID, store.Page{Limit: 2, Offset: 4})
	if err != nil || total != 5 || len(page) != 1 {
		t.Errorf("scan findings page: %d rows of %d, %v", len(page), total, err)
	}
}
