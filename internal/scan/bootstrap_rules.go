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
		// The whole URL is the credential: anyone holding it can post to the
		// channel. The generic pass skips URL paths, so this needs a rule.
		id: "slack-webhook-url", provider: "slack", severity: "medium",
		pattern:  `(?P<secret>https://hooks\.slack\.com/services/T[0-9A-Z]+/B[0-9A-Z]+/[0-9A-Za-z]{24})`,
		literals: []string{"hooks.slack.com"},
	},
	{
		id: "google-api-key", provider: "google", severity: "high",
		pattern:  `(?P<secret>AIza[0-9A-Za-z_-]{35})`,
		literals: []string{"AIza"},
	},
	{
		// The refresh token from Google's OAuth flow: long-lived, and exchangeable
		// for access tokens until the user revokes it. 1// is its fixed prefix.
		id: "google-oauth-refresh-token", provider: "google", severity: "high",
		pattern:  `(?P<secret>1//0[0-9A-Za-z_-]{30,})`,
		literals: []string{"1//0"},
	},
	{
		// re_, an 8-character id, an underscore and a 24-character secret. The
		// fixed layout is what keeps a literal as short as re_ from matching the
		// re_compile and re_match of ordinary code.
		id: "resend-api-key", provider: "resend", severity: "high",
		pattern:  `\b(?P<secret>re_[A-Za-z0-9]{8}_[A-Za-z0-9]{24})\b`,
		literals: []string{"re_"},
	},
	{
		id: "sendgrid-api-key", provider: "sendgrid", severity: "high",
		pattern:  `(?P<secret>SG\.[0-9A-Za-z_-]{22}\.[0-9A-Za-z_-]{43})`,
		literals: []string{"SG."},
	},

	{
		// A Giphy key is 32 alphanumerics with no prefix, so the setting name is
		// the signal. Front-end code rarely assigns it directly: it is usually
		// the fallback in `import.meta.env.VITE_GIPHY_API_KEY || '…'`, so the
		// value may sit some way past the = rather than right after it.
		id: "giphy-api-key", provider: "giphy", severity: "medium",
		pattern:  `(?i)giphy[\w.-]{0,30}\s*[:=][^\n]{0,80}?\b(?P<secret>[A-Za-z0-9]{32})\b`,
		literals: []string{"giphy", "GIPHY", "Giphy"},
	},
	{
		// The same key passed straight to the API in a request URL.
		id: "giphy-api-key-url", provider: "giphy", severity: "medium",
		pattern:  `api\.giphy\.com/[^\s"'\x60]*[?&]api_key=(?P<secret>[A-Za-z0-9]{32})\b`,
		literals: []string{"api.giphy.com"},
	},

	// AI providers. A leaked key here is billed per token, so it is spent within
	// hours of reaching a public repository; most issue a recognisable prefix
	// and need no context. Gemini keys are AIza… and already match
	// google-api-key above.
	{
		// Every OpenAI key format — legacy sk-, sk-proj-, sk-svcacct-, sk-admin-
		// — embeds T3BlbkFJ ("OpenAI" in base64) mid-key. It is a far more
		// selective literal than "sk-", which half the ecosystem also uses.
		id: "openai-api-key", provider: "openai", severity: "high",
		pattern:  `(?P<secret>sk-(?:proj-|svcacct-|admin-|None-)?[A-Za-z0-9_-]{16,}T3BlbkFJ[A-Za-z0-9_-]{16,})`,
		literals: []string{"T3BlbkFJ"},
	},
	{
		id: "anthropic-api-key", provider: "anthropic", severity: "high",
		pattern:  `(?P<secret>sk-ant-(?:api|admin)\d{2}-[A-Za-z0-9_-]{80,})`,
		literals: []string{"sk-ant-"},
	},
	{
		// One OpenRouter key routes to every model it fronts, OpenAI and
		// Anthropic included, on the owner's credit.
		id: "openrouter-api-key", provider: "openrouter", severity: "high",
		pattern:  `(?P<secret>sk-or-v1-[0-9a-f]{64})`,
		literals: []string{"sk-or-v1-"},
	},
	{
		id: "groq-api-key", provider: "groq", severity: "high",
		pattern:  `(?P<secret>gsk_[A-Za-z0-9]{52})`,
		literals: []string{"gsk_"},
	},
	{
		// hf_ is short enough to occur inside identifiers (pathf_, graphf_), so
		// the word boundary is what keeps it from matching mid-word.
		id: "huggingface-token", provider: "huggingface", severity: "high",
		pattern:  `\b(?P<secret>hf_[A-Za-z0-9]{34})\b`,
		literals: []string{"hf_"},
	},
	{
		id: "replicate-api-token", provider: "replicate", severity: "high",
		pattern:  `\b(?P<secret>r8_[A-Za-z0-9]{37})\b`,
		literals: []string{"r8_"},
	},
	{
		id: "perplexity-api-key", provider: "perplexity", severity: "high",
		pattern:  `(?P<secret>pplx-[A-Za-z0-9]{48})`,
		literals: []string{"pplx-"},
	},
	{
		id: "xai-api-key", provider: "xai", severity: "high",
		pattern:  `\b(?P<secret>xai-[A-Za-z0-9]{80})\b`,
		literals: []string{"xai-"},
	},
	{
		id: "langsmith-api-key", provider: "langsmith", severity: "medium",
		pattern:  `(?P<secret>lsv2_(?:pt|sk)_[0-9a-f]{32}_[0-9a-f]{10})`,
		literals: []string{"lsv2_"},
	},
	{
		id: "pinecone-api-key", provider: "pinecone", severity: "high",
		pattern:  `\b(?P<secret>pcsk_[A-Za-z0-9]{5,}_[A-Za-z0-9]{40,})`,
		literals: []string{"pcsk_"},
	},
	{
		// DeepSeek keys are sk- plus 32 hex, a shape other services share, so
		// the setting name is the signal.
		id: "deepseek-api-key", provider: "deepseek", severity: "high",
		pattern:  `(?i)deepseek[\w.-]{0,20}\s*[:=]\s*["']?(?P<secret>sk-[0-9a-f]{32})\b`,
		literals: []string{"deepseek", "DEEPSEEK", "DeepSeek", "Deepseek"},
	},
	{
		// Cohere, Mistral and Azure OpenAI issue bare alphanumeric or hex keys
		// with no prefix, so — as with the Cloudinary API key — the setting
		// name is the whole signal.
		id: "cohere-api-key", provider: "cohere", severity: "high",
		pattern:  `(?i)(?:cohere|co)[_.-]?api[_.-]?key\s*[:=]\s*["']?(?P<secret>[A-Za-z0-9]{40})\b`,
		literals: []string{"cohere", "COHERE", "Cohere", "CO_API_KEY", "co_api_key"},
	},
	{
		id: "mistral-api-key", provider: "mistral", severity: "high",
		pattern:  `(?i)mistral[\w.-]{0,20}\s*[:=]\s*["']?(?P<secret>[A-Za-z0-9]{32})\b`,
		literals: []string{"mistral", "MISTRAL", "Mistral"},
	},
	{
		// Classic Azure keys are 32 hex; the newer format is 84 alphanumerics.
		id: "azure-openai-api-key", provider: "azure-openai", severity: "high",
		pattern:  `(?i)azure[_.-]?openai[\w.-]{0,20}\s*[:=]\s*["']?(?P<secret>[0-9a-f]{32}|[A-Za-z0-9]{84})\b`,
		literals: []string{"azure", "AZURE", "Azure"},
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
		// The secret spans the whole block, not just the header. A header-only
		// match gave every key the same fingerprint, so a second key in the same
		// repository was dropped as a duplicate; and the key body, which a
		// service-account JSON keeps on one line beside "private_key", was left
		// for the entropy pass to report again one 64-character row at a time.
		// The END marker is optional so a truncated key is still reported.
		id: "private-key-block", provider: "generic", severity: "critical",
		pattern:  `(?P<secret>-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP |ENCRYPTED )?PRIVATE KEY-----(?:[\s\S]*?-----END (?:RSA |EC |OPENSSH |DSA |PGP |ENCRYPTED )?PRIVATE KEY-----)?)`,
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
