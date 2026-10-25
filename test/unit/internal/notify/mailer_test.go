package notify_test

import (
	"strings"
	"testing"

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
