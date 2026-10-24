package scan

import (
	"bytes"
	"regexp"
	"slices"
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

// secretAssignment matches the text just before a value that is being assigned
// to a secret-named setting, in properties, env, YAML, JSON or code form.
var secretAssignment = regexp.MustCompile(`(?i)(secret|passwo?r?d|token|api[_-]?key|private[_-]?key)["']?\s*[:=]\s*["']?$`)

// inURLPath matches when the bytes before a token run, unbroken, from a URL
// scheme: the token is a host or path segment. Userinfo and query values are
// excluded by the character class so user:pass@ and ?key= still get checked.
var inURLPath = regexp.MustCompile(`[a-z]+://[^\s"'<>()@?&=]*$`)

// hasImageMagic reports a base64-encoded PNG, JPEG or GIF, which is long,
// high-entropy and often sits on a line that happens to say "token".
func hasImageMagic(token string) bool {
	return strings.HasPrefix(token, "iVBORw0KGgo") ||
		strings.HasPrefix(token, "/9j/") ||
		strings.HasPrefix(token, "R0lGOD")
}

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
		before := blob[max(0, loc[0]-64):loc[0]]
		if bytes.HasSuffix(before, []byte("${")) {
			continue // a variable reference, not a value
		}
		if bytes.HasSuffix(before, []byte("base64,")) || hasImageMagic(token) {
			continue // an embedded image: a data: URI or a notebook's output cell
		}
		if inURLPath.Match(before) {
			continue // a CDN path or asset hash; credentials in URLs have named rules
		}
		// A wordy value assigned straight to a secret setting is a hardcoded
		// secret, weak or not — jwt.secret=mySecretKey… signs real tokens. Only
		// elsewhere on the line is a word-shaped token taken for an identifier.
		if looksLikeIdentifier(token) && !secretAssignment.Match(before) {
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

// looksLikeIdentifier reports whether token is built from words rather than
// random characters: a camelCase method name, a SCREAMING_SNAKE variable
// reference, a package or file path. These sit next to "token" and "password"
// all the time in source code and cleared the entropy bar easily, so before
// this check they made up most of the generic findings.
//
// The signal is segment length. Split on separators, case changes and
// letter/digit boundaries, words come out four or more characters long on
// average, while a random key fragments into runs of one or two. A vowel floor
// stops a long run of random letters from passing as a word.
func looksLikeIdentifier(token string) bool {
	var segments, letters, vowels int
	var prev rune
	for i, r := range token {
		isUpper, isLower := r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isUpper && !isLower && !isDigit {
			prev = 0 // a separator; the next character opens a new segment
			continue
		}
		prevUpper, prevLower := prev >= 'A' && prev <= 'Z', prev >= 'a' && prev <= 'z'
		prevDigit := prev >= '0' && prev <= '9'
		next := rune(0)
		if i+1 < len(token) {
			next = rune(token[i+1])
		}
		switch {
		case prev == 0,
			isUpper && prevLower, // fooBar
			isUpper && prevUpper && next >= 'a' && next <= 'z', // HTTPServer
			isDigit != prevDigit:                               // abc123
			segments++
		}
		if isUpper || isLower {
			letters++
			switch r | 0x20 {
			case 'a', 'e', 'i', 'o', 'u':
				vowels++
			}
		}
		prev = r
	}
	if segments < 2 || letters == 0 {
		return false
	}
	alnum := 0
	for _, r := range token {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			alnum++
		}
	}
	return float64(alnum)/float64(segments) >= 3.5 && float64(vowels)/float64(letters) >= 0.25
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

// charsetClass names the alphabet a secret is drawn from. Together with its
// length it is the shape the enricher clusters unmapped credentials on
// (plan.md §5.4): two AWS keys differ in value but share a shape.
//
// It is derived from the secret and is safe to persist. A class and a length
// describe the format a provider issues, not the value — nothing about the
// credential can be reconstructed from them.
func charsetClass(secret string) string {
	if secret == "" {
		return ""
	}

	var hasLetter, hasDigit, hasB64Sym, hasOther bool
	hex := true
	for _, r := range secret {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			hasLetter = true
		case r >= 'g' && r <= 'z', r >= 'G' && r <= 'Z':
			hasLetter, hex = true, false
		case r == '+', r == '/', r == '_', r == '-', r == '=':
			hasB64Sym, hex = true, false
		default:
			hasOther, hex = true, false
		}
	}

	switch {
	case hasOther:
		return "mixed"
	case hex && hasDigit:
		return "hex"
	case hasB64Sym:
		return "base64"
	case hasLetter && hasDigit:
		return "alnum"
	case hasLetter:
		return "alpha"
	default:
		return "numeric"
	}
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
// The "your…" arm allows words in between on purpose. Config templates spell
// the filler out in full — your-cloudinary-api-secret, YOUR_32_CHARACTER_KEY —
// and an arm that only accepted "your" immediately before the noun matched none
// of them, so those templates were reported as findings.
var placeholder = regexp.MustCompile(`(?i)(example|changeme|your[-_a-z0-9]*[-_](key|token|secret|password|pass)|xxxx+|0000+|placeholder|dummy|sample|<[^>]+>)`)

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
var deniedDirs = []string{"vendor/", "node_modules/", "testdata/", ".git/", ".idea/"}

// deniedFiles are generated manifests whose long tokens are integrity hashes
// and package paths. Every *.lock is already covered by the suffix check.
var deniedFiles = []string{"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "go.sum"}

func deniedPath(path string) bool {
	p := strings.ToLower(path)
	for _, d := range deniedDirs {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return true
		}
	}
	if slices.Contains(deniedFiles, p[strings.LastIndexByte(p, '/')+1:]) {
		return true
	}
	return strings.HasSuffix(p, ".min.js") ||
		strings.HasSuffix(p, ".svg") ||
		strings.HasSuffix(p, ".drawio") ||
		strings.HasSuffix(p, ".map") ||
		strings.HasSuffix(p, "lock") ||
		strings.HasSuffix(p, ".lock")
}
