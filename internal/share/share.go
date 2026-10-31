// Package share builds the read-only report behind a share link and renders it
// as JSON, Markdown and printable HTML.
//
// A share link points at someone else's leaked credentials, and a link can be
// forwarded, so what it reveals is decided here once rather than by each
// caller. Neither level ever carries a secret, a fingerprint, the committer's
// name or address, or anything a liveness probe learned about whose account a
// key opens. The summary level carries counts only. The full level adds what
// the disclosure mail already gives the owner — file, line, masked value and a
// link to the commit — which is enough to find the value in public history, and
// is why it is never the default.
package share

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/libs/entity"
)

// tokenBytes is the entropy of a link: 256 bits, so a link cannot be guessed
// and enumerating them is not a thing.
const tokenBytes = 32

// NewToken returns a fresh link token and the hash to store for it.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("share: token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, Hash(token), nil
}

// ErrBadToken is a token that cannot be one of ours. It is answered exactly
// like an unknown token.
var ErrBadToken = errors.New("share: malformed token")

// ParseToken checks a token's shape and returns its hash.
func ParseToken(token string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != tokenBytes {
		return nil, ErrBadToken
	}
	return Hash(token), nil
}

// Hash is what the database keeps instead of the token.
func Hash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// Report is everything a share link shows.
type Report struct {
	Repo            string             `json:"repo"`
	RepoURL         string             `json:"repo_url"`
	Detail          entity.ShareDetail `json:"detail"`
	IncludeResolved bool               `json:"include_resolved"`
	GeneratedAt     time.Time          `json:"generated_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
	LastScannedAt   *time.Time         `json:"last_scanned_at,omitempty"`
	Summary         Summary            `json:"summary"`
	// Findings and Guide are empty at the summary level.
	Findings []Item      `json:"findings"`
	Guide    []GuideStep `json:"guide"`
}

// Summary is safe at either level: numbers, no locations.
type Summary struct {
	Total      int     `json:"total"`
	Open       int     `json:"open"`
	Live       int     `json:"live"`        // confirmed live by a probe
	OnHead     int     `json:"on_head"`     // still on the default branch
	Files      int     `json:"files"`       // distinct files involved
	BySeverity []Count `json:"by_severity"` // most severe first
	ByProvider []Count `json:"by_provider"` // largest first
	ByStatus   []Count `json:"by_status"`
}

type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// Item is one finding at the full level.
type Item struct {
	Provider    string `json:"provider"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	Location    string `json:"location"` // path:line
	Masked      string `json:"masked"`
	Commit      string `json:"commit,omitempty"`       // short sha
	CommittedAt string `json:"committed_at,omitempty"` // YYYY-MM-DD
	Link        string `json:"link,omitempty"`
	OnHead      bool   `json:"on_head"`
	Live        bool   `json:"live"`
}

type GuideStep struct {
	Title    string   `json:"title"`
	Text     []string `json:"text"`
	Commands []string `json:"commands"`
}

// Build assembles the report a link shows, from the repository and its
// findings, at the link's level.
func Build(repo entity.Repository, findings []entity.Finding, link entity.ShareLink, now time.Time) Report {
	shown := make([]entity.Finding, 0, len(findings))
	for _, f := range findings {
		if link.IncludeResolved || f.Status == entity.FindingOpen {
			shown = append(shown, f)
		}
	}
	shown = notify.OrderForReport(shown)

	r := Report{
		Repo: repo.FullName(), RepoURL: "https://github.com/" + repo.FullName(),
		Detail: link.Detail, IncludeResolved: link.IncludeResolved,
		GeneratedAt: now.UTC(), ExpiresAt: link.ExpiresAt.UTC(), LastScannedAt: repo.LastScannedAt,
		Summary:  summarise(shown),
		Findings: []Item{}, Guide: []GuideStep{},
	}
	if link.Detail != entity.ShareFull {
		return r
	}

	for _, f := range shown {
		it := Item{
			Provider: notify.ProviderName(f), Rule: f.RuleID,
			Severity: f.Severity.String(), Status: string(f.Status),
			Location: f.FilePath, Masked: f.MaskedPreview,
			Link: notify.FindingLink(repo, f), OnHead: f.ReachableFromHEAD,
			Live: f.Metadata["probe_status"] == "live",
		}
		if f.LineNumber > 0 {
			it.Location += fmt.Sprintf(":%d", f.LineNumber)
		}
		if f.CommitSHA != "" {
			it.Commit = f.CommitSHA[:min(12, len(f.CommitSHA))]
		}
		if !f.CommittedAt.IsZero() {
			it.CommittedAt = f.CommittedAt.UTC().Format("2006-01-02")
		}
		r.Findings = append(r.Findings, it)
	}
	// The removal advice is for what is still open; a resolved finding needs
	// none, and its paths would only lengthen the commands.
	var open []entity.Finding
	for _, f := range shown {
		if f.Status == entity.FindingOpen {
			open = append(open, f)
		}
	}
	if len(open) > 0 {
		for _, s := range notify.RemovalGuide(repo, open) {
			r.Guide = append(r.Guide, GuideStep{Title: s.Title, Text: nonNil(s.Text), Commands: nonNil(s.Commands)})
		}
	}
	return r
}

var severityOrder = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

func summarise(fs []entity.Finding) Summary {
	s := Summary{Total: len(fs)}
	sev, prov, status := map[string]int{}, map[string]int{}, map[string]int{}
	files := map[string]bool{}
	for _, f := range fs {
		if f.Status == entity.FindingOpen {
			s.Open++
		}
		if f.Metadata["probe_status"] == "live" {
			s.Live++
		}
		if f.ReachableFromHEAD {
			s.OnHead++
		}
		files[path.Clean(f.FilePath)] = true
		sev[f.Severity.String()]++
		prov[notify.ProviderName(f)]++
		status[string(f.Status)]++
	}
	s.Files = len(files)
	s.BySeverity = counts(sev, func(a, b string) bool { return severityOrder[a] < severityOrder[b] })
	s.ByProvider = counts(prov, nil)
	s.ByStatus = counts(status, nil)
	return s
}

// counts orders by less when given, else largest first and then by name.
func counts(m map[string]int, less func(a, b string) bool) []Count {
	out := make([]Count, 0, len(m))
	for k, n := range m {
		out = append(out, Count{k, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if less != nil {
			return less(out[i].Key, out[j].Key)
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
