// Package probe validates a leaked credential by making a single
// identity/liveness call to its provider, so the scanner can tell a user a key
// is still active and whose account it is.
//
// Every probe here is read-only and identity-only. It asks "does this key work,
// and who does it belong to" — GitHub's /user, Stripe's /account, Slack's
// auth.test — and never lists, reads or changes account resources. The point is
// a warning ("this key is LIVE, rotate it"), not access to the account.
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
	"net/http"

	"github.com/mservice/gitscanner/internal/scan"
)

// Prober implements scan.Prober over real provider HTTP APIs.
type Prober struct {
	client    *http.Client
	endpoints map[string]string // provider -> base URL; overridable in tests
}

// New returns a Prober that calls the real provider endpoints.
func New(client *http.Client) *Prober {
	if client == nil {
		client = http.DefaultClient
	}
	return &Prober{client: client, endpoints: defaultEndpoints}
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
func (p *Prober) do(ctx context.Context, method, url string, header http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, vs := range header {
		req.Header[k] = vs
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, body, nil
}

func bearer(secret string) http.Header {
	return http.Header{"Authorization": {"Bearer " + secret}}
}

// probeBearer is the common shape: GET path with a bearer token, status decides
// the outcome, no identity parsed.
func probeBearer(path string) prober {
	return func(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
		status, _, err := p.do(ctx, http.MethodGet, base+path, bearer(secret))
		if err != nil {
			return scan.ProbeResult{Outcome: scan.ProbeError}
		}
		return scan.ProbeResult{Outcome: classify(status)}
	}
}

func probeGitHub(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	status, body, err := p.do(ctx, http.MethodGet, base+"/user",
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
	}
	return out
}

func probeStripe(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	// Stripe uses the secret as the HTTP basic-auth username with no password.
	status, body, err := p.do(ctx, http.MethodGet, base+"/v1/account",
		http.Header{"Authorization": {basic(secret)}})
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	out := scan.ProbeResult{Outcome: classify(status)}
	if out.Outcome == scan.ProbeLive {
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &a)
		if a.ID != "" {
			out.Identity = map[string]string{"account": a.ID}
		}
	}
	return out
}

// probeSlack is special: Slack answers 200 even for a bad token, with
// {"ok": false} in the body, so the body decides, not the status.
func probeSlack(ctx context.Context, p *Prober, base, secret string) scan.ProbeResult {
	status, body, err := p.do(ctx, http.MethodGet, base+"/api/auth.test", bearer(secret))
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
	status, _, err := p.do(ctx, http.MethodGet,
		base+"/v1/gifs/trending?limit=1&api_key="+secret, nil)
	if err != nil {
		return scan.ProbeResult{Outcome: scan.ProbeError}
	}
	return scan.ProbeResult{Outcome: classify(status)}
}

// basic is the HTTP basic-auth header for a key used as the username with an
// empty password, which is how Stripe authenticates a secret key.
func basic(user string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"))
}
