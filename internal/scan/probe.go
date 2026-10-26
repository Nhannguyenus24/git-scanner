package scan

import (
	"context"
	"encoding/base32"
	"strings"
)

// Probing turns a leaked credential into something a user can act on: not just
// "a key-shaped string is in your history" but "this key is live, it belongs to
// this account, rotate it now". There are two layers, with very different
// risk, and the engine is the only place either can run because it is the only
// place the plaintext secret exists (plan.md §6).
//
//  1. Offline extraction decodes facts the credential carries in itself — the
//     AWS account id embedded in an access key id, a Stripe key's live/test
//     mode. No network, no risk; always on.
//  2. A liveness probe authenticates to the provider with the found credential
//     and reads back its identity. It makes an outbound request with someone's
//     key, so it is gated (Engine.prober is nil unless the operator enabled it
//     for targets they own) and strictly identity-only: whether the key works
//     and whose account it is, never an enumeration of what the account holds.
//
// Everything produced here is non-secret and safe to store and show. The secret
// itself is consumed into a fingerprint and never travels onward.

// ProbeOutcome is the result of a liveness probe.
type ProbeOutcome string

const (
	ProbeLive       ProbeOutcome = "live"       // the credential authenticated
	ProbeDead       ProbeOutcome = "dead"       // the provider rejected it
	ProbeUnverified ProbeOutcome = "unverified" // no prober for this provider
	ProbeError      ProbeOutcome = "error"      // the probe could not complete
)

// ProbeResult is what a liveness probe learned. Identity holds only non-secret
// facts — account id, owner, scopes — suitable to store and display.
type ProbeResult struct {
	Outcome  ProbeOutcome
	Identity map[string]string
}

// Prober validates one credential against its provider with a single
// identity/liveness request. Implementations MUST NOT enumerate account
// resources and MUST return only non-secret facts. A Prober is only ever
// invoked for a provider slug a named rule assigned.
type Prober interface {
	Probe(ctx context.Context, provider, secret string) ProbeResult
}

// extractOffline derives non-secret metadata from the credential alone, with no
// network access. It returns the keys to merge into a finding's metadata:
//
//   - key_type: which kind of credential the provider issued (a GitHub
//     fine-grained token, a Slack bot token, an OpenAI project key), read from
//     its prefix. The kind decides the blast radius more than the provider does.
//   - mode: live or test, for providers that issue both.
//   - region: the datacentre a key is pinned to, where the key spells it out.
//   - aws_account_id: the account an AWS access key id belongs to.
//
// Stripe's mode was stored as stripe_mode before mode was general; readers fall
// back to it for findings stored then.
func extractOffline(provider, secret string) map[string]string {
	meta := map[string]string{}
	for _, kt := range keyTypes[provider] {
		if strings.HasPrefix(secret, kt.prefix) {
			meta["key_type"] = kt.keyType
			break
		}
	}
	if m := credentialMode(provider, secret); m != "" {
		meta["mode"] = m
	}
	switch provider {
	case "aws":
		if id, ok := awsAccountID(secret); ok {
			meta["aws_account_id"] = id
		}
	case "mailchimp":
		// The key ends in its datacentre, -us1 … -us21, which is also the API host.
		if i := strings.LastIndex(secret, "-"); i >= 0 {
			meta["region"] = secret[i+1:]
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// keyTypes maps a provider to the prefixes that distinguish its credential
// kinds. Within a provider the first match wins, so a prefix that extends
// another (sk-proj- and sk-) must come before it.
var keyTypes = map[string][]struct{ prefix, keyType string }{
	"aws":          {{"AKIA", "long-term"}, {"ASIA", "temporary"}},
	"github":       {{"github_pat_", "fine-grained-pat"}, {"ghp_", "classic-pat"}, {"gho_", "oauth"}, {"ghu_", "user-to-server"}, {"ghs_", "server-to-server"}},
	"slack":        {{"https://hooks.slack.com/", "webhook"}, {"xoxb-", "bot"}, {"xoxp-", "user"}, {"xoxa-", "app"}, {"xoxr-", "refresh"}, {"xoxs-", "session"}},
	"openai":       {{"sk-proj-", "project"}, {"sk-svcacct-", "service-account"}, {"sk-admin-", "admin"}, {"sk-", "legacy"}},
	"anthropic":    {{"sk-ant-admin", "admin"}, {"sk-ant-api", "api"}},
	"google":       {{"AIza", "api-key"}, {"GOCSPX-", "oauth-client-secret"}, {"1//0", "refresh-token"}, {"6L", "recaptcha-secret"}},
	"stripe":       {{"sk_", "secret-key"}, {"rk_", "restricted-key"}},
	"digitalocean": {{"dop_v1_", "personal"}, {"doo_v1_", "oauth"}, {"dor_v1_", "refresh"}, {"dot_v1_", "temporary"}},
	"shopify":      {{"shpat_", "app-access-token"}, {"shpca_", "custom-app"}, {"shppa_", "private-app"}, {"shpss_", "shared-secret"}},
	"square":       {{"sq0atp-", "access-token"}, {"sq0csp-", "oauth-secret"}},
	"sentry":       {{"sntrys_", "org-auth-token"}, {"sntryu_", "user-auth-token"}},
	"doppler":      {{"dp.pt.", "personal"}, {"dp.st.", "service"}, {"dp.ct.", "cli"}, {"dp.sa.", "service-account"}, {"dp.scim.", "scim"}, {"dp.audit.", "audit"}},
	"planetscale":  {{"pscale_pw_", "password"}, {"pscale_tkn_", "service-token"}},
	"vault":        {{"hvs.", "service"}, {"hvb.", "batch"}, {"hvr.", "recovery"}},
	"tailscale":    {{"tskey-auth-", "auth"}, {"tskey-api-", "api"}, {"tskey-client-", "oauth-client"}},
	"grafana":      {{"glsa_", "service-account"}, {"glc_", "cloud-access-policy"}},
}

// credentialMode reports live or test for providers whose keys say which.
func credentialMode(provider, secret string) string {
	switch provider {
	case "stripe", "razorpay":
		if strings.Contains(secret, "_live_") {
			return "live"
		}
		if strings.Contains(secret, "_test_") {
			return "test"
		}
	case "flutterwave":
		if strings.HasPrefix(secret, "FLWSECK_TEST-") {
			return "test"
		}
		return "live"
	}
	return ""
}

// awsBase32 is RFC 4648 base32 without padding, which is how an access key id is
// encoded.
var awsBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// awsAccountID recovers the 12-digit account number that AWS encodes into an
// access key id. The key id is the public identifier, not the secret — it is
// routinely committed beside the secret — and this only reads back a number
// already baked into it, offline. The method is the documented one: drop the
// four-character prefix, base32-decode, and extract 34 bits from the first six
// bytes.
func awsAccountID(keyID string) (string, bool) {
	if len(keyID) < 20 {
		return "", false
	}
	raw, err := awsBase32.DecodeString(keyID[4:20])
	if err != nil || len(raw) < 6 {
		return "", false
	}
	var z uint64
	for _, b := range raw[:6] {
		z = z<<8 | uint64(b)
	}
	const mask = 0x7fffffffff80
	acct := (z & mask) >> 7
	return pad12(acct), true
}

func pad12(n uint64) string {
	s := itoa(n)
	for len(s) < 12 {
		s = "0" + s
	}
	return s
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
