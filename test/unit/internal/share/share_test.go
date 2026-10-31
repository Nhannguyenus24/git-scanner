package share_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/share"
	"github.com/mservice/gitscanner/libs/entity"
)

var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func repo() entity.Repository {
	return entity.Repository{Owner: "acme", Name: "billing", CloneURL: "https://github.com/acme/billing.git"}
}

// findings carries everything a report must never show alongside what it may.
func findings() []entity.Finding {
	return []entity.Finding{
		{
			RuleID: "stripe-secret-key", ProviderSlug: "stripe", Severity: entity.SeverityCritical,
			Status: entity.FindingOpen, FilePath: ".env", LineNumber: 3, MaskedPreview: "sk_l…9c1d",
			Fingerprint: "fp-secret-hmac", CommitSHA: "0123456789abcdef0123", CommittedAt: now.AddDate(0, -1, 0),
			AuthorName: "Jane Committer", AuthorEmail: "jane@example.com", ReachableFromHEAD: true,
			Metadata: map[string]string{"probe_status": "live", "probe_owner": "acct_owner_identity", "aws_account_id": "123456789012"},
		},
		{
			RuleID: "github-pat", ProviderSlug: "github", Severity: entity.SeverityHigh,
			Status: entity.FindingRotated, FilePath: "src/<script>alert(1)</script>.go", LineNumber: 9,
			MaskedPreview: "ghp_…abcd", CommitSHA: "fedcba9876543210", Metadata: map[string]string{},
		},
	}
}

// What no level may carry: the fingerprint, who committed it, and what a probe
// learned about whose account the key opens.
var neverShown = []string{"fp-secret-hmac", "Jane Committer", "jane@example.com", "acct_owner_identity", "123456789012"}

func render(t *testing.T, r share.Report) string {
	t.Helper()
	j, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	h, err := share.HTML(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(j) + share.Markdown(r) + h
}

func TestSummaryShowsCountsAndNoLocations(t *testing.T) {
	link := entity.ShareLink{Detail: entity.ShareSummary, ExpiresAt: now.Add(time.Hour), IncludeResolved: true}
	r := share.Build(repo(), findings(), link, now)

	if len(r.Findings) != 0 || len(r.Guide) != 0 {
		t.Fatalf("summary has %d findings and %d guide steps", len(r.Findings), len(r.Guide))
	}
	s := r.Summary
	if s.Total != 2 || s.Open != 1 || s.Live != 1 || s.OnHead != 1 || s.Files != 2 {
		t.Errorf("summary = %+v", s)
	}
	if s.BySeverity[0].Key != "critical" {
		t.Errorf("by severity = %+v, want critical first", s.BySeverity)
	}
	out := render(t, r)
	for _, bad := range append(neverShown, ".env", "sk_l…9c1d", "0123456789ab", "github.com/acme/billing/blob") {
		if strings.Contains(out, bad) {
			t.Errorf("summary output contains %q", bad)
		}
	}
}

func TestFullShowsLocationsButNoIdentity(t *testing.T) {
	link := entity.ShareLink{Detail: entity.ShareFull, ExpiresAt: now.Add(time.Hour), IncludeResolved: true}
	r := share.Build(repo(), findings(), link, now)
	if len(r.Findings) != 2 {
		t.Fatalf("findings = %d", len(r.Findings))
	}
	first := r.Findings[0]
	if first.Location != ".env:3" || first.Masked != "sk_l…9c1d" || !first.Live || first.Commit != "0123456789ab" ||
		first.Link != "https://github.com/acme/billing/blob/0123456789abcdef0123/.env#L3" {
		t.Errorf("first finding = %+v", first)
	}
	if len(r.Guide) == 0 {
		t.Error("no removal guide for an open finding")
	}
	out := render(t, r)
	for _, bad := range neverShown {
		if strings.Contains(out, bad) {
			t.Errorf("full output contains %q", bad)
		}
	}
	h, _ := share.HTML(r)
	if strings.Contains(h, "<script>alert(1)</script>") {
		t.Error("a file path was written into the HTML unescaped")
	}
	if !strings.Contains(h, `name="referrer" content="no-referrer"`) {
		t.Error("the HTML report would send its URL on as a referrer")
	}
}

func TestResolvedFindingsAreLeftOutUnlessAsked(t *testing.T) {
	link := entity.ShareLink{Detail: entity.ShareFull, ExpiresAt: now.Add(time.Hour)}
	r := share.Build(repo(), findings(), link, now)
	if r.Summary.Total != 1 || len(r.Findings) != 1 || r.Findings[0].Status != "open" {
		t.Errorf("summary %+v, findings %+v", r.Summary, r.Findings)
	}
}

func TestMarkdownTableSurvivesAwkwardValues(t *testing.T) {
	fs := []entity.Finding{{
		RuleID: "x", ProviderSlug: "a|b", Severity: entity.SeverityLow, Status: entity.FindingOpen,
		FilePath: "dir/we|ird`name.yml", MaskedPreview: "ab`c…wxyz", Metadata: map[string]string{},
	}}
	md := share.Markdown(share.Build(repo(), fs, entity.ShareLink{Detail: entity.ShareFull, ExpiresAt: now.Add(time.Hour)}, now))
	rows := 0
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, "dir/we") {
			rows++
		}
		if strings.HasPrefix(line, "| low") && strings.Contains(line, "dir/we") && strings.Count(strings.ReplaceAll(line, `\|`, ""), "|") != 7 {
			t.Errorf("row has the wrong number of cells: %s", line)
		}
	}
	if rows != 1 {
		t.Errorf("the finding spans %d lines, want 1", rows)
	}
}

func TestTokens(t *testing.T) {
	tok, hash, err := share.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) < 40 {
		t.Errorf("token %d characters, want 256 bits", len(tok))
	}
	got, err := share.ParseToken(tok)
	if err != nil || string(got) != string(hash) {
		t.Errorf("parse = %x, %v; want the hash", got, err)
	}
	other, _, _ := share.NewToken()
	if other == tok {
		t.Error("two tokens are equal")
	}
	for _, bad := range []string{"", "short", tok + "x", strings.Repeat("!", 43)} {
		if _, err := share.ParseToken(bad); err == nil {
			t.Errorf("ParseToken(%q) accepted", bad)
		}
	}
}
