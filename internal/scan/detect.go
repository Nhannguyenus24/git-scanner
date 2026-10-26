package scan

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
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

	// ProbeURL is the connection URL to validate liveness against, when it
	// differs from Secret — a database password split across variables is stored
	// as the password but probed as a full URL assembled from its siblings. It
	// holds the plaintext credential, so like Secret it never leaves the engine
	// and is never persisted. Empty means probe Secret itself.
	ProbeURL string
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
	return d.DetectCounted(path, blob, nil)
}

// DetectCounted is Detect, also counting into st what each gate did. st may be
// nil. It does not change what is detected: every counter sits beside a
// decision that was already there.
func (d *Detector) DetectCounted(path string, blob []byte, st *DetectStats) []Match {
	if isBinary(blob) {
		st.inc(statBlobBinary)
		return nil
	}
	if deniedPath(path) {
		st.inc(statBlobDeniedPath)
		return nil
	}

	var matches []Match
	seen := map[string]struct{}{}  // dedupe identical secrets within one blob
	named := map[string]struct{}{} // values a named rule already claimed

	for _, rule := range d.rs.candidates(blob) {
		for _, span := range rule.secretIndexes(blob) {
			st.inc(statNamedCandidate)
			lo, hi := span[0], span[1]
			secret := string(blob[lo:hi])
			if _, dup := seen[rule.ID+"\x00"+secret]; dup {
				st.inc(statNamedDuplicate)
				continue
			}
			if _, claimed := named[secret]; claimed {
				st.inc(statNamedDuplicate)
				continue // an earlier rule (by ID order) already reported this value
			}
			line := lineContaining(blob, lo)
			if !rule.keywordsPresent(keywordWindow(blob, lo, hi)) {
				st.inc(statNamedKeywordMissing)
				continue
			}
			if isPlaceholder(secret) {
				st.inc(statNamedPlaceholder)
				continue
			}
			if namedSuppressed(rule.ID, path, blob, lo, hi, secret) {
				st.inc(statNamedSuppressed)
				continue
			}
			seen[rule.ID+"\x00"+secret] = struct{}{}
			named[secret] = struct{}{}
			// Claimed before the local check, so the generic pass does not pick
			// the password back out of a URL that was dropped for being local.
			if hostScopedProviders[rule.Provider] && localURL(secret) {
				st.inc(statNamedLocalURL)
				continue
			}

			st.inc(statNamedReported)
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
	}

	// Example and template files hold values meant to be replaced. A named rule
	// still runs there — a real key pasted into .env.example is still a leak and
	// still has its own shape — but an unrecognised high-entropy string in one
	// is almost always a stand-in.
	if exampleFile(path) {
		st.inc(statGenericSkippedExampleFile)
		return matches
	}
	return append(matches, d.genericHighEntropy(path, blob, seen, named, st)...)
}

// highEntropyToken matches a long run of base64/hex-ish characters — the shape
// of most opaque API keys.
// Base64url/hex body with optional trailing = padding; = is never
// internal, so KEY=value splits into the key and the value rather than one token.
var highEntropyToken = regexp.MustCompile(`[A-Za-z0-9+/_-]{20,}={0,2}`)

// secretKeyword is the proximity signal: a high-entropy token is only reported
// as a generic secret when its line also names one of these, which keeps commit
// hashes, UUIDs and base64 asset blobs out of the results.
//
// Every alternative is a word that names a secret, not a word that merely sits
// near one. A bare "auth" used to be here and matched OAUTH, author and
// AuthenticationFilter — GOOGLE_OAUTH_CLIENT_ID=<public id> was reported as a
// secret for exactly that reason. Short words (pwd) are left to classifyKey,
// which matches whole words: as a substring, three letters turn up inside
// random base64 and URL parameters often enough to let junk through.
var secretKeyword = regexp.MustCompile(`(?i)(secret|token|passw(or)?d|api[_-]?key|access[_-]?key|private[_-]?key|credential|authorization|auth[_-]?(token|key))`)

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
func (d *Detector) genericHighEntropy(path string, blob []byte, seen, named map[string]struct{}, st *DetectStats) []Match {
	var out []Match
	for _, loc := range highEntropyToken.FindAllIndex(blob, -1) {
		st.inc(statGenericToken)
		token := string(blob[loc[0]:loc[1]])
		if _, dup := seen["generic\x00"+token]; dup {
			st.inc(statGenericDuplicate)
			continue
		}
		if overlapsNamed(token, named) {
			st.inc(statGenericDuplicate)
			continue // a named rule already reported this value
		}
		if shannonEntropy(token) < d.minEntropy {
			st.inc(statGenericLowEntropy)
			continue
		}
		if isPlaceholder(token) || isFakeValue(token) {
			st.inc(statGenericPlaceholder)
			continue
		}
		before := blob[max(0, loc[0]-64):loc[0]]
		if bytes.HasSuffix(before, []byte("${")) {
			st.inc(statGenericVariableRef)
			continue // a variable reference, not a value
		}
		if bytes.HasSuffix(before, []byte("base64,")) || hasImageMagic(token) {
			st.inc(statGenericEmbeddedImage)
			continue // an embedded image: a data: URI or a notebook's output cell
		}
		if inURLPath.Match(before) {
			st.inc(statGenericURLPath)
			continue // a CDN path or asset hash; credentials in URLs have named rules
		}
		// A wordy value assigned straight to a secret setting is a hardcoded
		// secret, weak or not. Only elsewhere on the line is a word-shaped token
		// taken for an identifier.
		if looksLikeIdentifier(token) && !secretAssignment.Match(before) {
			st.inc(statGenericIdentifier)
			continue
		}
		if isTestPath(path) {
			st.inc(statGenericTestPath)
			continue // fixtures; a named rule still runs in test files
		}
		if publicIdentifier(token, blob[loc[1]:min(len(blob), loc[1]+40)]) {
			st.inc(statGenericPublicIdentifier)
			continue // a value designed to be public: publishable key, OAuth client id
		}
		line := contextAround(blob, loc[0], loc[1])
		// The decisive question is what the value is assigned to. A key that
		// names a secret is enough on its own; a key that names a public
		// identifier rules the value out however secret-ish the line around it
		// looks; only with no key, or an uninformative one, does the presence of
		// a secret word nearby decide.
		key := assignmentKey(blob[max(0, loc[0]-96):loc[0]])
		switch classifyKey(key) {
		case keyPublic:
			st.inc(statGenericPublicKey)
			continue
		case keySecret:
		default:
			if !secretKeyword.MatchString(line) {
				st.inc(statGenericNoSecretKeyword)
				continue
			}
		}
		// A password for a host-scoped service (DB_PASSWORD, REDIS_PASS,
		// MONGO_PASSWORD) split across separate variables from its DB_HOST is a
		// local development value when every host named nearby is local — the
		// same case the URL rules drop via localURL, which cannot see here
		// because host and password are on different lines. Scoped to keys that
		// name such a service, and to a narrow neighbourhood, so a provider key
		// (caught by a named rule anyway) near a stray localhost is never hidden.
		neigh := neighbourhood(blob, loc[0], loc[1])
		if hostScopedKey(key) && allHostsDevLocal(neigh) {
			st.inc(statGenericDevLocal)
			continue
		}
		if exampleLine.MatchString(line) {
			st.inc(statGenericExampleLine)
			continue // documentation: `example = "eyJ…"`, **Example:**, e.g. …
		}
		if inJWT(blob, loc[0], loc[1]) || isJSONSegment(token) || jwtSecretContext(path, blob, loc[0], line) {
			st.inc(statGenericJWT)
			continue
		}
		st.inc(statGenericReported)
		seen["generic\x00"+token] = struct{}{}
		m := Match{
			RuleID:       "generic-high-entropy",
			Provider:     "", // unmapped: the enricher turns this into a candidate
			Severity:     "medium",
			Secret:       token,
			Line:         lineNumber(blob, loc[0]),
			NearbyEnvVar: envVarIn(line),
			NearbyHost:   firstHost(line),
		}
		// A DB/cache/broker password split from its host across variables can be
		// attributed and probed when the neighbourhood names the engine and a
		// routable host. The stored secret stays the password (its fingerprint
		// and preview); ProbeURL carries the assembled connection URL, which is
		// used only to validate liveness and is never persisted. Attribution is
		// made only when the engine is unambiguous, so a probe never guesses the
		// wrong protocol for a host.
		if hostScopedKey(key) {
			if c, ok := assembleDBConn(key, neigh, token); ok {
				m.RuleID = "db-split-env"
				m.Provider = c.engine
				m.Severity = "high"
				m.ProbeURL = c.url()
			}
		}
		out = append(out, m)
	}
	return out
}

// keywordSpan is how far a rule's keywords may sit from its match. Wider than a
// line on purpose: a value inside cloudinary.config({ ... }) or a recaptcha:
// YAML block has the word that names its provider a few lines above it.
const keywordSpan = 400

func keywordWindow(blob []byte, lo, hi int) string {
	return string(blob[max(0, lo-keywordSpan):min(len(blob), hi+keywordSpan)])
}

// namedSuppressed applies the checks a named rule's shape cannot express: a
// value with the right shape can still be a stand-in, or a credential that is
// public by design. Each case below was a false positive in the corpus.
func namedSuppressed(ruleID, path string, blob []byte, lo, hi int, secret string) bool {
	switch {
	case isFakeValue(secret):
		return true // the AWS documentation key and other published examples
	case isTestPath(path) && longestSequentialRun(secret) >= 8:
		// A synthetic fixture (ghp_0123456789…, cloudinary://123456789012345…)
		// in a test file. Scoped to test files and to an 8-character sequential
		// run, so a real key — which in production code is reported whatever it
		// contains — is never hidden by this.
		return true
	case strings.HasPrefix(ruleID, "private-key") && !hasKeyMaterial(blob[lo:min(len(blob), lo+800)]):
		return true // a PEM header with no key in it, e.g. a test-table literal
	case strings.Contains(secret, "://") && placeholderURLCredential(secret):
		return true // smtp://user:pass@host in a comment, ?password=${DB_PASS}
	case ruleID == "google-api-key" && firebaseClientContext(path, blob, lo, hi):
		return true // a Firebase client key, shipped inside every app on purpose
	}
	return false
}

// documentationKeys are published example credentials (AWS's own docs use
// AKIAIOSFODNN7EXAMPLE and its secret). Matched by prefix, so the variants
// people make by changing the last letter are caught too.
var documentationKeys = []string{"AKIAIOSFODNN7", "wJalrXUtnFEMI/K7MDENG"}

// isFakeValue reports a value with the shape of a credential but the content of
// a published example. It matches only exact documentation keys — a run-based
// heuristic (e.g. "contains 012345") was tried and dropped: a random key can
// contain a short sequential run, and suppressing it would hide a real secret,
// which is the one failure a scanner must never make.
func isFakeValue(v string) bool {
	for _, k := range documentationKeys {
		if strings.Contains(v, k) {
			return true
		}
	}
	return false
}

// longestSequentialRun is the longest stretch of letters or digits each one step
// from the last in the same direction, ignoring case: "0123" is 4, "dcba" is 4.
// Used only inside test files (see namedSuppressed) to spot synthetic fixtures;
// never applied to production code, where it would risk hiding a real key.
func longestSequentialRun(v string) int {
	if len(v) == 0 {
		return 0
	}
	best, run, dir := 1, 1, 0
	low := func(c byte) byte {
		if c >= 'A' && c <= 'Z' {
			return c + 32
		}
		return c
	}
	cls := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return 1
		case c >= 'a' && c <= 'z':
			return 2
		}
		return 0
	}
	for i := 1; i < len(v); i++ {
		a, b := low(v[i-1]), low(v[i])
		d := int(b) - int(a)
		if cls(a) != 0 && cls(a) == cls(b) && (d == 1 || d == -1) && (run == 1 || d == dir) {
			run, dir = run+1, d
		} else if cls(a) != 0 && cls(a) == cls(b) && (d == 1 || d == -1) {
			run, dir = 2, d
		} else {
			run, dir = 1, 0
		}
		best = max(best, run)
	}
	return best
}

