//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/store"

	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// SharedKeys groups rows by fingerprint in Go, after the SQL orders them, so
// the test checks the grouping as much as the query: one key in three
// repositories comes back as one key with three repositories, led by its most
// severe copy, and a key in one repository is not shared at all.
func TestSharedKeysGroupsByFingerprint(t *testing.T) {
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
		// Deleting the target cascades to its repositories, jobs and findings.
		_, _ = db.Pool().Exec(context.Background(), `DELETE FROM targets WHERE id = $1`, target.ID)
	})

	suffix := id.New().String()[:8]
	shared := "test-shared-" + id.New().String() + "-0000000000000000"
	alone := "test-alone-" + id.New().String()
	severities := []entity.Severity{entity.SeverityMedium, entity.SeverityCritical, entity.SeverityHigh}
	for i, sev := range severities {
		repo, err := db.UpsertRepository(ctx, entity.Repository{
			TargetID: target.ID, Owner: "insights-" + suffix, Name: string(rune('a' + i)),
			CloneURL: "https://github.com/insights/x.git", DefaultBranch: "main",
		})
		if err != nil {
			t.Fatal(err)
		}
		insert := func(fp string) {
			f := entity.Finding{
				ID: id.New(), ScanJobID: job.ID, RepositoryID: repo.ID, RuleID: "stripe-secret-key",
				ProviderSlug: "stripe", Fingerprint: fp, MaskedPreview: "sk_l…" + sev.String(),
				BlobOID: "b", CommitSHA: "c", FilePath: ".env", Severity: sev, Status: entity.FindingOpen,
				ReachableFromHEAD: i == 0, Metadata: map[string]string{},
			}
			if _, err := db.InsertFinding(ctx, f); err != nil {
				t.Fatal(err)
			}
		}
		insert(shared)
		if i == 0 {
			insert(alone)
		}
	}

	keys, total, err := db.SharedKeys(ctx, 2, 500)
	if err != nil {
		t.Fatal(err)
	}
	if total < len(keys) {
		t.Errorf("total %d is less than the %d keys returned", total, len(keys))
	}
	var found bool
	for _, k := range keys {
		switch k.KeyID {
		case shared[:16]:
			found = true
			if len(k.Repos) != 3 {
				t.Fatalf("shared key has %d repositories, want 3", len(k.Repos))
			}
			if k.Severity != entity.SeverityCritical.String() {
				t.Errorf("key severity %q, want the most severe copy (critical)", k.Severity)
			}
		case alone[:16]:
			t.Error("a key found in one repository was reported as shared")
		}
		if len(k.KeyID) > 16 {
			t.Errorf("key id %q is longer than the fingerprint prefix", k.KeyID)
		}
	}
	if !found {
		t.Fatal("shared key missing from the result")
	}
}

// The calendar buckets by UTC day: a commit just before midnight UTC on a
// leap day must land on that day, whatever the server's own time zone is.
func TestLeakCalendarBucketsByUTCDay(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	before, err := db.LeakCalendar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := func(days []store.LeakDay, day string) (int, int) {
		for _, d := range days {
			if d.Day == day {
				return d.Findings, d.Critical
			}
		}
		return 0, 0
	}
	const day = "1999-02-28"
	f0, c0 := count(before, day)

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
	repo, err := db.UpsertRepository(ctx, entity.Repository{
		TargetID: target.ID, Owner: "calendar-" + id.New().String()[:8], Name: "r",
		CloneURL: "https://github.com/calendar/r.git", DefaultBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 23:30 UTC, written with a +07:00 offset so it reads as the next day locally.
	at := time.Date(1999, 3, 1, 6, 30, 0, 0, time.FixedZone("ICT", 7*3600))
	for _, sev := range []entity.Severity{entity.SeverityCritical, entity.SeverityLow} {
		if _, err := db.InsertFinding(ctx, entity.Finding{
			ID: id.New(), ScanJobID: job.ID, RepositoryID: repo.ID, RuleID: "r", Fingerprint: "cal-" + id.New().String(),
			MaskedPreview: "sk_l…9c1d", BlobOID: "b", CommitSHA: "c", FilePath: "f", Severity: sev, Status: entity.FindingOpen,
			CommittedAt: at, Metadata: map[string]string{},
		}); err != nil {
			t.Fatal(err)
		}
	}
	after, err := db.LeakCalendar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f, c := count(after, day); f-f0 != 2 || c-c0 != 1 {
		t.Fatalf("%s: +%d findings, +%d critical; want +2 and +1", day, f-f0, c-c0)
	}
}
