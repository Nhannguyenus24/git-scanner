package entity

import (
	"errors"
	"fmt"
	"time"
)

// ShareDetail is how much a share link reveals.
type ShareDetail string

const (
	// ShareSummary gives counts by severity, provider and status, and no
	// locations: safe to forward, not enough to find a credential.
	ShareSummary ShareDetail = "summary"
	// ShareFull adds each finding's file, line and commit link — what the
	// disclosure mail gives the owner, and enough to find the value in public
	// history. Meant for the owner or whoever will rotate it.
	ShareFull ShareDetail = "full"
)

// Share link lifetimes. Every link expires; a forgotten link to a leak in
// public history must not stay a map to it forever.
const (
	ShareDefaultDays = 7
	ShareMaxDays     = 30
)

// ShareLinkState is derived, never stored: a link is revoked, expired or active.
type ShareLinkState string

const (
	ShareActive  ShareLinkState = "active"
	ShareExpired ShareLinkState = "expired"
	ShareRevoked ShareLinkState = "revoked"
)

// ShareLink is a read-only link to one repository's results. The token itself
// is shown once, when the link is created; only its hash is kept.
type ShareLink struct {
	ID              ID
	RepositoryID    ID
	Detail          ShareDetail
	IncludeResolved bool
	CreatedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	Views           int
	LastViewedAt    *time.Time
}

// State reports whether the link still opens at now.
func (l ShareLink) State(now time.Time) ShareLinkState {
	switch {
	case l.RevokedAt != nil:
		return ShareRevoked
	case !now.Before(l.ExpiresAt):
		return ShareExpired
	}
	return ShareActive
}

// ParseShareDetail accepts the two levels; empty means the safer one.
func ParseShareDetail(s string) (ShareDetail, error) {
	switch ShareDetail(s) {
	case "", ShareSummary:
		return ShareSummary, nil
	case ShareFull:
		return ShareFull, nil
	}
	return "", fmt.Errorf("detail must be %q or %q", ShareSummary, ShareFull)
}

func (l ShareLink) Validate() error {
	var errs []error
	if l.RepositoryID == "" {
		errs = append(errs, errors.New("share link: repository is required"))
	}
	if l.Detail != ShareSummary && l.Detail != ShareFull {
		errs = append(errs, fmt.Errorf("share link: detail must be %q or %q", ShareSummary, ShareFull))
	}
	if !l.ExpiresAt.After(l.CreatedAt) {
		errs = append(errs, errors.New("share link: must expire after it is created"))
	}
	if l.ExpiresAt.Sub(l.CreatedAt) > ShareMaxDays*24*time.Hour+time.Minute {
		errs = append(errs, fmt.Errorf("share link: may last at most %d days", ShareMaxDays))
	}
	return errors.Join(errs...)
}
