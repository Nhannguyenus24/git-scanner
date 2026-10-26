package scan_test

import (
	"sort"
	"testing"

	"github.com/mservice/gitscanner/internal/scan"
)

// TestProvidersAreDerivedFromTheRuleset checks that the dashboard's provider
// list reflects the live ruleset: every provider a rule attributes to appears
// exactly once, ordered by slug, with the most serious of its rules' severities.
func TestProvidersAreDerivedFromTheRuleset(t *testing.T) {
	ps := scan.BootstrapRuleset().Providers()
	if len(ps) == 0 {
		t.Fatal("no providers derived from the bootstrap ruleset")
	}

	seen := map[string]bool{}
	slugs := make([]string, len(ps))
	for i, p := range ps {
		if seen[p.Slug] {
			t.Errorf("provider %q listed more than once", p.Slug)
		}
		seen[p.Slug] = true
		slugs[i] = p.Slug
		if p.Rules < 1 {
			t.Errorf("provider %q has %d rules, want >= 1", p.Slug, p.Rules)
		}
		switch p.Severity {
		case "critical", "high", "medium", "low":
		default:
			t.Errorf("provider %q has unexpected severity %q", p.Slug, p.Severity)
		}
	}

	if !sort.StringsAreSorted(slugs) {
		t.Errorf("providers are not sorted by slug: %v", slugs)
	}

	// Spot-check a few that must be present, including one of the newly added
	// platform providers and the multi-rule aggregation for stripe.
	for _, want := range []string{"aws", "gitlab", "stripe", "twilio"} {
		if !seen[want] {
			t.Errorf("expected provider %q to be supported", want)
		}
	}

	// Stripe has a live (critical) and a test (low) rule; the summary takes the
	// most serious, and counts both rules.
	for _, p := range ps {
		if p.Slug != "stripe" {
			continue
		}
		if p.Severity != "critical" {
			t.Errorf("stripe severity = %q, want critical", p.Severity)
		}
		if p.Rules < 2 {
			t.Errorf("stripe rules = %d, want >= 2", p.Rules)
		}
	}
}