// pemBody is a run of key material: base64 lines are 64 characters, so 40 is
// well inside a real one and longer than anything in a PEM header.
var pemBody = regexp.MustCompile(`[A-Za-z0-9+/=]{40,}`)

// hasKeyMaterial reports whether a private-key header is followed by a body.
// Legacy encrypted PEMs put Proc-Type/DEK-Info lines first, so the body is
// searched in the next few hundred bytes rather than right after the header.
func hasKeyMaterial(span []byte) bool {
	if i := bytes.Index(span, []byte("PRIVATE KEY-----")); i >= 0 {
		span = span[i+len("PRIVATE KEY-----"):]
	}
	return pemBody.Match(span)
}

// placeholderPasswords are the passwords people write when they mean "a
// password goes here".
var placeholderPasswords = map[string]bool{
	"pass": true, "password": true, "passwd": true, "pwd": true, "secret": true,
	"changeme": true, "xxx": true, "xxxx": true, "***": true, "****": true, "token": true,
}

// placeholderURLCredential reports scheme://user:password@host where the
// password is a stand-in, a template variable, or the same as the user.
func placeholderURLCredential(u string) bool {
	user, pass, ok := urlCredential(u)
	if !ok {
		return false
	}
	lp := strings.ToLower(pass)
	return placeholderPasswords[lp] || strings.HasPrefix(pass, "${") || strings.HasPrefix(pass, "<") ||
		strings.HasPrefix(pass, "{{") || strings.EqualFold(user, pass) || isPlaceholder(pass)
}

