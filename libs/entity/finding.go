package entity

import (
	"errors"
	"time"
)

// FindingStatus tracks remediation.
type FindingStatus string

const (
	FindingOpen          FindingStatus = "open"
	FindingRotated       FindingStatus = "rotated"
	FindingFalsePositive FindingStatus = "false_positive"
	FindingIgnored       FindingStatus = "ignored"
)

// Finding is one detected credential.
//
// It deliberately has no field for the secret itself. Per plan.md §6 the row
// carries a keyed HMAC fingerprint for dedupe and a masked preview for humans.
// The plaintext is never persisted here; an ownership-verified flow that needs
// it uses envelope encryption in a separate table with a TTL.
type Finding struct {
	ID           ID
	ScanJobID    ID
	RepositoryID ID

	RuleID       string // which detection rule fired
	ProviderSlug string // resolved against the registry; empty until enriched

	// Fingerprint is HMAC-SHA256(secret) hex. Stable across scans, so it is both
	// the dedupe key and the idempotency key for redelivered tasks.
	Fingerprint string
	// MaskedPreview is safe to display and to put in an email, e.g. "sk_live_4f2a…9c1d".
	MaskedPreview string

	BlobOID    string
	CommitSHA  string
	FilePath   string
	LineNumber int

	AuthorName  string
	AuthorEmail string
	CommittedAt time.Time

	// ReachableFromHEAD distinguishes a live leak from one only in history.
	// It is a severity input (plan.md §0.2), not a reason to ignore the finding:
	// history is public either way and the credential still needs rotating.
	ReachableFromHEAD bool

	Severity   Severity
	Status     FindingStatus
	DetectedAt time.Time

	// Metadata holds offline-extracted fields from the registry's extract block
	// (account, key_id, region...), and the probe outcome (probe_status and
	// non-secret identity) when live probing is enabled.
	Metadata map[string]string
}

func (f Finding) Validate() error {
	var errs []error

	if f.Fingerprint == "" {
		errs = append(errs, errors.New("fingerprint is required"))
	}
	if f.RuleID == "" {
		errs = append(errs, errors.New("rule id is required"))
	}
	if f.BlobOID == "" {
		errs = append(errs, errors.New("blob oid is required"))
	}
	if looksLikeSecret(f.MaskedPreview) {
		errs = append(errs, errors.New("masked preview appears to contain an unmasked value"))
	}

	return errors.Join(errs...)
}

// looksLikeSecret is a guard against a masking bug writing plaintext into the
// preview column. A correct preview always contains the ellipsis separator.
func looksLikeSecret(preview string) bool {
	if preview == "" {
		return false
	}
	for _, r := range preview {
		if r == '…' {
			return false
		}
	}
	return true
}
