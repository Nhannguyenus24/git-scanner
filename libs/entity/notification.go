package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// NotificationState tracks one disclosure mail through its lifecycle.
type NotificationState string

const (
	NotificationPending    NotificationState = "pending"
	NotificationSent       NotificationState = "sent"
	NotificationFailed     NotificationState = "failed"
	NotificationSuppressed NotificationState = "suppressed"
)

// Notification is one responsible-disclosure mail about a repository.
//
// Per plan.md §0.3 a repository is contacted at most once: the database carries
// a unique partial index on (repository_id) for the pending and sent states, so
// the cap holds even when two notifier replicas race.
type Notification struct {
	ID           ID
	RepositoryID ID
	Recipient    string
	FindingIDs   []ID

	State  NotificationState
	Error  string
	SentAt *time.Time

	CreatedAt time.Time
}

func (n Notification) Validate() error {
	var errs []error

	if n.RepositoryID == "" {
		errs = append(errs, errors.New("repository id is required"))
	}
	if !validEmail(n.Recipient) {
		errs = append(errs, fmt.Errorf("invalid recipient %q", n.Recipient))
	}
	if len(n.FindingIDs) == 0 {
		errs = append(errs, errors.New("notification needs at least one finding"))
	}

	return errors.Join(errs...)
}

// validEmail is a deliberately shallow check. It exists to catch an empty or
// obviously malformed address before it reaches the mail transport, not to
// decide deliverability — only sending can establish that.
func validEmail(s string) bool {
	if s == "" || len(s) > 254 || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	local, domain, found := strings.Cut(s, "@")
	if !found || local == "" || domain == "" {
		return false
	}
	return strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// SuppressionMatches reports whether an address is covered by a suppression
// pattern, which is either a full address or a bare domain.
//
// Matching is case-insensitive because mail domains are, and the domain form
// must match on a label boundary: "example.com" must not suppress
// "notexample.com".
func SuppressionMatches(pattern, recipient string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	r := strings.ToLower(strings.TrimSpace(recipient))
	if p == "" || r == "" {
		return false
	}
	if p == r {
		return true
	}

	_, domain, found := strings.Cut(r, "@")
	if !found {
		return false
	}
	return domain == p || strings.HasSuffix(domain, "."+p)
}