// urlCredential extracts the user and password from a connection URL, in any
// of the three places drivers accept them: userinfo (scheme://user:pass@host),
// query parameters (jdbc:postgresql://host/db?user=…&password=…) and SQL
// Server's ;-separated properties (jdbc:sqlserver://host;user=…;password=…).
func urlCredential(u string) (user, pass string, ok bool) {
	_, rest, found := strings.Cut(u, "://")
	if !found {
		return "", "", false
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?;"); i >= 0 {
		authority = rest[:i]
	}
	// userinfo form: scheme://user:pass@host. ok is whether a ":" split the
	// userinfo, i.e. a password is present (an empty one still counts).
	if userinfo, _, has := strings.Cut(authority, "@"); has {
		user, pass, ok = strings.Cut(userinfo, ":")
		return user, pass, ok
	}
	// query/property form: only a password= parameter sets ok, so a URL with no
	// credentials (a plain webhook) reports none rather than an empty match.
	for _, kv := range strings.FieldsFunc(rest, func(r rune) bool { return r == '?' || r == '&' || r == ';' }) {
		k, v, kvOK := strings.Cut(kv, "=")
		if !kvOK {
			continue
		}
		switch strings.ToLower(k) {
		case "user", "username":
			user = v
		case "password":
			pass, ok = v, true
		}
	}
	return user, pass, ok
}

