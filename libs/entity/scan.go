package entity

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TargetKind is what a scan was asked to cover.
type TargetKind string

const (
	TargetRepo      TargetKind = "repo"
	TargetUser      TargetKind = "user"
	TargetOrg       TargetKind = "org"
	TargetAllowlist TargetKind = "allowlist"
	// TargetRandom scans randomly sampled public repositories. It deliberately
	// breaks the plan.md §0.1 rule that a bulk target is a bounded, authorized
	// set: it exists only to exercise the detector against unseen repositories
	// while tuning precision, and must not be used where disclosure mail or
	// liveness probing is enabled. Value carries the requested count as a
	// decimal string; "0" means run until paused ("forever").
	TargetRandom TargetKind = "random"
)

// Target is the thing a scan expands from. Per plan.md §0.1 a bulk target is
// always a bounded, authorized set: an org or an explicit allowlist, never
// "all of GitHub".
type Target struct {
	ID        ID
	Kind      TargetKind
	Value     string   // repo URL, username, org login
	Allowlist []string // only for TargetAllowlist
	CreatedAt time.Time
}

func (t Target) Validate() error {
	var errs []error

	switch t.Kind {
	case TargetRepo:
		if err := validateRepoURL(t.Value); err != nil {
			errs = append(errs, err)
		}
	case TargetUser, TargetOrg:
		if !validLogin(t.Value) {
			errs = append(errs, fmt.Errorf("invalid GitHub login %q", t.Value))
		}
	case TargetAllowlist:
		if len(t.Allowlist) == 0 {
			errs = append(errs, errors.New("allowlist target needs at least one entry"))
		}
		for _, u := range t.Allowlist {
			if err := validateRepoURL(u); err != nil {
				errs = append(errs, err)
			}
		}
	case TargetRandom:
		// Value is the requested repository count; "0" (or empty) means forever.
		if t.Value != "" {
			if n, err := strconv.Atoi(t.Value); err != nil || n < 0 {
				errs = append(errs, fmt.Errorf("random target count must be a non-negative integer, got %q", t.Value))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("unknown target kind %q", t.Kind))
	}

	return errors.Join(errs...)
}

// ScanState is the scan job state machine.
type ScanState string

const (
	ScanQueued    ScanState = "queued"
	ScanRunning   ScanState = "running"
	ScanPaused    ScanState = "paused"
	ScanCompleted ScanState = "completed"
	ScanFailed    ScanState = "failed"
	ScanCancelled ScanState = "cancelled"
)

// validTransitions is the whole state machine in one place. Keeping it as data
// means the API, the workers and the tests all agree on what is legal.
var validTransitions = map[ScanState][]ScanState{
	ScanQueued:    {ScanRunning, ScanCancelled},
	ScanRunning:   {ScanPaused, ScanCompleted, ScanFailed, ScanCancelled},
	ScanPaused:    {ScanRunning, ScanCancelled},
	ScanCompleted: {},
	ScanFailed:    {ScanQueued}, // retry
	ScanCancelled: {},
}

// CanTransition reports whether from -> to is a legal move.
func CanTransition(from, to ScanState) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Terminal reports whether no further transition is possible.
func (s ScanState) Terminal() bool { return len(validTransitions[s]) == 0 }

// ScanJob tracks one unit of scanning work and the cursor needed to resume it.
type ScanJob struct {
	ID       ID
	TargetID ID
	State    ScanState

	// Cursor is what pause/resume restores (plan.md §8). For a discovery job it
	// is the GitHub pagination cursor; for a repo scan it is the last committed
	// blob-walk position.
	Cursor string

	ReposTotal    int
	ReposScanned  int
	BlobsScanned  int
	FindingsCount int

	Error       string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RegistryVer int64 // registry version this job ran against (plan.md §5.6)
}

// Progress returns completion in [0,1]. Zero when the total is not yet known,
// which is normal while discovery is still paginating.
func (j ScanJob) Progress() float64 {
	if j.ReposTotal <= 0 {
		return 0
	}
	if j.ReposScanned >= j.ReposTotal {
		return 1
	}
	return float64(j.ReposScanned) / float64(j.ReposTotal)
}

// Repository is a repo discovered for scanning.
type Repository struct {
	ID            ID
	TargetID      ID
	Owner         string
	Name          string
	CloneURL      string
	DefaultBranch string
	IsFork        bool
	IsPrivate     bool
	Stars         int
	// RemoteCreatedAt is when the repository was created on its host. CreatedAt
	// below is when this system first discovered it; the two answer different
	// questions and a finding's age is judged against this one.
	RemoteCreatedAt *time.Time
	// AppType is the detected stack (plan.md §4.3), feeding severity scoring.
	AppType       string
	LastScannedAt *time.Time
	// HeadSHA at the last scan; lets the next run skip untouched repos.
	HeadSHA   string
	CreatedAt time.Time
}

func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

func validLogin(s string) bool {
	if s == "" || len(s) > 39 || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return false
	}
	for _, c := range s {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '-' {
			return false
		}
	}
	return true
}

// validateRepoURL rejects anything that is not an https clone URL.
//
// This is a security boundary, not formatting: the value reaches `git clone`, and
// git speaks ssh://, file:// and ext:: among others. ext:: in particular executes
// a command. Restricting to https here, together with protocol.allow=never at the
// git invocation (plan.md §13.3), closes that path.
func validateRepoURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid repo URL %q: %w", raw, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("repo URL %q must use https, got %q", raw, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("repo URL %q has no host", raw)
	}
	if u.User != nil {
		return fmt.Errorf("repo URL %q must not embed credentials", raw)
	}
	return nil
}
