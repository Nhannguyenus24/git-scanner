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
// network access. It returns the keys to merge into a finding's metadata.
func extractOffline(provider, secret string) map[string]string {
	switch provider {
	case "aws":
		if id, ok := awsAccountID(secret); ok {
			return map[string]string{"aws_account_id": id}
		}
	case "stripe":
		if strings.Contains(secret, "_live_") {
			return map[string]string{"stripe_mode": "live"}
		}
		if strings.Contains(secret, "_test_") {
			return map[string]string{"stripe_mode": "test"}
		}
	}
	return nil
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