// firebaseClientContext reports a Google API key that is a Firebase client key:
// inside google-services.json or GoogleService-Info.plist, or in a web
// firebaseConfig object (which always carries authDomain). Firebase documents
// these keys as safe to ship in client code; they are restricted by app, not
// kept secret. A Gemini or Maps server key in an ordinary config is unaffected.
func firebaseClientContext(path string, blob []byte, lo, hi int) bool {
	base := strings.ToLower(path[strings.LastIndexByte(path, '/')+1:])
	if base == "google-services.json" || base == "googleservice-info.plist" {
		return true
	}
	w := blob[max(0, lo-keywordSpan):min(len(blob), hi+keywordSpan)]
	return bytes.Contains(w, []byte("authDomain")) || bytes.Contains(w, []byte(".firebaseapp.com"))
}

// keyClass is what a setting name says about the value assigned to it.
type keyClass int

const (
	keyUnknown keyClass = iota // no key, or one that says nothing either way
	keySecret                  // client_secret, DB_PASSWORD, api_key, hash-secret
	keyPublic                  // client_id, site-key, publishable_key, public_key
)

// assignKey finds the name a value is assigned to, in the text just before it:
// KEY=v, key: v, "key": "v", key = 'v', key := v, key => v, and the default of
// a placeholder, ${RESEND_API_KEY:v}.
var assignKey = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_.\-]*)["'\]]?\s*(?::=|=>|[:=])\s*["'\x60]?$`)

// assignmentKey returns the setting name the value right after before is
// assigned to, or "" when the value is not the right-hand side of anything.
func assignmentKey(before []byte) string {
	if i := bytes.LastIndexByte(before, '\n'); i >= 0 {
		before = before[i+1:] // a key never spans lines
	}
	m := assignKey.FindSubmatch(before)
	if m == nil {
		return ""
	}
	return string(m[1])
}

// keyWords splits a setting name into lower-case words across every naming
// style: snake_case, kebab-case, dotted.paths and camelCase.
func keyWords(key string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	var prev rune
	for _, r := range key {
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z':
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	flush()
	return words
}

// classifyKey decides what a setting name says about its value.
//
// Public markers win over secret words, because the public variant of a
// credential pair is named after the secret one: publishable_key, public_key,
// site_key all contain "key". An id is an identifier — client_id, app_id,
// key_id — unless the name also says secret (client_secret_id is not a thing,
// but app_secret is).
func classifyKey(key string) keyClass {
	if key == "" {
		return keyUnknown
	}
	words := keyWords(key)
	has := func(w string) bool {
		for _, x := range words {
			if x == w {
				return true
			}
		}
		return false
	}
	joined := strings.Join(words, "")
	switch {
	case has("public") || has("publishable") || has("pub"):
		return keyPublic
	case (has("site") && has("key")) || strings.Contains(joined, "sitekey"):
		return keyPublic
	}
	secret := has("secret") || has("password") || has("passwd") || has("pwd") ||
		has("token") || has("credential") || has("credentials") || has("private") ||
		has("apikey") || (has("api") && has("key")) || strings.Contains(joined, "secret") ||
		strings.Contains(joined, "password") || strings.Contains(joined, "apikey")
	if secret {
		return keySecret
	}
	if len(words) > 0 && (words[len(words)-1] == "id" || has("username") || has("user") ||
		has("email") || has("url") || has("uri") || has("host") || has("endpoint") || has("name")) {
		return keyPublic
	}
	return keyUnknown
}

// googleClientID is the shape of an OAuth client id: a project number, a dash
// and a lowercase hash, always followed by .apps.googleusercontent.com.
var googleClientID = regexp.MustCompile(`^[0-9]{6,}-[a-z0-9]{16,}$`)

// publicIdentifier reports a value that is public by design, so finding it in a
// repository is not a leak. after is the text that follows the token.
func publicIdentifier(token string, after []byte) bool {
	switch {
	case strings.HasPrefix(token, "pk_live_") || strings.HasPrefix(token, "pk_test_"):
		return true // Stripe publishable key: shipped to browsers on purpose
	case googleClientID.MatchString(token) && bytes.HasPrefix(after, []byte(".apps.googleusercontent.com")):
		return true // OAuth client id: sent in every authorization URL
	}
	return false
}

// isTestPath reports a test source file or fixture directory. The generic pass
// skips them: an unrecognised high-entropy string in a test is a fixture far
// more often than a leak. Named rules still run, so a real key with its own
// shape pasted into a test is still reported.
func isTestPath(path string) bool {
	p := strings.ToLower(path)
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "test", "tests", "__tests__", "spec", "specs", "fixtures", "__fixtures__", "__mocks__":
			return true
		}
	}
	base := p[strings.LastIndexByte(p, '/')+1:]
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, "test.java") || strings.HasSuffix(base, "tests.java") ||
		strings.HasSuffix(base, "test.kt") || strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")
}

