package scan

import (
	"fmt"
	"regexp"
	"strings"
)

// Rule is one detection pattern. It is the compiled, in-memory form of a
// registry detect block (plan.md §5.2); Phase 1 ships a built-in bootstrap set
// (bootstrap_rules.go) and Phase 3 loads the same shape from the registry.
type Rule struct {
	// ID is stable and is stored on every finding, so renaming one orphans its
	// history. Matches a provider slug once the registry owns the rule.
	ID       string
	Provider string
	Severity string

	// Literals are the fixed substrings a match must contain (AKIA, sk_live_,
	// ghp_). They feed the prefilter: a blob with none of them skips this rule
	// without ever running the regex (plan.md §4.1 step 4). A rule with no
	// literal always runs, which is why entropy rules are kept separate.
	Literals []string

	// Keywords, when set, must also appear somewhere in the surrounding text.
	// They cut false positives on short, generic patterns.
	Keywords []string

	re *regexp.Regexp
}

// compile builds the regular expression. Go's regexp is RE2, so there is no
// catastrophic backtracking, but a pattern must still carry a capture group
// named "secret" so the engine knows which span is the credential.
func (r *Rule) compile(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("scan: rule %s: %w", r.ID, err)
	}
	if idx := re.SubexpIndex("secret"); idx < 0 {
		return fmt.Errorf("scan: rule %s: pattern has no (?P<secret>...) group", r.ID)
	}
	r.re = re
	return nil
}

// maxMatchesPerRule bounds how many matches one rule yields from one blob. A
// limit is needed because a generated file can hold thousands of URL-shaped
// strings; 64 is far beyond any real config file.
const maxMatchesPerRule = 64

// secretIndexes returns the byte range of the captured secret for every match
// in b. It used to return only the first, which was harmless while each file
// held one key of a kind — but a config with a localhost URL above the
// production one had the production URL hidden behind the dropped local one.
func (r *Rule) secretIndexes(b []byte) [][2]int {
	i := r.re.SubexpIndex("secret") * 2
	var out [][2]int
	for _, m := range r.re.FindAllSubmatchIndex(b, maxMatchesPerRule) {
		if i < 0 || m[i] < 0 {
			continue
		}
		out = append(out, [2]int{m[i], m[i+1]})
	}
	return out
}

// keywordsPresent reports whether every keyword appears in text (case-insensitive).
func (r *Rule) keywordsPresent(text string) bool {
	if len(r.Keywords) == 0 {
		return true
	}
	lower := strings.ToLower(text)
	for _, k := range r.Keywords {
		if !strings.Contains(lower, strings.ToLower(k)) {
			return false
		}
	}
	return true
}
