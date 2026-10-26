package scan_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/entity"
)

func TestFileKind(t *testing.T) {
	cases := map[string]string{
		".env":                         "env",
		"config/.env.production":       "env",
		".env.example":                 "example",
		"examples/app/settings.py":     "example",
		"src/__tests__/client.test.ts": "test",
		"pkg/store/store_test.go":      "test",
		"deploy/id_rsa":                "credential_file",
		".npmrc":                       "credential_file",
		"keys/client_secret_123.json":  "credential_file",
		".github/workflows/deploy.yml": "ci",
		".gitlab-ci.yml":               "ci",
		"Dockerfile":                   "container",
		"docker-compose.prod.yml":      "container",
		"infra/main.tf":                "infra",
		"terraform.tfvars":             "infra",
		"k8s/secret.yaml":              "infra",
		"analysis/explore.ipynb":       "notebook",
		"config/application.yml":       "config",
		"appsettings.json":             "config",
		"scripts/deploy.sh":            "script",
		"README.md":                    "docs",
		"backup/dump.sql":              "data",
		"internal/api/server.go":       "source",
		"web/src/App.tsx":              "source",
		"LICENSE":                      "other",
		"runs/0f/full.jsonl":           "data",

		// Third-party code wins over everything, tests and config included.
		"venv/lib/python3.10/site-packages/Django-4.1.dist-info/RECORD": "dependency",
		".venv/Lib/site-packages/requests/tests/test_auth.py":           "dependency",
		"web/node_modules/pkg/config.json":                              "dependency",
		"pkg-1.0.egg-info/PKG-INFO":                                     "dependency",
	}
	for path, want := range cases {
		if got := scan.FileKind(path); got != want {
			t.Errorf("FileKind(%q) = %q, want %q", path, got, want)
		}
	}
}

// Every counter sits beside a decision Detect already made, so counting must
// not change what is reported, and each drop must land under its own gate.
func TestDetectCountedRecordsEachGate(t *testing.T) {
	det := scan.NewDetector(scan.BootstrapRuleset())
	blob := []byte(strings.Join([]string{
		"STRIPE_KEY=" + dummyStripeKey,
		// Matches the rule's shape, so it reaches — and is dropped by — the
		// placeholder gate rather than never being a candidate.
		"STRIPE_OTHER=sk_live" + "_" + strings.Repeat("x", 24),
		"API_TOKEN=${API_TOKEN_FROM_THE_ENVIRONMENT}",
	}, "\n"))

	var st scan.DetectStats
	counted := det.DetectCounted("config.env", blob, &st)
	plain := det.Detect("config.env", blob)
	if len(counted) != len(plain) {
		t.Fatalf("counted run reported %d, plain run %d", len(counted), len(plain))
	}

	m := st.Map()
	if m["named_reported"] != 1 {
		t.Errorf("named_reported = %d, want 1 (the live key)", m["named_reported"])
	}
	if m["named_drop_placeholder"] != 1 {
		t.Errorf("named_drop_placeholder = %d, want 1", m["named_drop_placeholder"])
	}
	if m["generic_candidates"] == 0 {
		t.Error("generic pass counted no candidates")
	}
	for k, v := range m {
		if v == 0 {
			t.Errorf("Map() returned zero counter %q", k)
		}
	}

	var bin scan.DetectStats
	det.DetectCounted("logo.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00"), &bin)
	if bin.Map()["blobs_binary"] != 1 {
		t.Errorf("binary blob not counted: %v", bin.Map())
	}
	var denied scan.DetectStats
	det.DetectCounted("node_modules/x/index.js", []byte("x"), &denied)
	if denied.Map()["blobs_denied_path"] != 1 {
		t.Errorf("denied path not counted: %v", denied.Map())
	}
}

