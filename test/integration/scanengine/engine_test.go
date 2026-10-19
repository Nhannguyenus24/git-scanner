//go:build integration

// Scans a real git repository built on the fly, exercising the git CLI path
// (clone --mirror, rev-list, cat-file). Run with: make test-integration
package scanengine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/entity"
)

type sink struct{ f []entity.Finding }

func (s *sink) Emit(_ context.Context, f entity.Finding) error { s.f = append(s.f, f); return nil }

// buildRepo creates a git repo: a secret committed then deleted (history only),
// a secret still at HEAD, a decoy, and a go.mod for app-type detection.
func buildRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("go.mod", "module example.com/app\n")
	write("old.env", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLX\n")
	run("add", "-A")
	run("commit", "-qm", "first")

	// Remove the historical secret, add a live one and a decoy.
	_ = os.Remove(filepath.Join(dir, "old.env"))
	stripeKey := "sk_live" + "_0123456789abcdefghijklmn"
	stripePlaceholder := "sk_live" + "_your_key_here_placeholder0"
	write("config.env", "STRIPE_SECRET_KEY="+stripeKey+"\nEXAMPLE="+stripePlaceholder+"\n")
	run("add", "-A")
	run("commit", "-qm", "second")
	return dir
}

func TestEngineScansRealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repoDir := buildRepo(t)

	h, err := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	eng := scan.NewEngine(&scan.GitCLI{ScratchDir: t.TempDir(), AllowProtocols: []string{"file"}},
		scan.NewDetector(scan.BootstrapRuleset()), h)

	var s sink
	res, err := eng.Scan(context.Background(), "job", "repo", "file://"+repoDir, &s)
	if err != nil {
		t.Fatal(err)
	}

	byRule := map[string]entity.Finding{}
	for _, f := range s.f {
		byRule[f.RuleID] = f
	}

	// Both the historical AWS key and the live Stripe key are found.
	if _, ok := byRule["aws-access-key-id"]; !ok {
		t.Error("historical AWS key in a deleted file was not found")
	}
	stripe, ok := byRule["stripe-secret-key"]
	if !ok {
		t.Fatal("live Stripe key was not found")
	}
	// The placeholder decoy must not be reported.
	if len(s.f) != 2 {
		t.Errorf("findings = %d, want 2 (aws + stripe, no decoy)", len(s.f))
	}

	// Reachability: Stripe is at HEAD, AWS only in history.
	if !stripe.ReachableFromHEAD {
		t.Error("Stripe key at HEAD should be reachable")
	}
	if byRule["aws-access-key-id"].ReachableFromHEAD {
		t.Error("AWS key only in history must not be reachable from HEAD")
	}
	if res.AppType != "go" {
		t.Errorf("app type = %q, want go", res.AppType)
	}
}