// JWT context is excluded from the generic pass by decision, not because it
// cannot matter. In the repositories scanned so far every hit was a sample
// token in API docs or a test, or an application's own signing secret — the
// mySecretKey… and dev-secret-for-testing defaults that tutorials ship — and
// they outnumbered every other unmapped finding. An issued JWT is short-lived
// and names its issuer in its payload; a signing secret only matters with
// access to the service that uses it. A provider whose API key happens to be a
// JWT (Supabase's service_role key, for one) needs a named rule, which this
// does not affect.

// jwtShape is a compact JWS: base64url header, payload and signature. The
// header of every JWT starts {" which encodes as eyJ.
var jwtShape = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`)

// inJWT reports whether the token at [lo, hi) is one segment of a JWT. The
// token pattern stops at dots, so a JWT arrives as three separate tokens, and
// the signature segment alone looks like any other random string.
func inJWT(blob []byte, lo, hi int) bool {
	start := bytes.LastIndexByte(blob[:lo], '\n') + 1
	end := bytes.IndexByte(blob[hi:], '\n')
	if end < 0 {
		end = len(blob)
	} else {
		end += hi
	}
	for _, m := range jwtShape.FindAllIndex(blob[start:end], -1) {
		if start+m[0] <= lo && hi <= start+m[1] {
			return true
		}
	}
	return false
}

// isJSONSegment reports a token that is base64url-encoded JSON: a JWT header or
// payload quoted on its own, as API docs do with "eyJhbGciOi…" and an ellipsis.
// Encoded JSON is data, not a credential, and a JWT header is identical across
// every token an issuer signs.
func isJSONSegment(token string) bool {
	if !strings.HasPrefix(token, "eyJ") {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(token, "="))
	if err != nil {
		// A truncated sample may not be a whole number of base64 quanta; what
		// does decode still has to read as the start of a JSON object.
		n := len(token) / 4 * 4
		raw, err = base64.RawURLEncoding.DecodeString(token[:n])
		if err != nil {
			return false
		}
	}
	return bytes.HasPrefix(raw, []byte(`{"`)) && utf8.Valid(raw)
}

