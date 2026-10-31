package scan_test

import (
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/scan"
)

func detect(path, blob string) []scan.Match {
	return scan.NewDetector(scan.BootstrapRuleset()).Detect(path, []byte(blob))
}

var (
	dummyStripeKey         = "sk_live" + "_0123456789abcdefghijklmn"
	dummyStripePlaceholder = "sk_live" + "_your_key_here_placeholder00"
)

func TestDetectFindsKnownProviders(t *testing.T) {
	cases := []struct{ name, blob, rule string }{
		{"aws", "AWS_ACCESS_KEY_ID=AKIAQYLPMN5HHHFPZAM2", "aws-access-key-id"},
		{"github pat", "token: ghp_0123456789abcdefghijklmnopqrstuvwxyz", "github-pat"},
		{"stripe", "STRIPE_KEY=" + dummyStripeKey, "stripe-secret-key"},
		{"cloudinary", "url=cloudinary://123456789012345:abcdefghijklmnopqrstu@demo", "cloudinary-url"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\n" + strings.Repeat("MIIBVgIBADANBgkqhki", 4) + "\n-----END RSA PRIVATE KEY-----", "private-key-block"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) == 0 {
			t.Errorf("%s: no match", c.name)
			continue
		}
		if ms[0].RuleID != c.rule {
			t.Errorf("%s: rule = %s, want %s", c.name, ms[0].RuleID, c.rule)
		}
	}
}

func TestDetectExtractsContext(t *testing.T) {
	ms := detect(".env", "STRIPE_SECRET_KEY="+dummyStripeKey)
	if len(ms) != 1 {
		t.Fatalf("matches = %d, want 1", len(ms))
	}
	if ms[0].NearbyEnvVar != "STRIPE_SECRET_KEY" {
		t.Errorf("env var = %q", ms[0].NearbyEnvVar)
	}
	if ms[0].Line != 1 {
		t.Errorf("line = %d, want 1", ms[0].Line)
	}
}

func TestDetectSkipsPlaceholdersAndVendored(t *testing.T) {
	if ms := detect(".env", "key="+dummyStripePlaceholder); len(ms) != 0 {
		t.Errorf("placeholder was reported: %+v", ms)
	}
	if ms := detect("vendor/x/f.go", "AKIAQYLPMN5HHHFPZAM2"); len(ms) != 0 {
		t.Errorf("vendored path was scanned: %+v", ms)
	}
	if ms := detect("a.go", "\x00AKIAQYLPMN5HHHFPZAM2"); len(ms) != 0 {
		t.Errorf("binary blob was scanned: %+v", ms)
	}
}

func TestDetectReportsLineNumber(t *testing.T) {
	blob := strings.Repeat("harmless line\n", 5) + "KEY=ghp_0123456789abcdefghijklmnopqrstuvwxyz"
	ms := detect("a.txt", blob)
	if len(ms) != 1 || ms[0].Line != 6 {
		t.Fatalf("line = %v, want 6", ms)
	}
}

func TestGenericHighEntropy(t *testing.T) {
	// A long opaque value next to a secret-ish keyword, matched by no rule.
	ms := detect(".env", "APP_SECRET=Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa")
	if len(ms) != 1 || ms[0].RuleID != "generic-high-entropy" {
		t.Fatalf("matches = %+v, want one generic-high-entropy", ms)
	}
	if ms[0].Provider != "" {
		t.Errorf("generic match should be unmapped, got provider %q", ms[0].Provider)
	}

	// No nearby keyword: a bare high-entropy token (e.g. a commit hash) is not
	// reported, which is what keeps the false-positive rate down.
	if ms := detect("a.txt", "build id Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa"); len(ms) != 0 {
		t.Errorf("token with no keyword was reported: %+v", ms)
	}

	// Low-entropy value: not a secret even with a keyword.
	if ms := detect(".env", "SECRET=aaaaaaaaaaaaaaaaaaaaaaaa"); len(ms) != 0 {
		t.Errorf("low-entropy value reported: %+v", ms)
	}
}

