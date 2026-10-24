package scan_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/entity"
)

// fakeRepo is an in-memory repository, so the engine is tested without git.
type fakeRepo struct {
	blobs    []scan.Blob
	head     map[string]struct{}
	intro    map[string]scan.Commit
	introErr error
	closed   bool
}

func (r *fakeRepo) Blobs(_ context.Context, fn func(scan.Blob) error) error {
	for _, b := range r.blobs {
		if err := fn(b); err != nil {
			return err
		}
	}
	return nil
}
func (r *fakeRepo) HeadOIDs(context.Context) (map[string]struct{}, error) { return r.head, nil }
func (r *fakeRepo) Close() error                                          { r.closed = true; return nil }
func (r *fakeRepo) Introductions(_ context.Context, _ map[string]struct{}) (map[string]scan.Commit, error) {
	return r.intro, r.introErr
}

type fakeCloner struct{ repo *fakeRepo }

func (c *fakeCloner) Clone(context.Context, string) (scan.Repo, error) { return c.repo, nil }

type collectSink struct {
	mu sync.Mutex
	f  []entity.Finding
}

func (s *collectSink) Emit(_ context.Context, f entity.Finding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.f = append(s.f, f)
	return nil
}

func newEngine(t *testing.T, repo *fakeRepo) *scan.Engine {
	t.Helper()
	h, err := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	det := scan.NewDetector(scan.BootstrapRuleset())
	return scan.NewEngine(&fakeCloner{repo: repo}, det, h)
}

func TestEngineScanEmitsFindings(t *testing.T) {
	dummyStripeKey := "sk_live" + "_0123456789abcdefghijklmn"
	repo := &fakeRepo{
		blobs: []scan.Blob{
			{OID: "oid1", Path: ".env", Data: []byte("STRIPE=" + dummyStripeKey)},
			{OID: "oid2", Path: "go.mod", Data: []byte("module example.com/x")},
			{OID: "oid3", Path: "README.md", Data: []byte("nothing here")},
		},
		head: map[string]struct{}{"oid1": {}},
	}
	eng := newEngine(t, repo)
	sink := &collectSink{}

	res, err := eng.Scan(context.Background(), "job1", "repo1", "https://x/y.git", sink)
	if err != nil {
		t.Fatal(err)
	}

	if res.BlobsScanned != 3 {
		t.Errorf("blobs = %d, want 3", res.BlobsScanned)
	}
	if res.FindingsCount != 1 || len(sink.f) != 1 {
		t.Fatalf("findings = %d, want 1", res.FindingsCount)
	}
	if res.AppType != "go" {
		t.Errorf("app type = %q, want go", res.AppType)
	}
	if !repo.closed {
		t.Error("repository was not closed")
	}

	f := sink.f[0]
	if f.ProviderSlug != "stripe" || f.Severity != entity.SeverityCritical {
		t.Errorf("finding = %+v", f)
	}
	if !f.ReachableFromHEAD {
		t.Error("finding on oid1 should be reachable from HEAD")
	}
	if err := f.Validate(); err != nil {
		t.Errorf("emitted finding is invalid: %v", err)
	}
	if f.MaskedPreview == "" || f.Fingerprint == "" {
		t.Error("fingerprint and preview must be set")
	}
}

func TestEngineAttributesFindingsToCommits(t *testing.T) {
	when := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		blobs: []scan.Blob{
			{OID: "b1", Path: "renamed/.env", Data: []byte("token: ghp_0123456789abcdefghijklmnopqrstuvwxyz")},
			{OID: "b2", Path: "other.env", Data: []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLX")},
		},
		head:  map[string]struct{}{},
		intro: map[string]scan.Commit{"b1": {SHA: "c0ffee", AuthorName: "dev", AuthorEmail: "dev@x", When: when, Path: ".env"}},
	}
	f := scanFake(t, repo)
	byBlob := map[string]entity.Finding{}
	for _, x := range f {
		byBlob[x.BlobOID] = x
	}
	got := byBlob["b1"]
	if got.CommitSHA != "c0ffee" || got.AuthorName != "dev" || !got.CommittedAt.Equal(when) {
		t.Errorf("b1 attribution = %q %q %v", got.CommitSHA, got.AuthorName, got.CommittedAt)
	}
	// The introducing commit's path wins: it is the one that exists at that commit.
	if got.FilePath != ".env" {
		t.Errorf("b1 path = %q, want .env", got.FilePath)
	}
	// A blob the lookup could not place is still reported, just unattributed.
	if b2, ok := byBlob["b2"]; !ok || b2.CommitSHA != "" || b2.FilePath != "other.env" {
		t.Errorf("b2 = %+v", b2)
	}
}

// Losing provenance must not lose the finding.
func TestEngineEmitsWhenAttributionFails(t *testing.T) {
	repo := &fakeRepo{
		blobs:    []scan.Blob{{OID: "b1", Path: ".env", Data: []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLX")}},
		head:     map[string]struct{}{},
		introErr: errors.New("git log exploded"),
	}
	h, _ := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	eng := scan.NewEngine(&fakeCloner{repo: repo}, scan.NewDetector(scan.BootstrapRuleset()), h)
	var s collectSink
	res, err := eng.Scan(context.Background(), "job", "repo", "x", &s)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.f) != 1 || res.AttributionErr == nil {
		t.Errorf("findings = %d, attribution err = %v; want 1 and an error", len(s.f), res.AttributionErr)
	}
}

func scanFake(t *testing.T, repo *fakeRepo) []entity.Finding {
	t.Helper()
	h, _ := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	eng := scan.NewEngine(&fakeCloner{repo: repo}, scan.NewDetector(scan.BootstrapRuleset()), h)
	var s collectSink
	res, err := eng.Scan(context.Background(), "job", "repo", "x", &s)
	if err != nil {
		t.Fatal(err)
	}
	if res.AttributionErr != nil {
		t.Fatal(res.AttributionErr)
	}
	return s.f
}
