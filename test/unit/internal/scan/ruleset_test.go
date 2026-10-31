package scan_test

import (
	"sort"
	"strings"
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

// TestRuleDetailsDescribeEachRule checks the provider detail view's data: every
// rule is described, the anchor is read correctly off representative patterns,
// and the example is a drawing of the layout, not a value.
func TestRuleDetailsDescribeEachRule(t *testing.T) {
	byID := map[string]scan.RuleInfo{}
	for _, p := range scan.BootstrapRuleset().Providers() {
		if len(p.Details) != p.Rules {
			t.Errorf("%s: %d details for %d rules", p.Slug, len(p.Details), p.Rules)
		}
		for _, d := range p.Details {
			if d.Shape == "" || d.Example == "" {
				t.Errorf("%s: empty shape %q or example %q", d.ID, d.Shape, d.Example)
			}
			byID[d.ID] = d
		}
	}
	cases := []struct{ id, anchor, shape, example string }{
		// The parser factors AKIA|ASIA into A(KIA|SIA); it is still a prefix.
		{"aws-access-key-id", "prefix", "A(KIA|SIA)[0-9A-Z]{16}", "AKIA" + strings.Repeat("•", 16)},
		{"github-pat", "prefix", "gh[opsu]_[0-9A-Za-z]{36}", "gh•_" + strings.Repeat("•", 36)},
		// (?i) folding adds ſ and K to the class; they are not shown.
		{"cohere-api-key", "setting", "[0-9A-Za-z]{40}", strings.Repeat("•", 40)},
		{"mailchimp-api-key", "shape", "[0-9a-f]{32}-us[0-9]{1,2}", strings.Repeat("•", 32) + "-us•"},
		{"postgres-url", "url", "", ""},
		{"private-key-block", "block", "", ""},
	}
	for _, c := range cases {
		d, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: no details", c.id)
			continue
		}
		if d.Anchor != c.anchor {
			t.Errorf("%s: anchor %q, want %q", c.id, d.Anchor, c.anchor)
		}
		if c.shape != "" && d.Shape != c.shape {
			t.Errorf("%s: shape %q, want %q", c.id, d.Shape, c.shape)
		}
		if c.example != "" && d.Example != c.example {
			t.Errorf("%s: example %q, want %q", c.id, d.Example, c.example)
		}
	}
}
