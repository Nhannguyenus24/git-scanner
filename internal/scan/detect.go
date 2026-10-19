package scan

import (
	"bytes"
	"regexp"
	"strings"
)

// Match is one detected credential inside a blob, before any storage form is
// derived. Secret is the raw value and never leaves the engine: the worker
// turns it into a fingerprint and a masked preview immediately (plan.md §6).
type Match struct {
	RuleID   string
	Provider string
	Severity string

	Secret string
	Line   int // 1-based line of the secret within the blob

	// Context for the enricher's shape signature and for the local-config
	// filter: the environment-variable name and host near the match.
	NearbyEnvVar string
	NearbyHost   string
}

// Detector runs a ruleset plus the generic entropy pass over blob contents and
// applies the false-positive filters (plan.md §4.1 steps 4–6). It holds no
// mutable state, so one Detector is safe to share across the worker's goroutines.
type Detector struct {
	rs *Ruleset

	// minEntropy is the threshold for the generic high-entropy detector. Tuned
	// against the labelled corpus in Phase 8; the bootstrap value is deliberately
	// conservative to keep the false-positive rate low before that tuning exists.
	minEntropy float64
}

// NewDetector builds a detector over rs.
func NewDetector(rs *Ruleset) *Detector {
	return &Detector{rs: rs, minEntropy: 4.0}
}

// Ruleset returns the ruleset backing this detector (for the active version).
func (d *Detector) Ruleset() *Ruleset { return d.rs }

// envAssign captures the identifier on the left of an assignment, so a match
// can be attributed to FOO_API_KEY=... whether the file is env, YAML or code.
var envAssign = regexp.MustCompile(`(?i)([A-Z][A-Z0-9_]{2,})\s*[:=]`)

// Detect returns every credential found in blob. path is used only to skip
// files that structurally cannot hold a real secret (plan.md §4.1 step 6).
func (d *Detector) Detect(path string, blob []byte) []Match {
	if isBinary(blob) || deniedPath(path) {
		return nil
	}

	var matches []Match
	seen := map[string]struct{}{}  // dedupe identical secrets within one blob
	named := map[string]struct{}{} // values a named rule already claimed

	for _, rule := range d.rs.candidates(blob) {
		lo, hi, ok := rule.secretIndex(blob)
		if !ok {
			continue
		}
		secret := string(blob[lo:hi])
		if _, dup := seen[rule.ID+"\x00"+secret]; dup {
			continue
		}
		line := lineContaining(blob, lo)
		if !rule.keywordsPresent(line) || isPlaceholder(secret) {
			continue
		}
		seen[rule.ID+"\x00"+secret] = struct{}{}
		named[secret] = struct{}{}

		matches = append(matches, Match{
			RuleID:       rule.ID,
			Provider:     rule.Provider,
			Severity:     rule.Severity,
			Secret:       secret,
			Line:         lineNumber(blob, lo),
			NearbyEnvVar: envVarIn(line),
			NearbyHost:   firstHost(line),
		})
	}

	return append(matches, d.genericHighEntropy(blob, seen, named)...)
}

// highEntropyToken matches a long run of base64/hex-ish characters — the shape
// of most opaque API keys.
// Base64url/hex body with optional trailing = padding; = is never
// internal, so KEY=value splits into the key and the value rather than one token.
var highEntropyToken = regexp.MustCompile(`[A-Za-z0-9+/_-]{20,}={0,2}`)

// secretKeyword is the proximity signal: a high-entropy token is only reported
// as a generic secret when its line also names one of these, which keeps commit
// hashes, UUIDs and base64 asset blobs out of the results.
var secretKeyword = regexp.MustCompile(`(?i)(secret|token|passwo?rd|api[_-]?key|access[_-]?key|private[_-]?key|credential|auth)`)

// genericHighEntropy is the catch-all detector for opaque credentials no rule
// names (plan.md §4.1 step 5). A hit becomes a provider candidate downstream,
// which is how the registry learns which services are still unmapped (§5.4).
func (d *Detector) genericHighEntropy(blob []byte, seen, named map[string]struct{}) []Match {
	var out []Match
	for _, loc := range highEntropyToken.FindAllIndex(blob, -1) {
		token := string(blob[loc[0]:loc[1]])
		if _, dup := seen["generic\x00"+token]; dup {
			continue
		}
		if overlapsNamed(token, named) {
			continue // a named rule already reported this value
		}
		if shannonEntropy(token) < d.minEntropy || isPlaceholder(token) {
			continue
		}
		line := lineContaining(blob, loc[0])
		if !secretKeyword.MatchString(line) {
			continue
		}
		seen["generic\x00"+token] = struct{}{}
		out = append(out, Match{
			RuleID:       "generic-high-entropy",
			Provider:     "", // unmapped: the enricher turns this into a candidate
			Severity:     "medium",
			Secret:       token,
			Line:         lineNumber(blob, loc[0]),
			NearbyEnvVar: envVarIn(line),
			NearbyHost:   firstHost(line),
		})
	}
	return out
}

// overlapsNamed reports whether token is, contains, or sits inside a value a
// named rule already claimed, so a known key is not also reported as generic.
func overlapsNamed(token string, named map[string]struct{}) bool {
	for v := range named {
		if token == v || strings.Contains(token, v) || strings.Contains(v, token) {
			return true
		}
	}
	return false
}

// lineContaining returns the whole line that byte offset off falls on, used for
// keyword checks and context extraction.
func lineContaining(blob []byte, off int) string {
	start := bytes.LastIndexByte(blob[:off], '\n') + 1
	end := bytes.IndexByte(blob[off:], '\n')
	if end < 0 {
		return string(blob[start:])
	}
	return string(blob[start : off+end])
}

// lineNumber returns the 1-based line of byte offset off.
func lineNumber(blob []byte, off int) int {
	return 1 + bytes.Count(blob[:off], []byte{'\n'})
}

func envVarIn(line string) string {
	if m := envAssign.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// firstHost reuses the local-config host extraction so the enricher's shape
// signature and the notifier's severity both see the same host.
func firstHost(line string) string {
	if hosts := ReferencedHosts(line); len(hosts) > 0 {
		return hosts[0]
	}
	return ""
}

// isBinary sniffs the first 512 bytes for a NUL, the same heuristic git uses.
func isBinary(blob []byte) bool {
	n := min(len(blob), 512)
	return bytes.IndexByte(blob[:n], 0) >= 0
}

// placeholder matches the decoys that make up most false positives: example
// values, obvious repeats, and the classic "your key here" fillers.
var placeholder = regexp.MustCompile(`(?i)(example|changeme|your[-_]?(key|token|secret)|xxxx+|0000+|placeholder|dummy|sample|<[^>]+>)`)

func isPlaceholder(secret string) bool {
	if placeholder.MatchString(secret) {
		return true
	}
	// A value made of one repeated character (aaaa…, 1111…) is never a real key.
	if len(secret) > 3 {
		first := secret[0]
		same := true
		for i := 1; i < len(secret); i++ {
			if secret[i] != first {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}

// deniedPaths are directories and file shapes that hold vendored code, test
// fixtures or build output, where a "secret" is almost always a decoy or a
// third party's public example (plan.md §4.1 step 6).
var deniedDirs = []string{"vendor/", "node_modules/", "testdata/", ".git/"}

func deniedPath(path string) bool {
	p := strings.ToLower(path)
	for _, d := range deniedDirs {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return true
		}
	}
	return strings.HasSuffix(p, ".min.js") ||
		strings.HasSuffix(p, ".map") ||
		strings.HasSuffix(p, "lock") ||
		strings.HasSuffix(p, ".lock")
}
