package notify_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/libs/entity"
)

func TestRenderDisclosureContentAndInvariant(t *testing.T) {
	repo := entity.Repository{Owner: "octocat", Name: "demo"}
	findings := []entity.Finding{{
		RuleID: "aws-access-key-id", ProviderSlug: "aws", Severity: entity.SeverityCritical,
		MaskedPreview: "AKIA…ZAM2", FilePath: ".env", LineNumber: 3, ReachableFromHEAD: true,
		Metadata: map[string]string{
			"provider_name": "Amazon Web Services", "category": "cloud",
			"probe_status": "live", "probe_account": "arn:aws:iam::0523:user/ci",
			"aws_account_id": "052310077262",
		},
	}}
	info := notify.DisclosureInfo{
		Project:   "GitScanner, a community secret-scanning project",
		Signature: "— Nhan, a software engineer in Vietnam",
		InfoURL:   "https://example.org/gitscanner",
	}
	m := notify.RenderDisclosure(repo, findings, info)
	body := m.Body

	must := []string{
		"octocat/demo", "community secret-scanning project", "AKIA…ZAM2",
		"Amazon Web Services", "category: cloud", "CONFIRMED LIVE",
		"arn:aws:iam::0523:user/ci", "052310077262",
		"Rotate", "still present on the default branch",
		"example.org/gitscanner", "software engineer in Vietnam",
	}
	for _, w := range must {
		if !strings.Contains(body, w) {
			t.Errorf("disclosure body missing %q\n---\n%s", w, body)
		}
	}

	// The invariant (plan.md §0.3): a disclosure mail never solicits. This is the
	// line that keeps a security notice from reading as leverage.
	for _, banned := range []string{"coffee", "donat", "sponsor", "paypal", "bounty", "patreon", "$"} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Errorf("disclosure body contains solicitation %q — forbidden", banned)
		}
	}
}

func TestRenderDisclosureDefaultsAndDeadStatus(t *testing.T) {
	m := notify.RenderDisclosure(
		entity.Repository{Owner: "a", Name: "b"},
		[]entity.Finding{{RuleID: "x", Severity: entity.SeverityLow, MaskedPreview: "xxxx…yyyy",
			Metadata: map[string]string{"probe_status": "dead"}}},
		notify.DisclosureInfo{}, // no project/signature configured
	)
	if !strings.Contains(m.Body, "an automated secret-scanning project") {
		t.Errorf("missing default project phrasing:\n%s", m.Body)
	}
	if !strings.Contains(m.Body, "provider rejected it") {
		t.Errorf("dead probe status not shown:\n%s", m.Body)
	}
}

// The mail has to be checkable without trusting its sender: each finding links
// to the line at the commit that introduced it.
func TestRenderDisclosureLinksToTheCommit(t *testing.T) {
	repo := entity.Repository{Owner: "octocat", Name: "demo", CloneURL: "https://github.com/octocat/demo.git"}
	f := entity.Finding{
		RuleID: "stripe-secret-key", Severity: entity.SeverityCritical, MaskedPreview: "sk_l…mn",
		FilePath: "config/.env", LineNumber: 4, CommitSHA: "0123456789abcdef0123",
		AuthorName: "Octo Cat", CommittedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	body := notify.RenderDisclosure(repo, []entity.Finding{f}, notify.DisclosureInfo{}).Body

	for _, w := range []string{
		"https://github.com/octocat/demo/blob/0123456789abcdef0123/config/.env#L4",
		"commit:   0123456789ab by Octo Cat on 2025-03-01",
		"rule:     stripe-secret-key",
	} {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q\n---\n%s", w, body)
		}
	}
}

// An env file is dropped from history whole; a source file keeps its other
// contents, so only the values are rewritten.
func TestRenderDisclosureRemovalGuideFitsTheFile(t *testing.T) {
	repo := entity.Repository{Owner: "a", Name: "b", CloneURL: "https://github.com/a/b.git"}
	findings := []entity.Finding{
		{RuleID: "x", Severity: entity.SeverityHigh, FilePath: ".env.production"},
		{RuleID: "y", Severity: entity.SeverityHigh, FilePath: "src/main.go"},
	}
	body := notify.RenderDisclosure(repo, findings, notify.DisclosureInfo{}).Body

	for _, w := range []string{
		"git clone https://github.com/a/b.git && cd b",
		"git filter-repo --invert-paths --path .env.production",
		"echo '.env.production' >> .gitignore",
		"git filter-repo --replace-text replacements.txt",
		"Files involved: src/main.go",
		"git push --force --all",
	} {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q\n---\n%s", w, body)
		}
	}
	if strings.Contains(body, "--path src/main.go") {
		t.Error("guide tells the reader to delete a source file from history")
	}
	m := notify.RenderDisclosure(repo, findings, notify.DisclosureInfo{})
	for _, part := range []string{m.Body, m.HTML} {
		if strings.Contains(part, "$") {
			t.Error("mail contains '$', which the solicitation check forbids")
		}
	}
	if !strings.Contains(m.HTML, "git filter-repo --invert-paths --path .env.production") {
		t.Error("HTML part lacks the removal commands")
	}
}
