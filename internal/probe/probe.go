// Package probe validates a leaked credential by making a single
// identity/liveness call to its provider, so the scanner can tell a user a key
// is still active and whose account it is.
//
// Every probe here is read-only and identity-only. It asks "does this key work,
// and who does it belong to" — GitHub's /user, Stripe's /account, Slack's
// auth.test — and never lists, reads or changes account resources. The point is
// a warning ("this key is LIVE, rotate it"), not access to the account.
//
// Alongside identity, a probe records the quota and rate-limit facts the
// provider volunteers in that same response: GitHub and OpenAI return the key's
// remaining allowance in response headers, Stripe its account country. Reading
// what the response already carries is zero extra requests and still no
// enumeration — it just gives the user "this key is live AND has 4,900 calls
// left this hour" instead of liveness alone.
//
// It runs only when the operator turned probing on for targets they own
// (config PROBE_SECRETS), and only inside the scan engine, where the plaintext
// already exists; nothing here is reachable from a path that does not already
// hold the secret.
package probe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/mservice/gitscanner/internal/scan"
)

// Prober implements scan.Prober over real provider HTTP APIs.
type Prober struct {
	client    *http.Client
	endpoints map[string]string // provider -> base URL; overridable in tests
	dial      dialFunc          // database connections; guardedDial unless a test overrides it
}

// New returns a Prober that calls the real provider endpoints.
func New(client *http.Client) *Prober {
	if client == nil {
		client = http.DefaultClient
	}
	return &Prober{client: client, endpoints: defaultEndpoints, dial: guardedDial}
}

// WithDialer replaces the guarded dialer the database probes use, so tests can
// reach a fake server on loopback, which the real dialer refuses.
func (p *Prober) WithDialer(d func(ctx context.Context, network, addr string) (net.Conn, error)) *Prober {
	p.dial = d
	return p
}

// WithEndpoints overrides provider base URLs, so tests can point at httptest
// servers. Keys are provider slugs.
func (p *Prober) WithEndpoints(e map[string]string) *Prober {
	merged := map[string]string{}
	for k, v := range p.endpoints {
		merged[k] = v
	}
	for k, v := range e {
		merged[k] = v
	}
	p.endpoints = merged
	return p
}

var defaultEndpoints = map[string]string{
	"github":   "https://api.github.com",
	"openai":   "https://api.openai.com",
	"stripe":   "https://api.stripe.com",
	"sendgrid": "https://api.sendgrid.com",
	"slack":    "https://slack.com",
	"giphy":    "https://api.giphy.com",
}

