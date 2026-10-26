package probe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mservice/gitscanner/internal/probe"
	"github.com/mservice/gitscanner/internal/scan"
)

// server stands in for a provider API: it checks the credential arrived the way
// that provider expects, and returns the body the prober reads identity from.
func server(t *testing.T, want func(*http.Request) bool, status int, body string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !want(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestProbeGitHubLive(t *testing.T) {
	s := server(t, func(r *http.Request) bool {
		return r.URL.Path == "/user" && r.Header.Get("Authorization") == "Bearer ghp_real"
	}, 200, `{"login":"octocat"}`)
	got := probe.New(nil).WithEndpoints(map[string]string{"github": s.URL}).
		Probe(context.Background(), "github", "ghp_real")
	if got.Outcome != scan.ProbeLive || got.Identity["owner"] != "octocat" {
		t.Errorf("got %+v, want live/octocat", got)
	}
}

func TestProbeGitHubDead(t *testing.T) {
	s := server(t, func(r *http.Request) bool { return false }, 200, "")
	got := probe.New(nil).WithEndpoints(map[string]string{"github": s.URL}).
		Probe(context.Background(), "github", "ghp_revoked")
	if got.Outcome != scan.ProbeDead {
		t.Errorf("outcome = %s, want dead", got.Outcome)
	}
}

func TestProbeStripeBasicAuth(t *testing.T) {
	// Stripe takes the key as the basic-auth username; base64("sk_live_x:") .
	s := server(t, func(r *http.Request) bool {
		u, p, ok := r.BasicAuth()
		return ok && u == "sk_live_x" && p == "" && r.URL.Path == "/v1/account"
	}, 200, `{"id":"acct_123"}`)
	got := probe.New(nil).WithEndpoints(map[string]string{"stripe": s.URL}).
		Probe(context.Background(), "stripe", "sk_live_x")
	if got.Outcome != scan.ProbeLive || got.Identity["account"] != "acct_123" {
		t.Errorf("got %+v, want live/acct_123", got)
	}
}

// Slack answers 200 even for a bad token, so the body's ok flag decides.
func TestProbeSlackBodyDecides(t *testing.T) {
	live := server(t, func(r *http.Request) bool { return true }, 200, `{"ok":true,"team":"T1","user":"U1"}`)
	if got := probe.New(nil).WithEndpoints(map[string]string{"slack": live.URL}).
		Probe(context.Background(), "slack", "xoxb-x"); got.Outcome != scan.ProbeLive || got.Identity["team"] != "T1" {
		t.Errorf("got %+v, want live/T1", got)
	}
	dead := server(t, func(r *http.Request) bool { return true }, 200, `{"ok":false,"error":"invalid_auth"}`)
	if got := probe.New(nil).WithEndpoints(map[string]string{"slack": dead.URL}).
		Probe(context.Background(), "slack", "xoxb-bad"); got.Outcome != scan.ProbeDead {
		t.Errorf("outcome = %s, want dead", got.Outcome)
	}
}

func TestProbeGiphyAPIKeyInQuery(t *testing.T) {
	s := server(t, func(r *http.Request) bool {
		return r.URL.Query().Get("api_key") == "giphykey"
	}, 200, `{"data":[]}`)
	got := probe.New(nil).WithEndpoints(map[string]string{"giphy": s.URL}).
		Probe(context.Background(), "giphy", "giphykey")
	if got.Outcome != scan.ProbeLive {
		t.Errorf("outcome = %s, want live", got.Outcome)
	}
}

// A provider with no prober is reported unverified, not guessed at.
func TestProbeUnknownProvider(t *testing.T) {
	if got := probe.New(nil).Probe(context.Background(), "nonesuch", "x"); got.Outcome != scan.ProbeUnverified {
		t.Errorf("outcome = %s, want unverified", got.Outcome)
	}
}

// A 5xx or rate limit is neither live nor dead: it is an error we cannot read.
func TestProbeServerErrorIsError(t *testing.T) {
	s := server(t, func(r *http.Request) bool { return true }, 500, "")
	if got := probe.New(nil).WithEndpoints(map[string]string{"openai": s.URL}).
		Probe(context.Background(), "openai", "sk-x"); got.Outcome != scan.ProbeError {
		t.Errorf("outcome = %s, want error", got.Outcome)
	}
}

// headerServer answers with both a body and response headers, so a probe can
// read the quota facts a provider volunteers alongside identity.
func headerServer(t *testing.T, status int, headers map[string]string, body string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

// A live GitHub key records the rate-limit allowance and scopes the /user
// response already carried — no extra request.
func TestProbeGitHubCapturesQuota(t *testing.T) {
	s := headerServer(t, 200, map[string]string{
		"X-RateLimit-Remaining": "4900",
		"X-RateLimit-Limit":     "5000",
		"X-OAuth-Scopes":        "repo, read:org",
	}, `{"login":"octocat"}`)
	got := probe.New(nil).WithEndpoints(map[string]string{"github": s.URL}).
		Probe(context.Background(), "github", "ghp_real")
	if got.Outcome != scan.ProbeLive {
		t.Fatalf("outcome = %s, want live", got.Outcome)
	}
	for k, want := range map[string]string{"owner": "octocat", "rate_remaining": "4900", "rate_limit": "5000", "scopes": "repo, read:org"} {
		if got.Identity[k] != want {
			t.Errorf("identity[%s] = %q, want %q", k, got.Identity[k], want)
		}
	}
}

// OpenAI's models endpoint returns request/token allowance in headers.
func TestProbeOpenAICapturesQuota(t *testing.T) {
	s := headerServer(t, 200, map[string]string{
		"x-ratelimit-remaining-requests": "58",
		"x-ratelimit-limit-requests":     "60",
		"x-ratelimit-remaining-tokens":   "149000",
	}, "")
	got := probe.New(nil).WithEndpoints(map[string]string{"openai": s.URL}).
		Probe(context.Background(), "openai", "sk-real")
	if got.Outcome != scan.ProbeLive {
		t.Fatalf("outcome = %s, want live", got.Outcome)
	}
	if got.Identity["rate_remaining"] != "58" || got.Identity["tokens_remaining"] != "149000" {
		t.Errorf("quota = %+v, want 58 requests / 149000 tokens", got.Identity)
	}
}

// Stripe's account carries the account's country and currency, non-secret facts
// worth showing beside the id.
func TestProbeStripeCapturesAccountFacts(t *testing.T) {
	s := server(t, func(r *http.Request) bool { return r.URL.Path == "/v1/account" },
		200, `{"id":"acct_1","country":"VN","default_currency":"vnd"}`)
	got := probe.New(nil).WithEndpoints(map[string]string{"stripe": s.URL}).
		Probe(context.Background(), "stripe", "sk_live_x")
	if got.Identity["country"] != "VN" || got.Identity["currency"] != "VND" {
		t.Errorf("facts = %+v, want VN/VND", got.Identity)
	}
}

// A dead key records no quota: the allowance belongs to a working key only.
func TestProbeDeadCapturesNoQuota(t *testing.T) {
	s := headerServer(t, 401, map[string]string{"X-RateLimit-Remaining": "4900"}, "")
	got := probe.New(nil).WithEndpoints(map[string]string{"github": s.URL}).
		Probe(context.Background(), "github", "ghp_revoked")
	if got.Outcome != scan.ProbeDead {
		t.Fatalf("outcome = %s, want dead", got.Outcome)
	}
	if _, ok := got.Identity["rate_remaining"]; ok {
		t.Error("a dead key must record no quota")
	}
}