func TestEngineExtractsKeyTypeModeAndRegion(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	h := func(n int) string { return strings.Repeat("3f9a0c7e1b", n/10+1)[:n] }
	cases := []struct {
		name, blob string
		want       map[string]string
	}{
		{"github fine-grained", "T=github_pat" + "_" + r(82), map[string]string{"key_type": "fine-grained-pat"}},
		{"github classic", "T=ghp" + "_" + r(36), map[string]string{"key_type": "classic-pat"}},
		{"openai project", "OPENAI_API_KEY=sk-proj-" + r(40) + "T3Blbk" + "FJ" + r(40), map[string]string{"key_type": "project"}},
		{"stripe test", "S=sk_test" + "_" + r(24), map[string]string{"key_type": "secret-key", "mode": "test"}},
		{"razorpay live", "K=rzp_live" + "_" + r(14), map[string]string{"mode": "live"}},
		{"flutterwave test", "K=FLWSECK_TEST-" + h(32), map[string]string{"mode": "test"}},
		{"mailchimp region", "MAILCHIMP_API_KEY=" + h(32) + "-us" + "14", map[string]string{"region": "us14"}},
		{"aws temporary", "AWS_ACCESS_KEY_ID=ASIA" + "QYLPMN5HHHFPZAM2", map[string]string{"key_type": "temporary"}},
	}
	for _, c := range cases {
		repo := &fakeRepo{blobs: []scan.Blob{{OID: "o", Path: ".env", Data: []byte(c.blob)}}}
		sink := &collectSink{}
		if _, err := newEngine(t, repo).Scan(context.Background(), "j", "r", "u", sink); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(sink.f) != 1 {
			t.Fatalf("%s: %d findings, want 1", c.name, len(sink.f))
		}
		for k, v := range c.want {
			if got := sink.f[0].Metadata[k]; got != v {
				t.Errorf("%s: %s = %q, want %q", c.name, k, got, v)
			}
		}
	}
}

// historyRepo is a fakeRepo that can also report removals and skipped blobs,
// the optional interfaces the git-backed repo implements.
type historyRepo struct {
	*fakeRepo
	removed map[string]scan.Commit
	skipped map[string]int
	asked   map[string]struct{}
}

func (r *historyRepo) Removals(_ context.Context, oids map[string]struct{}) (map[string]scan.Commit, error) {
	r.asked = oids
	return r.removed, nil
}
func (r *historyRepo) SkippedBlobs() map[string]int { return r.skipped }

type repoCloner struct{ repo scan.Repo }

func (c repoCloner) Clone(context.Context, string) (scan.Repo, error) { return c.repo, nil }

func TestEngineClassifiesIntroductionAndRemoval(t *testing.T) {
	when := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	repo := &historyRepo{
		fakeRepo: &fakeRepo{
			blobs: []scan.Blob{
				{OID: "live", Path: "app.env", Data: []byte("AWS_ACCESS_KEY_ID=AKIAQYLPMN5HHHFPZAM2")},
				{OID: "gone", Path: "old.env", Data: []byte("STRIPE=" + dummyStripeKey)},
				{OID: "keep", Path: "README.md", Data: []byte("x")},
			},
			head: map[string]struct{}{"live": {}, "keep": {}},
			intro: map[string]scan.Commit{
				"live": {SHA: "c1", Root: true, Subject: "whatever"},
				"gone": {SHA: "c2", Subject: "Merge pull request #4 from a/b"},
			},
		},
		removed: map[string]scan.Commit{
			"gone": {SHA: "c3", When: when, Subject: "oops, remove .env with the stripe secret"},
			// A removal for a blob still at HEAD must never be applied.
			"live": {SHA: "bogus", When: when},
		},
		skipped: map[string]int{"blobs_oversize": 2},
	}
	hasher, err := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	eng := scan.NewEngine(repoCloner{repo: repo}, scan.NewDetector(scan.BootstrapRuleset()), hasher)

	sink := &collectSink{}
	res, err := eng.Scan(context.Background(), "j", "r", "u", sink)
	if err != nil {
		t.Fatal(err)
	}
	if _, asked := repo.asked["live"]; asked {
		t.Error("removals asked for a blob still at HEAD")
	}

	byPath := map[string]entity.Finding{}
	for _, f := range sink.f {
		byPath[f.FilePath] = f
	}
	live, gone := byPath["app.env"], byPath["old.env"]
	if got := live.Metadata["intro_commit_kind"]; got != "initial" {
		t.Errorf("root commit intro kind = %q, want initial", got)
	}
	if live.RemovedCommitSHA != "" {
		t.Errorf("finding at HEAD stamped with removal %q", live.RemovedCommitSHA)
	}
	if got := gone.Metadata["intro_commit_kind"]; got != "merge" {
		t.Errorf("merge intro kind = %q, want merge", got)
	}
	if gone.RemovedCommitSHA != "c3" || !gone.RemovedAt.Equal(when) {
		t.Errorf("removal = %q at %v, want c3 at %v", gone.RemovedCommitSHA, gone.RemovedAt, when)
	}
	if got := gone.Metadata["removal_commit_kind"]; got != "cleanup" {
		t.Errorf("removal kind = %q, want cleanup", got)
	}
	if res.Stats["blobs_oversize"] != 2 || res.Stats["blobs_scanned"] != 3 {
		t.Errorf("stats = %v, want blobs_oversize 2 and blobs_scanned 3", res.Stats)
	}
}