// prober is one provider's identity call.
type prober func(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult

var probers = map[string]prober{
	"github":   probeGitHub,
	"openai":   probeBearer("/v1/models"),
	"stripe":   probeStripe,
	"sendgrid": probeBearer("/v3/scopes"),
	"slack":    probeSlack,
	"giphy":    probeGiphy,
}

// Probe routes to the provider's identity call, or reports Unverified when no
// prober is defined for it.
func (p *Prober) Probe(ctx context.Context, provider, secret string) scan.ProbeResult {
	if db, ok := dbProbers[provider]; ok {
		return db(ctx, p.dial, secret) // the secret is the whole connection URL
	}
	fn, ok := probers[provider]
	if !ok {
		return scan.ProbeResult{Outcome: scan.ProbeUnverified}
	}
	base, ok := p.endpoints[provider]
	if !ok {
		return scan.ProbeResult{Outcome: scan.ProbeUnverified}
	}
	return fn(ctx, p, base, secret)
}

// classify maps an HTTP status to an outcome: success is live, an auth
// rejection is dead, and anything else (rate limit, 5xx, transport error) is an
// error we cannot read as either.
func classify(status int) scan.ProbeOutcome {
	switch {
	case status >= 200 && status < 300:
		return scan.ProbeLive
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return scan.ProbeDead
	default:
		return scan.ProbeError
	}
}

// do issues one request and reads a bounded prefix of the body, so a hostile or
// huge response cannot exhaust memory.
func (p *Prober) do(ctx context.Context, method, url string, header http.Header) (int, http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	for k, vs := range header {
		req.Header[k] = vs
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, resp.Header, body, nil
}

// quotaFacts reads the non-secret quota and rate-limit fields a provider sends
// in its response headers, normalised to a small shared vocabulary. They ride
// the identity response, so there is no extra request and nothing enumerated —
// the provider offers them unasked. Returns nil when the response carries none.
func quotaFacts(h http.Header) map[string]string {
	out := map[string]string{}
	first := func(key string, names ...string) {
		for _, n := range names {
			if v := h.Get(n); v != "" {
				out[key] = v
				return
			}
		}
	}
	first("rate_remaining", "X-RateLimit-Remaining", "X-Ratelimit-Remaining-Requests", "Ratelimit-Remaining")
	first("rate_limit", "X-RateLimit-Limit", "X-Ratelimit-Limit-Requests", "Ratelimit-Limit")
	first("tokens_remaining", "X-Ratelimit-Remaining-Tokens")
	first("scopes", "X-Oauth-Scopes")
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeFacts adds extra facts to a result's Identity without overwriting the
// identity fields a prober set first.
func mergeFacts(out *scan.ProbeResult, extra map[string]string) {
	if len(extra) == 0 {
		return
	}
	if out.Identity == nil {
		out.Identity = map[string]string{}
	}
	for k, v := range extra {
		if _, ok := out.Identity[k]; !ok {
			out.Identity[k] = v
		}
	}
}

func bearer(secret string) http.Header {
	return http.Header{"Authorization": {"Bearer " + secret}}
}

// probeBearer is the common shape: GET path with a bearer token, status decides
// the outcome, no identity parsed.
func probeBearer(path string) prober {
	return func(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
		status, header, _, err := p.do(ctx, http.MethodGet, base+path, bearer(secret))
		if err != nil {
			return scan.ProbeResult{Outcome: scan.ProbeError}
		}
		out := scan.ProbeResult{Outcome: classify(status)}
		if out.Outcome == scan.ProbeLive {
			mergeFacts(&out, quotaFacts(header))
		}
		return out
	}
}

func probeGitHub(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	status, header, body, err := p.do(ctx, http.MethodGet, base+"/user",
		http.Header{"Authorization": {"Bearer " + secret}, "Accept": {"application/vnd.github+json"}})
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	out := scan.ProbeResult{Outcome: classify(status)}
	if out.Outcome == scan.ProbeLive {
		var u struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(body, &u)
		if u.Login != "" {
			out.Identity = map[string]string{"owner": u.Login}
		}
		mergeFacts(&out, quotaFacts(header))
	}
	return out
}

func probeStripe(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	// Stripe uses the secret as the HTTP basic-auth username with no password.
	status, header, body, err := p.do(ctx, http.MethodGet, base+"/v1/account",
		http.Header{"Authorization": {basic(secret)}})
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	out := scan.ProbeResult{Outcome: classify(status)}
	if out.Outcome == scan.ProbeLive {
		var a struct {
			ID       string `json:"id"`
			Country  string `json:"country"`
			Currency string `json:"default_currency"`
		}
		_ = json.Unmarshal(body, &a)
		id := map[string]string{}
		if a.ID != "" {
			id["account"] = a.ID
		}
		if a.Country != "" {
			id["country"] = a.Country
		}
		if a.Currency != "" {
			id["currency"] = strings.ToUpper(a.Currency)
		}
		if len(id) > 0 {
			out.Identity = id
		}
		mergeFacts(&out, quotaFacts(header))
	}
	return out
}

// probeSlack is special: Slack answers 200 even for a bad token, with
// {"ok": false} in the body, so the body decides, not the status.
func probeSlack(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	status, _, body, err := p.do(ctx, http.MethodGet, base+"/api/auth.test", bearer(secret))
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	if status < 200 || status >= 300 {
		return scan.ProbeResult{Outcome: classify(status)}
	}
	var r struct {
		OK   bool   `json:"ok"`
		Team string `json:"team"`
		User string `json:"user"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	if !r.OK {
		return scan.ProbeResult{Outcome: scan.ProbeDead}
	}
	id := map[string]string{}
	if r.Team != "" {
		id["team"] = r.Team
	}
	if r.User != "" {
		id["owner"] = r.User
	}
	if len(id) == 0 {
		id = nil
	}
	return scan.ProbeResult{Outcome: scan.ProbeLive, Identity: id}
}

func probeGiphy(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	status, header, _, err := p.do(ctx, http.MethodGet,
		base+"/v1/gifs/trending?limit=1&api_key="+secret, nil)
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	out := scan.ProbeResult{Outcome: classify(status)}
	if out.Outcome == scan.ProbeLive {
		mergeFacts(&out, quotaFacts(header))
	}
	return out
}

// basic is the HTTP basic-auth header for a key used as the username with an
// empty password, which is how Stripe authenticates a secret key.
func basic(user string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"))
}