var (
	// jwt must start a word: JWT_SECRET, jwt.secret, JwtUtil. Mid-word it is
	// just three letters a random base64 string contains often enough to matter.
	jwtWord      = regexp.MustCompile(`(?i)(^|[^a-z0-9])jwt`)
	jwtYAMLBlock = regexp.MustCompile(`(?im)^\s*jwt\s*:\s*$`)
)

// jwtSecretContext reports a value that configures JWT signing: named on its
// own line (JWT_SECRET=, jwt.secret:, @Value("${jwt.secret:…}")), in a file
// that is about JWTs (JwtUtil.java), or nested under a `jwt:` YAML block.
func jwtSecretContext(path string, blob []byte, off int, line string) bool {
	if jwtWord.MatchString(line) {
		return true
	}
	if jwtWord.MatchString(path[strings.LastIndexByte(path, '/')+1:]) {
		return true
	}
	// A YAML key sits a few lines below its parent; four lines up covers the
	// usual jwt: { secret, expiration, issuer } block without reaching into an
	// unrelated one.
	above := blob[:bytes.LastIndexByte(blob[:off], '\n')+1]
	for i := 0; i < 4 && len(above) > 0; i++ {
		prev := bytes.LastIndexByte(above[:len(above)-1], '\n') + 1
		if jwtYAMLBlock.Match(above[prev:]) {
			return true
		}
		above = above[:prev]
	}
	return false
}

// exampleLine matches a line that presents its value as an illustration.
var exampleLine = regexp.MustCompile(`(?i)(\bexamples?\b|\be\.g\.|\bfor instance\b)`)

// exampleFile reports a file that exists to be copied and filled in:
// .env.example, config.sample.json, settings.template.yml, or anything under an
// examples/ or samples/ directory.
func exampleFile(path string) bool {
	p := strings.ToLower(path)
	for _, seg := range strings.Split(p, "/") {
		if seg == "example" || seg == "examples" || seg == "sample" || seg == "samples" {
			return true
		}
	}
	base := p[strings.LastIndexByte(p, '/')+1:]
	for _, marker := range []string{".example", ".sample", ".template", ".tmpl", ".dist"} {
		if strings.HasSuffix(base, marker) || strings.Contains(base, marker+".") {
			return true
		}
	}
	return false
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

// contextWindow bounds how far either side of a generic token its context
// reaches. A line is the right unit in source code, but SQL dumps and minified
// bundles put megabytes on one line, and a keyword anywhere in that much text
// says nothing about the token: a 4 MB INSERT once had its every token judged
// by the word "JWT" in an unrelated article.
const contextWindow = 160

// contextAround returns the token's line, clipped to contextWindow bytes on
// each side of [lo, hi).
func contextAround(blob []byte, lo, hi int) string {
	start := max(bytes.LastIndexByte(blob[:lo], '\n')+1, lo-contextWindow)
	end := len(blob)
	if i := bytes.IndexByte(blob[hi:], '\n'); i >= 0 {
		end = hi + i
	}
	return string(blob[start:min(end, hi+contextWindow)])
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
