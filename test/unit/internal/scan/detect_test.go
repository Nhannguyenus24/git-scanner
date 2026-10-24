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
		{"aws", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLX", "aws-access-key-id"},
		{"github pat", "token: ghp_0123456789abcdefghijklmnopqrstuvwxyz", "github-pat"},
		{"stripe", "STRIPE_KEY=" + dummyStripeKey, "stripe-secret-key"},
		{"cloudinary", "url=cloudinary://123456789012345:abcdefghijklmnopqrstu@demo", "cloudinary-url"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----", "private-key-block"},
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
	if ms := detect("vendor/x/f.go", "AKIAIOSFODNN7EXAMPLX"); len(ms) != 0 {
		t.Errorf("vendored path was scanned: %+v", ms)
	}
	if ms := detect("a.go", "\x00AKIAIOSFODNN7EXAMPLX"); len(ms) != 0 {
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
	if ms := detect(".env.example", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLX"); len(ms) != 1 {
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
