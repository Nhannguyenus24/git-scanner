package entity

import (
	"errors"
	"fmt"
	"time"
)

// VersionState is the provider definition lifecycle (plan.md §5.6).
type VersionState string

const (
	VersionDraft      VersionState = "draft"
	VersionPublished  VersionState = "published"
	VersionDeprecated VersionState = "deprecated"
)

// Provider is a catalog entry: a service whose credentials we know how to
// recognise.
type Provider struct {
	ID             ID
	Slug           string // aws, stripe, cloudinary, azure_storage
	Name           string
	Category       string // cloud, payment, cdn, db, ai, messaging
	CurrentVersion int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProviderVersion is an immutable definition revision.
//
// DetectHash and ClassifyHash are stored separately on purpose (plan.md §5.7):
// comparing them against the previous version tells the publish step whether a
// change only reclassifies existing findings, or whether it introduces a new
// pattern that requires walking blobs again.
type ProviderVersion struct {
	ID         ID
	ProviderID ID
	Version    int
	State      VersionState

	Definition []byte // the YAML/JSON definition, stored verbatim

	DetectHash   string
	ClassifyHash string

	Author    string
	CreatedAt time.Time
	// PublishedAt is nil until the fixtures pass and it is published.
	PublishedAt *time.Time
}

// RequiresRescan reports whether moving from prev to v introduces new detection
// patterns. When only classification changed, enrichment is replayed over stored
// findings and no repository is cloned again.
func (v ProviderVersion) RequiresRescan(prev ProviderVersion) bool {
	return v.DetectHash != prev.DetectHash
}

// CandidateStatus tracks the unmapped backlog (plan.md §5.4).
type CandidateStatus string

const (
	CandidateNew     CandidateStatus = "new"
	CandidateTriaged CandidateStatus = "triaged"
	CandidateMapped  CandidateStatus = "mapped"
	CandidateIgnored CandidateStatus = "ignored"
)

// ProviderCandidate is a credential shape nothing in the catalog claims yet.
// These are what the "list services not yet mapped" endpoint returns.
type ProviderCandidate struct {
	ID ID

	// Signature clusters similar unknown secrets. Derived from charset, length,
	// leading token, nearby env var name and nearby hostname.
	Signature string

	// GuessedName comes mostly from the surrounding environment variable name,
	// which in practice names the missing provider outright:
	// CLOUDINARY_API_SECRET, AZURE_STORAGE_KEY, MAILGUN_API_KEY.
	GuessedName string

	EnvVarNames  []string
	NearbyHosts  []string
	SampleMasked string
	CharsetClass string
	LengthMin    int
	LengthMax    int

	Occurrences int
	RepoCount   int
	FirstSeenAt time.Time
	LastSeenAt  time.Time

	Status CandidateStatus
	// PromotedToSlug is set once someone turns this candidate into a definition.
	PromotedToSlug string
}

// Fixture is a synthetic test vector for a provider definition.
//
// Synthetic only. A real leaked credential must never be copied out of the
// findings table into a fixture: fixtures live in the repository and in CI logs,
// which is precisely where a live secret must not be.
type Fixture struct {
	ID             ID
	ProviderID     ID
	Name           string
	Input          string // a fake credential in the provider's format
	ShouldMatch    bool
	ExpectedFields map[string]string
}

func (p Provider) Validate() error {
	var errs []error

	if !validSlug(p.Slug) {
		errs = append(errs, fmt.Errorf("invalid provider slug %q: want lowercase letters, digits and underscore", p.Slug))
	}
	if p.Name == "" {
		errs = append(errs, errors.New("provider name is required"))
	}
	if p.Category == "" {
		errs = append(errs, errors.New("provider category is required"))
	}

	return errors.Join(errs...)
}

func validSlug(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
		if !ok {
			return false
		}
	}
	return true
}
