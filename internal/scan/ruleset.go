package scan

import (
	"bytes"
	"sort"
)

// Ruleset is a compiled, immutable set of rules plus the literal prefilter
// built from them. Workers hold one behind an atomic pointer and swap in a new
// one on a registry publish without locking (plan.md §5.6); nothing here
// mutates after NewRuleset returns.
type Ruleset struct {
	version int64

	alwaysRules []*Rule // rules with no literal (entropy/keyword), always evaluated

	// literals maps each literal to the rules that declared it, so a prefilter
	// hit selects exactly the rules worth running.
	literals  map[string][]*Rule
	prefilter *prefilter
}

// NewRuleset compiles rules into a ruleset tagged with version.
func NewRuleset(version int64, rules []*Rule) *Ruleset {
	rs := &Ruleset{version: version, literals: map[string][]*Rule{}}

	var lits []string
	for _, r := range rules {
		if len(r.Literals) == 0 {
			rs.alwaysRules = append(rs.alwaysRules, r)
			continue
		}
		for _, lit := range r.Literals {
			if _, seen := rs.literals[lit]; !seen {
				lits = append(lits, lit)
			}
			rs.literals[lit] = append(rs.literals[lit], r)
		}
	}
	rs.prefilter = newPrefilter(lits)
	return rs
}

// Version is the registry version this ruleset was built from.
func (rs *Ruleset) Version() int64 { return rs.version }

// ProviderInfo summarises one provider's coverage in a ruleset: how many rules
// attribute to it and the most serious severity any of them carries. It backs
// the dashboard's "supported providers" list, so the list is derived from the
// live ruleset and cannot drift from what the engine actually detects.
type ProviderInfo struct {
	Slug     string
	Rules    int
	Severity string
	// Details describes each of the provider's rules, ordered by rule id.
	Details []RuleInfo
}

// Providers returns the distinct providers this ruleset can attribute a finding
// to, ordered by slug. A rule carried under several literals is counted once.
func (rs *Ruleset) Providers() []ProviderInfo {
	seen := map[*Rule]struct{}{}
	agg := map[string]*ProviderInfo{}
	add := func(r *Rule) {
		if _, dup := seen[r]; dup {
			return
		}
		seen[r] = struct{}{}
		p := agg[r.Provider]
		if p == nil {
			p = &ProviderInfo{Slug: r.Provider}
			agg[r.Provider] = p
		}
		p.Rules++
		p.Details = append(p.Details, r.Info())
		if severityRank(r.Severity) > severityRank(p.Severity) {
			p.Severity = r.Severity
		}
	}
	for _, r := range rs.alwaysRules {
		add(r)
	}
	// Map iteration order is random, but dedup-by-pointer and the final sort by
	// slug make the result deterministic regardless of the order rules arrive.
	for _, rules := range rs.literals {
		for _, r := range rules {
			add(r)
		}
	}
	out := make([]ProviderInfo, 0, len(agg))
	for _, p := range agg {
		sort.Slice(p.Details, func(i, j int) bool { return p.Details[i].ID < p.Details[j].ID })
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// severityRank orders severities so Providers can pick the most serious one; an
// unrecognised or empty string ranks below every real level.
func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// candidates returns the rules worth evaluating against blob: every rule whose
// literal the prefilter found, plus every literal-free rule. The result is
// deduplicated and ordered by rule ID so detection is deterministic.
func (rs *Ruleset) candidates(blob []byte) []*Rule {
	set := map[*Rule]struct{}{}
	for _, lit := range rs.prefilter.matches(blob) {
		for _, r := range rs.literals[lit] {
			set[r] = struct{}{}
		}
	}
	for _, r := range rs.alwaysRules {
		set[r] = struct{}{}
	}

	out := make([]*Rule, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// prefilter answers "which of these literals occur in a blob" in one pass.
//
// A production registry of ~400 rules wants Aho-Corasick here; the bootstrap
// set is small, so this does a bytes.Contains per distinct literal. The
// interface (newPrefilter/matches) is what the engine depends on, so swapping
// in the automaton later changes nothing above it.
type prefilter struct {
	literals [][]byte
	strs     []string
}

func newPrefilter(lits []string) *prefilter {
	p := &prefilter{}
	for _, l := range lits {
		p.literals = append(p.literals, []byte(l))
		p.strs = append(p.strs, l)
	}
	return p
}

func (p *prefilter) matches(blob []byte) []string {
	var hit []string
	for i, lit := range p.literals {
		if bytes.Contains(blob, lit) {
			hit = append(hit, p.strs[i])
		}
	}
	return hit
}
