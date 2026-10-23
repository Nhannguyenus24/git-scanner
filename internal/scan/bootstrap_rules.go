package scan

// bootstrapRuleDefs is the built-in starter catalogue (plan.md Phase 1). It
// exists so the engine detects well-known credentials before the registry is
// populated in Phase 3; at that point these same shapes are imported as the
// first registry definitions and this file stops being the source of truth.
//
// Each pattern carries one (?P<secret>...) group marking the credential span.
// Literals must be substrings the pattern always contains, so the prefilter can
// gate the regex.
var bootstrapRuleDefs = []struct {
	id       string
	provider string
	severity string
	pattern  string
	literals []string
	keywords []string
}{
	{
		id: "aws-access-key-id", provider: "aws", severity: "high",
		pattern:  `(?P<secret>(?:AKIA|ASIA)[0-9A-Z]{16})`,
		literals: []string{"AKIA", "ASIA"},
	},
	{
		id: "github-pat", provider: "github", severity: "high",
		pattern:  `(?P<secret>gh[pous]_[0-9A-Za-z]{36})`,
		literals: []string{"ghp_", "gho_", "ghu_", "ghs_"},
	},
	{
		id: "github-fine-grained-pat", provider: "github", severity: "high",
		pattern:  `(?P<secret>github_pat_[0-9A-Za-z_]{82})`,
		literals: []string{"github_pat_"},
	},
	{
		id: "stripe-secret-key", provider: "stripe", severity: "critical",
		pattern:  `(?P<secret>sk_live_[0-9A-Za-z]{24,})`,
		literals: []string{"sk_live_"},
	},
	{
		// A test key moves no money, so it is reported at low severity rather
		// than not at all: it still identifies the account and tends to sit in
		// the same file as the live one.
		id: "stripe-test-key", provider: "stripe", severity: "low",
		pattern:  `(?P<secret>sk_test_[0-9A-Za-z]{24,})`,
		literals: []string{"sk_test_"},
	},
	{
		id: "google-oauth-client-secret", provider: "google", severity: "high",
		pattern:  `(?P<secret>GOCSPX-[0-9A-Za-z_-]{28})`,
		literals: []string{"GOCSPX-"},
	},
	{
		id: "slack-bot-token", provider: "slack", severity: "high",
		pattern:  `(?P<secret>xox[baprs]-[0-9A-Za-z-]{10,})`,
		literals: []string{"xoxb-", "xoxa-", "xoxp-", "xoxr-", "xoxs-"},
	},
	{
		id: "google-api-key", provider: "google", severity: "high",
		pattern:  `(?P<secret>AIza[0-9A-Za-z_-]{35})`,
		literals: []string{"AIza"},
	},
	{
		id: "sendgrid-api-key", provider: "sendgrid", severity: "high",
		pattern:  `(?P<secret>SG\.[0-9A-Za-z_-]{22}\.[0-9A-Za-z_-]{43})`,
		literals: []string{"SG."},
	},
	{
		// Cloudinary packs cloud name, key and secret into one URL.
		id: "cloudinary-url", provider: "cloudinary", severity: "high",
		pattern:  `(?P<secret>cloudinary://[0-9]{15}:[0-9A-Za-z_-]{20,}@[0-9A-Za-z_-]+)`,
		literals: []string{"cloudinary://"},
	},
	{
		// The same credentials are just as often carried as three separate
		// settings — Spring's cloudinary.api-secret, or a CLOUDINARY_API_SECRET
		// environment variable — which the URL rule above cannot see.
		//
		// The 27-character value has no distinctive prefix, so the pattern
		// anchors on the setting name instead. That context is what separates it
		// from any other base64 token of the same length, and it is why this
		// rule can be specific without the value itself being recognisable.
		id: "cloudinary-api-secret", provider: "cloudinary", severity: "critical",
		pattern: `(?i)cloudinary[._-]?api[._-]?secret\s*[:=]\s*["']?(?P<secret>[0-9A-Za-z_-]{27})`,
		// The prefilter is case-sensitive, so both spellings must be listed or
		// the uppercase environment-variable form never reaches the regex.
		literals: []string{"cloudinary", "CLOUDINARY", "Cloudinary"},
	},
	{
		// The API key is 15 digits and nothing else. Matched on its own it would
		// fire on every timestamp and identifier in a codebase, so the setting
		// name is not an optimisation here — it is the whole signal.
		id: "cloudinary-api-key", provider: "cloudinary", severity: "high",
		pattern:  `(?i)cloudinary[._-]?api[._-]?key\s*[:=]\s*["']?(?P<secret>[0-9]{15})`,
		literals: []string{"cloudinary", "CLOUDINARY", "Cloudinary"},
	},
	{
		// SMTP credentials embedded in a URL (smtp://user:pass@host). The host
		// reveals the provider; the local-config filter drops localhost ones.
		id: "smtp-url", provider: "smtp", severity: "medium",
		pattern:  `(?P<secret>smtps?://[^:@\s/]+:[^@\s/]+@[^\s/"']+)`,
		literals: []string{"smtp://", "smtps://"},
	},
	{
		id: "private-key-block", provider: "generic", severity: "critical",
		pattern:  `(?P<secret>-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----)`,
		literals: []string{"PRIVATE KEY-----"},
	},
}

// BootstrapRuleset compiles the built-in rules at registry version 0.
// It panics on a bad built-in pattern, which is a programming error caught by
// the package's tests, never a runtime condition.
func BootstrapRuleset() *Ruleset {
	rules := make([]*Rule, 0, len(bootstrapRuleDefs))
	for _, d := range bootstrapRuleDefs {
		r := &Rule{
			ID: d.id, Provider: d.provider, Severity: d.severity,
			Literals: d.literals, Keywords: d.keywords,
		}
		if err := r.compile(d.pattern); err != nil {
			panic(err)
		}
		rules = append(rules, r)
	}
	return NewRuleset(0, rules)
}
