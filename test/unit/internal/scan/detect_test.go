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
		{"camelCase method", "AuthTest.java", "export const getChangePasswordRequestSchema = (t) => token(t);"},
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
// signing key; the identifier filter must not hide it.
func TestGenericHighEntropyKeepsWordyHardcodedSecret(t *testing.T) {
	ms := detect("application.properties", "jwt.secret=mySuperSecretSigningKeyForTokens2024")
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