// The generic pass used to report the words of source code that sit beside
// "token" and "password": method names, paths, variable references. Each case
// here was a real false positive.
func TestGenericHighEntropySkipsWordShapedTokens(t *testing.T) {
	cases := []struct{ name, path, blob string }{
		{"camelCase method", "AuthService.java", "export const getChangePasswordRequestSchema = (t) => token(t);"},
		{"package path", "README.md", "see /backend/src/main/java/com/app/UserController for auth"},
		{"variable reference", "docker-compose.yml", "GF_SECURITY_ADMIN_PASSWORD: ${GF_SECURITY_ADMIN_PASSWORD_FROM_ENV}"},
		{"url path", "schema.sql", `token <img src="https://cdn.jsdelivr.net/gh/Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa/a.png">`},
		{"data uri", "index.html", `auth <img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAAACXBIWXMAAAsTAAALEwEAmpwYAAAKT2lDQ1BQaG90b3Nob3A">`},
		{"notebook image", "a.ipynb", `"image/png": "iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAAACXBIWXMAAAsTAAALEwEAmpwYAAAKT2lDQ1BQaG90b3Nob3A", "token"`},
		{"lockfile", "web/package-lock.json", `"integrity": "sha512-Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa", "token"`},
		{"ide metadata", ".idea/workspace.xml", `<change beforePath="Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa" token />`},
	}
	for _, c := range cases {
		if ms := detect(c.path, c.blob); len(ms) != 0 {
			t.Errorf("%s: reported %d match(es), want none", c.name, len(ms))
		}
	}
}

// A memorable value assigned straight to a secret setting is still a hardcoded
// secret; the identifier filter must not hide it. (A JWT signing secret is
// excluded separately, by TestGenericSkipsJWTContext.)
func TestGenericHighEntropyKeepsWordyHardcodedSecret(t *testing.T) {
	ms := detect("application.properties", "app.secret=mySuperSecretSigningKeyForTokens2024")
	if len(ms) != 1 || ms[0].RuleID != "generic-high-entropy" {
		t.Fatalf("matches = %d, want one generic-high-entropy", len(ms))
	}
}

// A service-account JSON keeps the whole key on one line beside "private_key".
// It must be one finding spanning the block, not one per 64-character row, and
// two different keys must not share a fingerprint.
func TestPrivateKeyBlockSpansTheKey(t *testing.T) {
	body := strings.Repeat(`Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0QaZx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa\n`, 4)
	blob := `{"private_key": "-----BEGIN PRIVATE KEY-----\n` + body + `-----END PRIVATE KEY-----\n"}`
	ms := detect("svc-firebase-adminsdk.json", blob)
	if len(ms) != 1 || ms[0].RuleID != "private-key-block" {
		t.Fatalf("matches = %d, want one private-key-block", len(ms))
	}
	if !strings.Contains(ms[0].Secret, "-----END PRIVATE KEY-----") {
		t.Errorf("secret does not span to the END marker")
	}
}

func TestSlackWebhookURL(t *testing.T) {
	url := "https://hooks.slack.com/services/" + "T0123ABCD/B0123ABCD/" + "Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1"
	ms := detect("notify.py", "WEBHOOK = '"+url+"'")
	if len(ms) != 1 || ms[0].RuleID != "slack-webhook-url" {
		t.Fatalf("matches = %+v, want one slack-webhook-url", ms)
	}
}

// AI keys are assembled from parts so that no complete key-shaped literal sits
// in the source, where push protection and other scanners would flag it.
func TestDetectAIProviderKeys(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	h := func(n int) string { return strings.Repeat("3f9a0c7e1b", n/10+1)[:n] }
	cases := []struct{ name, blob, rule string }{
		{"openai legacy", "OPENAI_API_KEY=sk-" + r(20) + "T3Blbk" + "FJ" + r(20), "openai-api-key"},
		{"openai project", "OPENAI_API_KEY=sk-proj-" + r(74) + "T3Blbk" + "FJ" + r(74), "openai-api-key"},
		{"anthropic", "ANTHROPIC_API_KEY=sk-ant-" + "api03-" + r(93) + "AA", "anthropic-api-key"},
		{"openrouter", "key: sk-or-" + "v1-" + h(64), "openrouter-api-key"},
		{"groq", "GROQ_API_KEY=gsk" + "_" + r(52), "groq-api-key"},
		{"huggingface", "HF_TOKEN=hf" + "_" + r(34), "huggingface-token"},
		{"replicate", "REPLICATE_API_TOKEN=r8" + "_" + r(37), "replicate-api-token"},
		{"perplexity", "PPLX_KEY=pplx" + "-" + r(48), "perplexity-api-key"},
		{"xai", "XAI_API_KEY=xai" + "-" + r(80), "xai-api-key"},
		{"langsmith", "LANGSMITH_API_KEY=lsv2" + "_pt_" + h(32) + "_" + h(10), "langsmith-api-key"},
		{"pinecone", "PINECONE_API_KEY=pcsk" + "_" + r(6) + "_" + r(60), "pinecone-api-key"},
		{"deepseek", "DEEPSEEK_API_KEY=sk-" + h(32), "deepseek-api-key"},
		{"cohere", "COHERE_API_KEY=" + r(40), "cohere-api-key"},
		{"mistral", "mistral.api-key: " + r(32), "mistral-api-key"},
		{"azure openai", "AZURE_OPENAI_API_KEY=" + h(32), "azure-openai-api-key"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			got := make([]string, len(ms))
			for i, m := range ms {
				got[i] = m.RuleID
			}
			t.Errorf("%s: rules = %v, want [%s]", c.name, got, c.rule)
		}
	}
}