// A credential kept across several versions of a file is one finding, not one
// per blob. Its introduction is the earliest copy's, it is at HEAD if any copy
// is, and otherwise it was removed when the last copy left the default branch.
// Before consolidation the stored row paired the newest copy's introduction
// with the oldest copy's removal — a credential "removed" before it arrived.
func TestEngineConsolidatesCopiesOfOneCredential(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	line := []byte("STRIPE=" + dummyStripeKey)
	blobs := []scan.Blob{
		{OID: "v3", Path: "app.properties", Data: append([]byte("# newest\n"), line...)},
		{OID: "v2", Path: "app.properties", Data: append([]byte("# middle\n\n"), line...)},
		{OID: "v1", Path: "app.properties", Data: line},
	}
	intro := map[string]scan.Commit{
		"v1": {SHA: "c1", When: day(1), Path: "app.properties"},
		"v2": {SHA: "c2", When: day(5), Path: "app.properties"},
		"v3": {SHA: "c3", When: day(9), Path: "app.properties"},
	}
	removed := map[string]scan.Commit{
		"v1": {SHA: "r1", When: day(5)},
		"v2": {SHA: "r2", When: day(9)},
		"v3": {SHA: "r3", When: day(12), Subject: "remove secret"},
	}
	hasher, err := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	scanWith := func(head map[string]struct{}) []entity.Finding {
		repo := &historyRepo{fakeRepo: &fakeRepo{blobs: blobs, head: head, intro: intro}, removed: removed}
		eng := scan.NewEngine(repoCloner{repo: repo}, scan.NewDetector(scan.BootstrapRuleset()), hasher)
		sink := &collectSink{}
		if _, err := eng.Scan(context.Background(), "j", "r", "u", sink); err != nil {
			t.Fatal(err)
		}
		return sink.f
	}

	gone := scanWith(map[string]struct{}{"other": {}})
	if len(gone) != 1 {
		t.Fatalf("emitted %d findings for one credential, want 1", len(gone))
	}
	f := gone[0]
	if f.CommitSHA != "c1" || !f.CommittedAt.Equal(day(1)) || f.BlobOID != "v1" || f.LineNumber != 1 {
		t.Errorf("introduction = %s %v blob %s line %d, want the earliest copy (c1, v1, line 1)",
			f.CommitSHA, f.CommittedAt, f.BlobOID, f.LineNumber)
	}
	if f.ReachableFromHEAD || f.RemovedCommitSHA != "r3" || !f.RemovedAt.Equal(day(12)) {
		t.Errorf("removal = %q at %v (reachable %v), want the last copy's r3", f.RemovedCommitSHA, f.RemovedAt, f.ReachableFromHEAD)
	}
	if f.Metadata["removal_commit_kind"] != "cleanup" {
		t.Errorf("removal kind = %q, want the last removal's (cleanup)", f.Metadata["removal_commit_kind"])
	}
	if f.RemovedAt.Before(f.CommittedAt) {
		t.Error("removed before it was introduced")
	}

	// One copy still at HEAD: the credential is live there, whatever happened
	// to the older copies.
	live := scanWith(map[string]struct{}{"v3": {}})
	if len(live) != 1 || !live[0].ReachableFromHEAD || live[0].RemovedCommitSHA != "" {
		t.Errorf("with v3 at HEAD: %+v, want one reachable finding with no removal", live)
	}
	if _, ok := live[0].Metadata["removal_commit_kind"]; ok {
		t.Error("reachable finding carries a removal kind")
	}
}
