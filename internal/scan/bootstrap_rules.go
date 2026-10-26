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
		// The same secret inside an SDK config object — cloudinary.config({
		// api_secret: '...' }) or api_secret=os.getenv("CLOUDINARY_API_SECRET",
		// "fallback") — where the setting is just api_secret and "cloudinary"
		// sits a few lines away. The keyword is checked in a window around the
		// match, not on the line, which is what makes this precise.
		id: "cloudinary-api-secret-config", provider: "cloudinary", severity: "critical",
		pattern:  `(?i)\bapi[_-]?secret\b["']?\s*[:=]\s*[^\n]{0,80}?["'\x60](?P<secret>[0-9A-Za-z_-]{27})["'\x60]`,
		literals: []string{"api_secret", "api-secret", "apiSecret", "API_SECRET", "ApiSecret"},
		keywords: []string{"cloudinary"},
	},
	{
		// reCAPTCHA secret keys are 6L plus 38 characters — the same shape as
		// the public site key, so only a setting that says secret, near the word
		// recaptcha, separates them.
		id: "recaptcha-secret-key", provider: "google", severity: "medium",
		pattern:  `(?i)secret[\w.-]{0,20}["']?\s*[:=]\s*["']?(?P<secret>6L[0-9A-Za-z_-]{38})\b`,
		literals: []string{"secret", "SECRET", "Secret"},
		keywords: []string{"recaptcha"},
	},

	// Hosting, SaaS and developer platforms. Each of these famous providers
	// issues a token with a fixed, self-identifying prefix, so — as with AKIA…
	// and ghp_… above — the prefix alone is signal enough and no setting-name
	// context is needed. The literal list must spell every prefix exactly: the
	// prefilter does not fold case.
	{
		id: "gitlab-pat", provider: "gitlab", severity: "high",
		pattern:  `(?P<secret>glpat-[0-9A-Za-z_-]{20})`,
		literals: []string{"glpat-"},
	},
	{
		id: "npm-access-token", provider: "npm", severity: "high",
		pattern:  `(?P<secret>npm_[0-9A-Za-z]{36})`,
		literals: []string{"npm_"},
	},
	{
		// dop_ personal, doo_ OAuth, dor_ refresh, dot_ temporary — each 64 hex.
		id: "digitalocean-token", provider: "digitalocean", severity: "high",
		pattern:  `(?P<secret>do[oprt]_v1_[0-9a-f]{64})`,
		literals: []string{"dop_v1_", "doo_v1_", "dor_v1_", "dot_v1_"},
	},
	{
		// shpat_ app token, shpca_ custom-app, shppa_ private-app, shpss_ shared
		// secret — all grant Admin-API access to the store.
		id: "shopify-token", provider: "shopify", severity: "high",
		pattern:  `(?P<secret>shp(?:at|ca|pa|ss)_[0-9a-fA-F]{32})`,
		literals: []string{"shpat_", "shpca_", "shppa_", "shpss_"},
	},
	{
		id: "square-access-token", provider: "square", severity: "high",
		pattern:  `(?P<secret>sq0atp-[0-9A-Za-z_-]{22})`,
		literals: []string{"sq0atp-"},
	},
	{
		// The OAuth application secret: full account control, so critical.
		id: "square-oauth-secret", provider: "square", severity: "critical",
		pattern:  `(?P<secret>sq0csp-[0-9A-Za-z_-]{43})`,
		literals: []string{"sq0csp-"},
	},
	{
		// 32 hex, a hyphen, then the datacentre suffix usN. The usN suffix is the
		// fixed part that separates it from any other 32-hex digest.
		id: "mailchimp-api-key", provider: "mailchimp", severity: "high",
		pattern:  `\b(?P<secret>[0-9a-f]{32}-us[0-9]{1,2})\b`,
		literals: []string{"-us"},
	},
	{
		// Mailgun's private key: the key- prefix plus 32 hex. The prefix is
		// common, so the fixed-length hex body is what carries the match.
		id: "mailgun-api-key", provider: "mailgun", severity: "medium",
		pattern:  `\b(?P<secret>key-[0-9a-f]{32})\b`,
		literals: []string{"key-"},
	},
	{
		// sntrys_ is an organisation auth token, sntryu_ a user token.
		id: "sentry-auth-token", provider: "sentry", severity: "high",
		pattern:  `(?P<secret>sntr(?:ys|yu)_[A-Za-z0-9_=+/-]{40,})`,
		literals: []string{"sntrys_", "sntryu_"},
	},
	{
		id: "linear-api-key", provider: "linear", severity: "high",
		pattern:  `(?P<secret>lin_api_[0-9A-Za-z]{40})`,
		literals: []string{"lin_api_"},
	},
	{
		// Notion's current integration-token prefix. The older secret_ form is
		// too generic to enable without context and is left to the entropy pass.
		id: "notion-integration-token", provider: "notion", severity: "high",
		pattern:  `(?P<secret>ntn_[0-9A-Za-z]{40,})`,
		literals: []string{"ntn_"},
	},
	{
		id: "figma-token", provider: "figma", severity: "high",
		pattern:  `(?P<secret>figd_[0-9A-Za-z_-]{32,})`,
		literals: []string{"figd_"},
	},
	{
		// dp.pt. personal, dp.st. service, dp.ct. CLI, dp.sa. service-account.
		id: "doppler-token", provider: "doppler", severity: "high",
		pattern:  `(?P<secret>dp\.(?:pt|st|ct|sa|scim|audit)\.[A-Za-z0-9]{40,44})`,
		literals: []string{"dp.pt.", "dp.st.", "dp.ct.", "dp.sa.", "dp.scim.", "dp.audit."},
	},
	{
		id: "supabase-access-token", provider: "supabase", severity: "high",
		pattern:  `(?P<secret>sbp_[0-9a-f]{40})`,
		literals: []string{"sbp_"},
	},
	{
		// pscale_pw_ is a database password, pscale_tkn_ a service token.
		id: "planetscale-token", provider: "planetscale", severity: "high",
		pattern:  `(?P<secret>pscale_(?:pw|tkn)_[0-9A-Za-z._-]{32,})`,
		literals: []string{"pscale_pw_", "pscale_tkn_"},
	},
	{
		// dapi plus 32 hex, optionally suffixed with -<n> for a token id.
		id: "databricks-token", provider: "databricks", severity: "high",
		pattern:  `\b(?P<secret>dapi[0-9a-f]{32}(?:-[0-9]+)?)\b`,
		literals: []string{"dapi"},
	},
	{
		// NRAK- is a user API key: 27 base62 chars follow. The 40-hex licence key
		// has no prefix of its own and is left to the entropy pass.
		id: "newrelic-user-key", provider: "newrelic", severity: "high",
		pattern:  `(?P<secret>NRAK-[A-Z0-9]{27})`,
		literals: []string{"NRAK-"},
	},
	{
		// glsa_ service-account token, glc_ cloud access-policy token. The gl
		// start is shared with GitLab's glpat-, but the literals are distinct so
		// the prefilter never confuses them.
		id: "grafana-token", provider: "grafana", severity: "high",
		pattern:  `(?P<secret>gl(?:sa|c)_[A-Za-z0-9_=+/-]{32,})`,
		literals: []string{"glsa_", "glc_"},
	},
	{
		id: "tailscale-key", provider: "tailscale", severity: "high",
		pattern:  `(?P<secret>tskey-(?:auth|api|client)-[0-9A-Za-z]+-[0-9A-Za-z]+)`,
		literals: []string{"tskey-"},
	},
	{
		id: "flyio-token", provider: "flyio", severity: "high",
		pattern:  `(?P<secret>fo1_[0-9A-Za-z_-]{43})`,
		literals: []string{"fo1_"},
	},
	{
		// The whole webhook URL is the credential: anyone holding it can post to
		// the channel. Covers the discord.com and legacy discordapp.com hosts and
		// the canary/ptb subdomains. The generic pass skips URL paths.
		id: "discord-webhook-url", provider: "discord", severity: "medium",
		pattern:  `(?P<secret>https://(?:canary\.|ptb\.)?discord(?:app)?\.com/api/(?:v[0-9]+/)?webhooks/[0-9]+/[0-9A-Za-z_-]+)`,
		literals: []string{"discord.com/api/webhooks", "discordapp.com/api/webhooks"},
	},
	{
		// The bot token carried in the Bot API URL — digits, a colon, then the
		// 35-character auth part. The api.telegram.org/bot path is the signal, so
		// no keyword is needed.
		id: "telegram-bot-token", provider: "telegram", severity: "medium",
		pattern:  `api\.telegram\.org/bot(?P<secret>[0-9]{8,10}:[A-Za-z0-9_-]{35})`,
		literals: []string{"api.telegram.org/bot"},
	},
	{
		// Twilio's auth token and API-key secret are bare hex with no prefix of
		// their own, so — as with the Cohere and Mistral keys above — the word
		// twilio on the setting is the whole signal. Without that gate a 32-hex
		// value would be attributed to Twilio on sight.
		id: "twilio-auth-token", provider: "twilio", severity: "high",
		pattern:  `(?i)twilio[\w.-]{0,24}(?:token|secret|auth)[\w.-]{0,10}\s*[:=]\s*["']?(?P<secret>[0-9a-f]{32})\b`,
		literals: []string{"twilio", "TWILIO", "Twilio"},
	},

	// A second batch of well-known platforms, same reasoning as the block above:
	// a fixed, self-identifying prefix is signal enough, so the prefix is the
	// literal and the pattern needs no context — except Datadog and Cloudflare,
	// whose keys are bare hex/base62 and so are gated on the provider name.
	{
		// Atlassian (Jira, Confluence, Bitbucket) API tokens begin with this
		// twelve-character marker, which is specific enough to stand alone.
		id: "atlassian-api-token", provider: "atlassian", severity: "high",
		pattern:  `(?P<secret>ATATT3xFfGF0[A-Za-z0-9+/=_-]{50,})`,
		literals: []string{"ATATT3xFfGF0"},
	},
	{
		id: "dockerhub-pat", provider: "dockerhub", severity: "high",
		pattern:  `(?P<secret>dckr_pat_[A-Za-z0-9_-]{27,})`,
		literals: []string{"dckr_pat_"},
	},
	{
		// hvs. service token, hvb. batch token, hvr. recovery token.
		id: "vault-token", provider: "vault", severity: "high",
		pattern:  `(?P<secret>hv[sbr]\.[A-Za-z0-9_-]{24,})`,
		literals: []string{"hvs.", "hvb.", "hvr."},
	},
	{
		// Terraform Cloud / Enterprise API token: a 14-char id, the fixed
		// .atlasv1. segment, then the secret.
		id: "terraform-cloud-token", provider: "terraform", severity: "high",
		pattern:  `(?P<secret>[A-Za-z0-9]{14}\.atlasv1\.[A-Za-z0-9_-]{60,})`,
		literals: []string{".atlasv1."},
	},
	{
		// A PyPI upload token is a macaroon; its base64 head always decodes from
		// "pypi.org", giving this long, highly specific literal.
		id: "pypi-upload-token", provider: "pypi", severity: "high",
		pattern:  `(?P<secret>pypi-AgEIcHlwaS5vcmc[A-Za-z0-9_-]{50,})`,
		literals: []string{"pypi-AgEIcHlwaS5vcmc"},
	},
	{
		id: "rubygems-api-key", provider: "rubygems", severity: "high",
		pattern:  `\b(?P<secret>rubygems_[0-9a-f]{48})\b`,
		literals: []string{"rubygems_"},
	},
	{
		id: "postman-api-key", provider: "postman", severity: "high",
		pattern:  `(?P<secret>PMAK-[0-9a-f]{24}-[0-9a-f]{34})`,
		literals: []string{"PMAK-"},
	},
	{
		// dt0c01. is the API-token prefix: a 24-char public id, then the secret.
		id: "dynatrace-token", provider: "dynatrace", severity: "high",
		pattern:  `(?P<secret>dt0c01\.[A-Z0-9]{24}\.[A-Z0-9]{64})`,
		literals: []string{"dt0c01."},
	},
	{
		// Datadog API/app keys are bare 32–40 hex with no prefix, so the setting
		// name carries the attribution — the Cohere/Twilio pattern again.
		id: "datadog-api-key", provider: "datadog", severity: "high",
		pattern:  `(?i)(?:datadog|dd)[_.-]?(?:api|app)[_.-]?key\s*[:=]\s*["']?(?P<secret>[0-9a-f]{32,40})\b`,
		literals: []string{"datadog", "DATADOG", "Datadog", "DD_API", "DD_APP", "dd_api", "dd_app"},
	},
	{
		// Cloudflare API tokens and the 37-hex global key have no prefix of their
		// own, so the word cloudflare on the setting is the whole signal.
		id: "cloudflare-api-token", provider: "cloudflare", severity: "high",
		pattern:  `(?i)cloudflare[\w.-]{0,24}(?:api[_.-]?(?:token|key)|global[_.-]?key)\s*[:=]\s*["']?(?P<secret>[A-Za-z0-9_-]{37,40})\b`,
		literals: []string{"cloudflare", "CLOUDFLARE", "Cloudflare"},
	},
	{
		id: "contentful-token", provider: "contentful", severity: "high",
		pattern:  `(?P<secret>CFPAT-[A-Za-z0-9_-]{43})`,
		literals: []string{"CFPAT-"},
	},
	{
		id: "pulumi-access-token", provider: "pulumi", severity: "high",
		pattern:  `\b(?P<secret>pul-[0-9a-f]{40})\b`,
		literals: []string{"pul-"},
	},
	{
		// Brevo (formerly Sendinblue): xkeysib- prefix, 64 hex, then a 16-char id.
		id: "brevo-api-key", provider: "brevo", severity: "high",
		pattern:  `(?P<secret>xkeysib-[0-9a-f]{64}-[A-Za-z0-9]{16})`,
		literals: []string{"xkeysib-"},
	},
	{
		id: "razorpay-live-key", provider: "razorpay", severity: "high",
		pattern:  `(?P<secret>rzp_live_[0-9A-Za-z]{14,})`,
		literals: []string{"rzp_live_"},
	},
	{
		// A test key moves no money, so low — same treatment as Stripe's.
		id: "razorpay-test-key", provider: "razorpay", severity: "low",
		pattern:  `(?P<secret>rzp_test_[0-9A-Za-z]{14,})`,
		literals: []string{"rzp_test_"},
	},
	{
		// Flutterwave secret key: FLWSECK- for live, FLWSECK_TEST- for test.
		id: "flutterwave-secret-key", provider: "flutterwave", severity: "high",
		pattern:  `(?P<secret>FLWSECK_TEST-[0-9a-f]{32,}|FLWSECK-[0-9a-f]{32}-X)`,
		literals: []string{"FLWSECK-", "FLWSECK_TEST-"},
	},
	{
		// Alibaba Cloud AccessKey ID: like AWS's AKIA, a public id worth flagging
		// because it travels beside its secret.
		id: "alibaba-access-key-id", provider: "alibaba", severity: "high",
		pattern:  `\b(?P<secret>LTAI[0-9A-Za-z]{12,20})\b`,
		literals: []string{"LTAI"},
	},
	{
		id: "tencent-secret-id", provider: "tencent", severity: "high",
		pattern:  `\b(?P<secret>AKID[0-9A-Za-z]{13,40})\b`,
		literals: []string{"AKID"},
	},
	{
		// Mapbox secret token: sk. plus a JWT. The generic pass skips JWTs, so a
		// named rule is the only way this is reported.
		id: "mapbox-secret-token", provider: "mapbox", severity: "medium",
		pattern:  `(?P<secret>sk\.eyJ[A-Za-z0-9_-]{50,}\.[A-Za-z0-9_-]{20,})`,
		literals: []string{"sk.eyJ"},
	},
	{
		id: "prefect-api-key", provider: "prefect", severity: "medium",
		pattern:  `\b(?P<secret>pnu_[A-Za-z0-9]{36})\b`,
		literals: []string{"pnu_"},
	},

	// Database, cache and broker connection URLs with a password in the
	// userinfo: scheme://user:pass@host[,host2][/db][?opts]. The secret is the
	// whole URL, not just the password, because the password alone unlocks
	// nothing without the host — and the prober needs both to check liveness.
	// One rule per engine so each is attributed to its own provider.
	//
	// The user may be empty (redis://:pass@host). The password excludes / so
	// a URL with no userinfo (postgres://host:5432/db?x=a@b) cannot read its
	// port and path as one. Localhost URLs are dropped by the local-config
	// filter (hostScopedProviders), private-range hosts are kept: those are
	// real shared infrastructure. The jdbc:mysql:// and jdbc:postgresql://
	// forms match too, since the scheme literal is a substring of them.
	{
		id: "postgres-url", provider: "postgres", severity: "high",
		pattern:  `(?P<secret>postgres(?:ql)?://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"postgres://", "postgresql://"},
	},
	{
		id: "mysql-url", provider: "mysql", severity: "high",
		pattern:  `(?P<secret>(?:mysql|mariadb)://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"mysql://", "mariadb://"},
	},
	{
		id: "mongodb-url", provider: "mongodb", severity: "high",
		pattern:  `(?P<secret>mongodb(?:\+srv)?://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"mongodb://", "mongodb+srv://"},
	},
	{
		id: "redis-url", provider: "redis", severity: "high",
		pattern:  `(?P<secret>rediss?://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"redis://", "rediss://"},
	},
	{
		id: "rabbitmq-url", provider: "rabbitmq", severity: "high",
		pattern:  `(?P<secret>amqps?://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"amqp://", "amqps://"},
	},
	{
		id: "mssql-url", provider: "mssql", severity: "high",
		pattern:  `(?P<secret>(?:sqlserver|mssql)://[^:@\s/"'\x60]*:[^@\s/"'\x60]+@[^\s/"'\x60<>()]+(?:/[^\s"'\x60<>()]*)?)`,
		literals: []string{"sqlserver://", "mssql://"},
	},
	// JDBC drivers also take the credentials as parameters rather than
	// userinfo — jdbc:postgresql://host/db?user=app&password=…, and SQL
	// Server's jdbc:sqlserver://host:1433;databaseName=…;password=… — which
	// the userinfo rules above cannot see. The authority must not contain @,
	// so a URL carrying both forms is reported once, by the userinfo rule.
	{
		id: "postgres-jdbc-url", provider: "postgres", severity: "high",
		pattern:  `(?P<secret>jdbc:postgresql://[^\s/@?"'\x60<>()]+(?:/[^\s?"'\x60<>()]*)?\?(?:[^\s&"'\x60<>()]*&)*password=[^\s&"'\x60<>()]+(?:&[^\s"'\x60<>()]*)?)`,
		literals: []string{"jdbc:postgresql://"},
	},
	{
		id: "mysql-jdbc-url", provider: "mysql", severity: "high",
		pattern:  `(?P<secret>jdbc:(?:mysql|mariadb)://[^\s/@?"'\x60<>()]+(?:/[^\s?"'\x60<>()]*)?\?(?:[^\s&"'\x60<>()]*&)*password=[^\s&"'\x60<>()]+(?:&[^\s"'\x60<>()]*)?)`,
		literals: []string{"jdbc:mysql://", "jdbc:mariadb://"},
	},
	{
		id: "mssql-jdbc-url", provider: "mssql", severity: "high",
		pattern:  `(?P<secret>jdbc:sqlserver://[^\s/@;"'\x60<>()]*(?:;[^\s;"'\x60<>()]*)*?;(?i:password)=[^\s;"'\x60<>()]+(?:;[^\s"'\x60<>()]*)?)`,
		literals: []string{"jdbc:sqlserver://"},
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