func TestAIProviderRulesNeedTheirShape(t *testing.T) {
	r := strings.Repeat("Zx9Kq2Lp7W", 10)
	cases := []struct{ name, blob string }{
		// sk- alone is used by too many services to attribute to OpenAI.
		{"sk- without the OpenAI marker", "const id = 'sk-" + r[:48] + "'"},
		{"hf_ inside an identifier", "pathf" + "_" + r[:34] + " = 1"},
		{"bare 32 hex, no DeepSeek setting", "commit sk-" + strings.Repeat("3f9a0c7e", 4)},
		{"template value", "OPENAI_API_KEY=your-openai-api-key"},
		{"variable reference", "GROQ_API_KEY=${GROQ_API_KEY}"},
	}
	for _, c := range cases {
		for _, m := range detect(".env", c.blob) {
			if m.Provider != "" {
				t.Errorf("%s: attributed to %s by %s", c.name, m.Provider, m.RuleID)
			}
		}
	}
}

// A random-looking value long and varied enough to clear the entropy bar.
const opaque = "Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0Qa"

// JWT signing secrets and sample tokens are left out of the generic pass by
// decision: every such hit in the scanned repositories was a tutorial default
// or a documentation sample, and they outnumbered every other unmapped finding.
func TestGenericSkipsJWTContext(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjM0NTY3ODkwIn0" + "." + "Zx9Kq2Lp7Wm4Rt8Nv3Yb6Hc1Jd5Fg0QaXy"
	cases := []struct{ name, path, blob string }{
		{"env", ".env", "JWT_SECRET=" + opaque},
		{"properties", "application.properties", "jwt.secret=" + opaque},
		{"spring default", "Config.java", `@Value("${jwt.secret:` + opaque + `}")`},
		{"jwt file", "src/security/JwtUtil.java", `private static final String SECRET = "` + opaque + `";`},
		{"yaml block", "application.yml", "jwt:\n  expiration: 3600\n  secret: " + opaque + "\n"},
		{"issued token", "AuthService.java", `String expiredToken = "` + jwt + `";`},
		{"header alone", "API.md", `"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`},
	}
	for _, c := range cases {
		if ms := detect(c.path, c.blob); len(ms) != 0 {
			t.Errorf("%s: reported %d match(es), want none", c.name, len(ms))
		}
	}
}

// The JWT exclusions must not spread to secrets that merely sit nearby.
func TestGenericJWTContextStaysNarrow(t *testing.T) {
	cases := []struct{ name, path, blob string }{
		// "jwt" inside a random string is three letters, not a context.
		{"jwt mid-word", ".env", "APP_SECRET=" + opaque + "xJWtq"},
		// A YAML block more than a few lines below jwt: belongs to something else.
		{"later yaml block", "application.yml",
			"jwt:\n  secret: ${JWT_SECRET}\n  ttl: 1\nmail:\n  host: x\n  port: 1\n  user: y\n  password: " + opaque + "\n"},
		// A keyword far away on a huge line is not context; one close by is.
		{"huge line, JWT far away", "dump.sql", "JWT " + strings.Repeat("lorem ipsum ", 400) + "api_secret: " + opaque},
	}
	for _, c := range cases {
		ms := detect(c.path, c.blob)
		if len(ms) != 1 || ms[0].RuleID != "generic-high-entropy" {
			t.Errorf("%s: matches = %d, want one generic", c.name, len(ms))
		}
	}
	far := "password " + strings.Repeat("lorem ipsum ", 100) + opaque
	if ms := detect("dump.sql", far); len(ms) != 0 {
		t.Errorf("keyword 1 KB away still counted as context: %d match(es)", len(ms))
	}
}

