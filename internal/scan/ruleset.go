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
