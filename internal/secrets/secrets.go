// Package secrets turns a raw detected credential into the two forms that are
// safe to store: a keyed fingerprint and a masked preview (plan.md §6).
//
// The plaintext itself never leaves this package unencrypted. Everything the
// rest of the system persists, logs, publishes on a queue or mails is derived
// here, so a review of this package is enough to be sure a live secret cannot
// reach the database in the clear. The one stored form of the value is Sealer's
// ciphertext, kept only for repositories whose owner is on the unmask list.
package secrets

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"unicode/utf8"
)

// Hasher derives a stable fingerprint from a secret under a keyed HMAC.
//
// The key must come from configuration, never a constant: an unkeyed hash of a
// known-format credential is brute-forceable, and the whole point is that the
// findings table reveals nothing about the values it indexes.
type Hasher struct {
	key []byte
}

// NewHasher returns a Hasher. The key must be at least 16 bytes.
func NewHasher(key []byte) (*Hasher, error) {
	if len(key) < 16 {
		return nil, errors.New("secrets: fingerprint key must be at least 16 bytes")
	}
	// Copy so a caller mutating its slice cannot change the fingerprints later.
	k := make([]byte, len(key))
	copy(k, key)
	return &Hasher{key: k}, nil
}

// Fingerprint is HMAC-SHA256(secret) in hex. Stable across scans for the same
// secret, so it serves as both the dedupe key and the idempotency key for a
// redelivered task (plan.md §6).
func (h *Hasher) Fingerprint(secret string) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

// Mask returns a preview safe to display, log and email: a short head and tail
// with the middle elided, e.g. "sk_live_4f2a…9c1d". A short secret is reduced
// to the separator alone, never shown, so masking can never widen exposure.
//
// The ellipsis is mandatory and is what entity.Finding.Validate checks for, so
// a masking bug that passed plaintext through would be caught before any write.
func Mask(secret string) string {
	const (
		head = 4
		tail = 4
		sep  = "…"
	)

	n := utf8.RuneCountInString(secret)
	// Need room for head + tail plus at least one elided rune, or masking would
	// reveal the whole value.
	if n < head+tail+1 {
		return sep
	}

	runes := []rune(secret)
	return string(runes[:head]) + sep + string(runes[n-tail:])
}