func TestGenericSkipsExamples(t *testing.T) {
	cases := []struct{ name, path, blob string }{
		{"example line", "README.md", "**Example:** api_key = " + opaque},
		{"schema example", "Dto.java", `@Schema(description = "token", example = "` + opaque + `")`},
		{"e.g.", "docs.md", "set API_TOKEN, e.g. " + opaque},
		{".env.example", "backend/.env.example", "API_SECRET=" + opaque},
		{"config.sample.json", "config.sample.json", `{"secret": "` + opaque + `"}`},
		{"examples dir", "examples/basic/main.go", `token := "` + opaque + `"`},
	}
	for _, c := range cases {
		if ms := detect(c.path, c.blob); len(ms) != 0 {
			t.Errorf("%s: reported %d match(es), want none", c.name, len(ms))
		}
	}
	// A named rule still runs in an example file: a real key pasted into a
	// template is still a leak, and its shape is still its own.
	if ms := detect(".env.example", "AWS_ACCESS_KEY_ID=AKIAQYLPMN5HHHFPZAM2"); len(ms) != 1 {
		t.Errorf("named rule in .env.example: %d match(es), want 1", len(ms))
	}
}

func TestGiphyAPIKey(t *testing.T) {
	key := strings.Repeat("aB3dE6gH9k", 4)[:32]
	cases := []struct{ name, blob, rule string }{
		{"env", "GIPHY_API_KEY=" + key, "giphy-api-key"},
		{"vite fallback", "const GIPHY_API_KEY = import.meta.env.VITE_GIPHY_API_KEY || '" + key + "';", "giphy-api-key"},
		{"config object", "giphy: { apiKey: '" + key + "' }", "giphy-api-key"},
		{"request url", "fetch(`https://api.giphy.com/v1/gifs/search?q=cat&api_key=" + key + "`)", "giphy-api-key-url"},
	}
	for _, c := range cases {
		ms := detect("Picker.jsx", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule || ms[0].Provider != "giphy" {
			got := make([]string, len(ms))
			for i, m := range ms {
				got[i] = m.RuleID
			}
			t.Errorf("%s: rules = %v, want [%s]", c.name, got, c.rule)
		}
	}
	// Without a Giphy setting, a 32-character value is not attributed to Giphy.
	for _, m := range detect("a.js", "const id = '"+key+"'") {
		if m.Provider == "giphy" {
			t.Errorf("bare 32-char value attributed to giphy")
		}
	}
}

// The generic pass decides on what a value is assigned to before it looks at the
// words around it. Each case here is a shape seen in the scanned corpus.
func TestGenericClassifiesTheAssignmentKey(t *testing.T) {
	reported := []struct{ name, blob string }{
		{"snake secret", "DB_PASSWORD=" + opaque},
		{"short word, whole", "DB_PWD=" + opaque}, // pwd only counts as a whole word
		{"camelCase", `clientSecret: "` + opaque + `"`},
		{"json", `{"api_key": "` + opaque + `"}`},
		{"kebab yaml", "hash-secret: " + opaque},
		{"spring default", "resend.api-key=${API_SECRET:" + opaque + "}"},
		{"recaptcha secret", "google.recaptcha.secret-key=" + opaque},
	}
	for _, c := range reported {
		ms := detect("application.properties", c.blob)
		if len(ms) != 1 || ms[0].RuleID != "generic-high-entropy" {
			t.Errorf("%s: matches = %d, want one generic", c.name, len(ms))
		}
	}

	public := []struct{ name, blob string }{
		// OAUTH contains "auth" — the old keyword list called this a secret.
		{"oauth client id", "GOOGLE_OAUTH_CLIENT_ID=" + opaque},
		{"recaptcha site key", "google.recaptcha.site-key=" + opaque},
		{"sitekey", `data-sitekey="` + opaque + `"`},
		{"publishable", "STRIPE_PUBLISHABLE_KEY=" + opaque},
		{"public key", "publicKey: " + opaque},
		{"key id", "AWS_KEY_ID=" + opaque + " # secret"},
		{"username", "db.username=" + opaque + " # password below"},
	}
	for _, c := range public {
		if ms := detect("application.properties", c.blob); len(ms) != 0 {
			t.Errorf("%s: reported %d match(es), want none", c.name, len(ms))
		}
	}
}

// Values that are public by design are not leaks, whatever they are called.
func TestGenericSkipsPublicIdentifiers(t *testing.T) {
	cases := []struct{ name, blob string }{
		{"stripe publishable", "STRIPE_KEY = 'pk_test_" + opaque + "'"},
		{"google client id", "client: 123456789012-" + strings.ToLower(opaque) + ".apps.googleusercontent.com # secret"},
	}
	for _, c := range cases {
		if ms := detect(".env", c.blob); len(ms) != 0 {
			t.Errorf("%s: reported %d match(es), want none", c.name, len(ms))
		}
	}
}

// "pwd" as a substring of random text is not a context: it used to let a URL
// parameter in a SQL dump through.
func TestGenericShortKeywordNeedsAWholeWord(t *testing.T) {
	blob := `<a href="https://x.example.org/p?__cft__[0]=` + opaque + `">xPWDq</a>`
	if ms := detect("dump.sql", blob); len(ms) != 0 {
		t.Errorf("reported %d match(es) from a substring, want none", len(ms))
	}
}

func TestGenericSkipsTestFiles(t *testing.T) {
	for _, path := range []string{
		"src/test/java/com/app/AuthServiceTest.java", "internal/scan/detect_test.go",
		"web/src/__tests__/api.js", "web/src/api.spec.ts", "tests/test_upload.py", "spec/fixtures/keys.yml",
	} {
		if ms := detect(path, "API_SECRET="+opaque); len(ms) != 0 {
			t.Errorf("%s: generic reported %d match(es), want none", path, len(ms))
		}
	}
	// A named rule still runs: a real key pasted into a test is still a leak.
	if ms := detect("src/test/java/AuthServiceTest.java", "AWS_ACCESS_KEY_ID=AKIAQYLPMN5HHHFPZAM2"); len(ms) != 1 {
		t.Errorf("named rule in a test file: %d match(es), want 1", len(ms))
	}
}

func TestResendAndGoogleRefreshToken(t *testing.T) {
	resend := "re_" + "Ab12Cd34" + "_" + strings.Repeat("Zx9Kq2Lp7W", 3)[:24]
	refresh := "1//0" + strings.Repeat("Zx9Kq2Lp7W_", 5)
	cases := []struct{ name, blob, rule string }{
		{"resend", "RESEND_API_KEY=" + resend, "resend-api-key"},
		{"google refresh", "google.refresh-token=" + refresh, "google-oauth-refresh-token"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			t.Errorf("%s: matches = %+v, want one %s", c.name, ms, c.rule)
		}
	}
	// re_ alone is ordinary code: the fixed layout keeps it from matching.
	for _, m := range detect("a.py", "m = re_compile_pattern_for_the_tokenizer(x)") {
		if m.RuleID == "resend-api-key" {
			t.Errorf("re_ in code attributed to resend")
		}
	}
}

// Suppressions a named rule's shape cannot express. Each was a false positive in
// the scanned corpus; none risks hiding a real key.
func TestNamedRuleSuppressions(t *testing.T) {
	// AWS's own documentation key is published, not a credential.
	if ms := detect(".env", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"); len(ms) != 0 {
		t.Errorf("AWS documentation key reported: %+v", ms)
	}
	// A PEM header with no key body (a literal in a test table, a doc snippet).
	if ms := detect("notes.md", "the file begins -----BEGIN RSA PRIVATE KEY----- and so on"); len(ms) != 0 {
		t.Errorf("PEM header with no body reported: %+v", ms)
	}
	// smtp://user:pass@host in a comment — pass is a placeholder.
	if ms := detect("README.md", "connect with smtp://user:pass@mail.example.org"); len(ms) != 0 {
		t.Errorf("placeholder smtp URL reported: %+v", ms)
	}
	// A Google API key inside a Firebase client config is publishable by design.
	if ms := detect("android/app/google-services.json", `"current_key": "AIzaSyD` + strings.Repeat("Zx9Kq2Lp7W", 4)[:32] + `"`); len(ms) != 0 {
		t.Errorf("Firebase client key reported: %+v", ms)
	}
	// But the same key shape in an ordinary config is still a finding.
	if ms := detect("application.properties", "gemini.api.key=AIzaSyD" + strings.Repeat("Zx9Kq2Lp7W", 4)[:32]); len(ms) != 1 {
		t.Errorf("Gemini key in a normal config: %d matches, want 1", len(ms))
	}
}

// reCAPTCHA secret keys and SDK-config cloudinary secrets are found via the word
// that names their provider in a window around the match, not on the line.
func TestKeywordWindowedRules(t *testing.T) {
	recaptcha := "6L" + strings.Repeat("Ab3dEf9kH1", 4)[:38]
	cloud := strings.Repeat("Zx9Kq2Lp7W", 3)[:27]
	cases := []struct{ name, blob, rule string }{
		{"recaptcha secret", "google.recaptcha.secret-key=" + recaptcha, "recaptcha-secret-key"},
		{"cloudinary config", "cloudinary.config({\n  cloud_name: 'demo',\n  api_secret: '" + cloud + "',\n})", "cloudinary-api-secret-config"},
	}
	for _, c := range cases {
		ms := detect("config.js", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			t.Errorf("%s: matches = %+v, want one %s", c.name, ms, c.rule)
		}
	}
}

// A synthetic fixture in a test file is suppressed; the same shape in production
// code, and a real-looking key in a test file, are both still reported — the
// suppression must never hide a real leak.
func TestSyntheticFixtureOnlyInTestFiles(t *testing.T) {
	fixture := "token: ghp_0123456789abcdefghijklmnopqrstuvwxyz"
	if ms := detect("internal/scan/detect_test.go", fixture); len(ms) != 0 {
		t.Errorf("synthetic github fixture in a test file reported: %+v", ms)
	}
	if ms := detect("internal/app/config.go", fixture); len(ms) != 1 {
		t.Errorf("synthetic-looking value in production code must still fire: %d, want 1", len(ms))
	}
	real := "token: ghp_" + strings.Repeat("Zx9Kq2Lp7W", 4)[:36]
	if ms := detect("auth/AuthServiceTest.java", real); len(ms) != 1 {
		t.Errorf("a real-looking key in a test file must still fire: %d, want 1", len(ms))
	}
}

// TestDetectPlatformProviderKeys covers the hosting / SaaS / developer-platform
// rules added alongside the AI set: each carries a self-identifying prefix, so a
// single value in a .env file must attribute to exactly its provider's rule.
func TestDetectPlatformProviderKeys(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	h := func(n int) string { return strings.Repeat("3f9a0c7e1b", n/10+1)[:n] }
	cases := []struct{ name, blob, rule string }{
		{"gitlab", "GITLAB_TOKEN=glpat" + "-" + r(20), "gitlab-pat"},
		{"npm", "//registry.npmjs.org/:_authToken=npm" + "_" + r(36), "npm-access-token"},
		{"digitalocean", "DIGITALOCEAN_TOKEN=dop" + "_v1_" + h(64), "digitalocean-token"},
		{"shopify", "SHOPIFY_TOKEN=shpat" + "_" + h(32), "shopify-token"},
		{"square access", "SQUARE_TOKEN=sq0atp" + "-" + r(22), "square-access-token"},
		{"square secret", "SQUARE_OAUTH_SECRET=sq0csp" + "-" + r(43), "square-oauth-secret"},
		{"mailchimp", "MAILCHIMP_API_KEY=" + h(32) + "-us" + "7", "mailchimp-api-key"},
		{"mailgun", "MAILGUN_API_KEY=key" + "-" + h(32), "mailgun-api-key"},
		{"sentry", "SENTRY_AUTH_TOKEN=sntrys" + "_" + r(45), "sentry-auth-token"},
		{"linear", "LINEAR_API_KEY=lin_api" + "_" + r(40), "linear-api-key"},
		{"notion", "NOTION_TOKEN=ntn" + "_" + r(46), "notion-integration-token"},
		{"figma", "FIGMA_TOKEN=figd" + "_" + r(40), "figma-token"},
		{"doppler", "DOPPLER_TOKEN=dp.st." + r(43), "doppler-token"},
		{"supabase", "SUPABASE_ACCESS_TOKEN=sbp" + "_" + h(40), "supabase-access-token"},
		{"planetscale", "DATABASE_PASSWORD=pscale_pw" + "_" + r(43), "planetscale-token"},
		{"databricks", "DATABRICKS_TOKEN=dapi" + h(32), "databricks-token"},
		{"newrelic", "NEW_RELIC_API_KEY=NRAK-" + strings.ToUpper(r(27)), "newrelic-user-key"},
		{"grafana", "GRAFANA_TOKEN=glsa" + "_" + r(40), "grafana-token"},
		{"tailscale", "TS_AUTHKEY=tskey-auth-" + r(10) + "-" + r(30), "tailscale-key"},
		{"flyio", "FLY_API_TOKEN=fo1" + "_" + r(43), "flyio-token"},
		{"discord", "WEBHOOK=https://discord.com/api/webhooks/123456789012345678/" + r(60), "discord-webhook-url"},
		{"telegram", "url=https://api.telegram.org/bot1234567890:" + r(35) + "/sendMessage", "telegram-bot-token"},
		{"twilio", "TWILIO_AUTH_TOKEN=" + h(32), "twilio-auth-token"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			got := make([]string, len(ms))
			for i, m := range ms {
				got[i] = m.RuleID
			}
			t.Errorf("%s: rules = %v, want [%s]", c.name, got, c.rule)
		}
	}
}

// TestDetectMorePlatformProviderKeys covers the second batch of platform rules.
func TestDetectMorePlatformProviderKeys(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	h := func(n int) string { return strings.Repeat("3f9a0c7e1b", n/10+1)[:n] }
	u := func(n int) string { return strings.Repeat("3F9A0C7E1B", n/10+1)[:n] }
	cases := []struct{ name, blob, rule string }{
		{"atlassian", "ATLASSIAN_API_TOKEN=ATATT3xFfGF0" + r(60), "atlassian-api-token"},
		{"dockerhub", "DOCKER_PAT=dckr_pat" + "_" + r(30), "dockerhub-pat"},
		{"vault", "VAULT_TOKEN=hvs." + r(90), "vault-token"},
		{"terraform", "TF_TOKEN=" + r(14) + ".atlasv1." + r(70), "terraform-cloud-token"},
		{"pypi", "PYPI_TOKEN=pypi-AgEIcHlwaS5vcmc" + r(60), "pypi-upload-token"},
		{"rubygems", "RUBYGEMS_API_KEY=rubygems" + "_" + h(48), "rubygems-api-key"},
		{"postman", "POSTMAN_API_KEY=PMAK-" + h(24) + "-" + h(34), "postman-api-key"},
		{"dynatrace", "DT_TOKEN=dt0c01." + u(24) + "." + u(64), "dynatrace-token"},
		{"datadog", "DD_API_KEY=" + h(32), "datadog-api-key"},
		{"cloudflare", "CLOUDFLARE_API_TOKEN=" + r(40), "cloudflare-api-token"},
		{"contentful", "CONTENTFUL_TOKEN=CFPAT-" + r(43), "contentful-token"},
		{"pulumi", "PULUMI_ACCESS_TOKEN=pul-" + h(40), "pulumi-access-token"},
		{"brevo", "BREVO_API_KEY=xkeysib-" + h(64) + "-" + r(16), "brevo-api-key"},
		{"razorpay live", "RAZORPAY_KEY=rzp_live" + "_" + r(14), "razorpay-live-key"},
		{"razorpay test", "RAZORPAY_KEY=rzp_test" + "_" + r(14), "razorpay-test-key"},
		{"flutterwave", "FLW_SECRET_KEY=FLWSECK-" + h(32) + "-X", "flutterwave-secret-key"},
		{"alibaba", "ALIBABA_ACCESS_KEY_ID=LTAI" + r(16), "alibaba-access-key-id"},
		{"tencent", "TENCENT_SECRET_ID=AKID" + r(32), "tencent-secret-id"},
		{"mapbox", "MAPBOX_SECRET=sk.eyJ" + r(60) + "." + r(30), "mapbox-secret-token"},
		{"prefect", "PREFECT_API_KEY=pnu" + "_" + r(36), "prefect-api-key"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			got := make([]string, len(ms))
			for i, m := range ms {
				got[i] = m.RuleID
			}
			t.Errorf("%s: rules = %v, want [%s]", c.name, got, c.rule)
		}
	}
}

// TestPlatformRulesNeedTheirShape guards the few platform rules whose prefix is
// generic: they must stay dark on a value that merely looks similar.
func TestPlatformRulesNeedTheirShape(t *testing.T) {
	cases := []struct{ name, blob string }{
		// A 32-hex digest is not a Twilio secret without the word twilio nearby.
		{"bare 32 hex, no twilio", "CHECKSUM=" + strings.Repeat("3f9a0c7e", 4)},
		// key- without exactly 32 hex following is not a Mailgun key.
		{"key- prefix, wrong body", "monkey-business = 1"},
		// A template value must not attribute.
		{"gitlab template", "GITLAB_TOKEN=glpat-your-token-goes-here"},
	}
	for _, c := range cases {
		for _, m := range detect(".env", c.blob) {
			if m.Provider != "" {
				t.Errorf("%s: attributed to %s by %s", c.name, m.Provider, m.RuleID)
			}
		}
	}
}

// TestDetectCloudPaymentAndPlatformKeys covers the third batch: cloud and
// payment secrets anchored on their setting name, and more prefixed tokens.
func TestDetectCloudPaymentAndPlatformKeys(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	h := func(n int) string { return strings.Repeat("3f9a0c7e1b", n/10+1)[:n] }
	u := func(n int) string { return strings.Repeat("ZX9KQ2LP7W", n/10+1)[:n] }
	cases := []struct{ name, blob, rule string }{
		{"aws secret env", "AWS_SECRET_ACCESS_KEY=" + r(38) + "/+", "aws-secret-access-key"},
		{"aws secret ini", "[default]\naws_secret_access_key = " + r(40) + "\n", "aws-secret-access-key"},
		{"aws secret js", "new AWS.S3({ secretAccessKey: '" + r(40) + "' })", "aws-secret-access-key"},
		{"azure storage", "DefaultEndpointsProtocol=https;AccountName=acme;AccountKey=" + r(86) + "==;EndpointSuffix=core.windows.net", "azure-storage-account-key"},
		{"azure ad", "AZURE_CLIENT_SECRET=" + r(3) + "8Q~" + r(34), "azure-ad-client-secret"},
		{"stripe restricted", "STRIPE_KEY=rk_live" + "_" + r(40), "stripe-restricted-key"},
		{"stripe webhook", "STRIPE_WEBHOOK_SECRET=whsec" + "_" + r(32), "stripe-webhook-secret"},
		{"braintree", "BT=access_token$production$" + h(16) + "$" + h(32), "paypal-braintree-access-token"},
		{"vnpay", "vnp_HashSecret=" + u(32), "vnpay-hash-secret"},
		{"vnpay env", "VNPAY_HASH_SECRET=\"" + u(32) + "\"", "vnpay-hash-secret"},
		{"momo", "momo:\n  partnerCode: MOMOX\n  secretKey: " + r(32) + "\n", "momo-secret-key"},
		{"zalopay", "zalopay:\n  app_id: 2553\n  key1: " + r(32) + "\n", "zalopay-key"},
		{"slack app", "SLACK_APP_TOKEN=xapp-1-A0" + u(8) + "-1234567890123-" + h(64), "slack-app-token"},
		{"gitlab deploy", "CI_DEPLOY_TOKEN=gldt" + "-" + r(20), "gitlab-token"},
		{"heroku", "HEROKU_API_KEY=HRKU-AA" + r(58), "heroku-api-key"},
		{"supabase secret", "SUPABASE_SECRET_KEY=sb_secret" + "_" + r(31), "supabase-secret-key"},
		{"fcm", "FCM_SERVER_KEY=AAAA" + r(7) + ":APA91b" + r(134), "firebase-fcm-server-key"},
		{"twitter", "TWITTER_BEARER_TOKEN=" + strings.Repeat("A", 22) + r(90), "twitter-bearer-token"},
		{"discord bot", "DISCORD_BOT_TOKEN=M" + r(23) + "." + r(6) + "." + r(38), "discord-bot-token"},
		{"age", "AGE_KEY=AGE-SECRET-KEY-1" + u(58), "age-secret-key"},
		{"github refresh", "token=ghr" + "_" + r(36), "github-pat"},
	}
	for _, c := range cases {
		ms := detect(".env", c.blob)
		if len(ms) != 1 || ms[0].RuleID != c.rule {
			got := make([]string, len(ms))
			for i, m := range ms {
				got[i] = m.RuleID
			}
			t.Errorf("%s: rules = %v, want [%s]", c.name, got, c.rule)
		}
	}
}

// TestContextAnchoredRulesNeedTheirContext guards the rules whose value has no
// shape of its own: without the setting name or the provider's word nearby they
// must not attribute, and published sandbox keys must not fire at all.
func TestContextAnchoredRulesNeedTheirContext(t *testing.T) {
	r := func(n int) string { return strings.Repeat("Zx9Kq2Lp7W", n/10+1)[:n] }
	cases := []struct{ name, blob string }{
		{"secret access key, no aws", "minio:\n  secret_access_key: " + r(40)},
		{"secretKey, no momo", "jwt:\n  secretKey: " + r(32)},
		{"key1, no zalopay", "cache:\n  key1: " + r(32)},
		{"discord-shaped, no discord", "token=M" + r(23) + "." + r(6) + "." + r(38)},
		{"A-run, no bearer", "data=" + strings.Repeat("A", 22) + r(90)},
		{"momo sandbox", "momo:\n  secretKey: K951B6PE1waDMi640xX08PD3vg6EkVlz"},
		{"azurite", "AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="},
		{"aws docs secret", "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
	}
	for _, c := range cases {
		for _, m := range detect("config.yml", c.blob) {
			if m.Provider != "" {
				t.Errorf("%s: attributed to %s by %s", c.name, m.Provider, m.RuleID)
			}
		}
	}
}
