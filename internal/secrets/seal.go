package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// Sealer encrypts a raw credential for the one flow allowed to keep it: an
// operator testing detection against repositories whose owner they have put on
// the unmask list (their own accounts). Everything else still leaves this
// package only as a fingerprint or a masked preview.
//
// AES-256-GCM, with a fresh random nonce prefixed to each ciphertext. The key
// comes from configuration and never from the database, so a dump of the
// sealed_secrets table alone reveals nothing.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer takes the key as 64 hex characters (32 bytes), the form
// `openssl rand -hex 32` prints.
func NewSealer(hexKey string) (*Sealer, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("secrets: seal key must be 64 hex characters (32 bytes)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: seal cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: seal cipher: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts secret. aad binds the ciphertext to where it is stored, so a
// sealed value copied onto another finding's row fails to open.
func (s *Sealer) Seal(secret string, aad []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secrets: seal nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, []byte(secret), aad), nil
}

// Open reverses Seal. It fails on a wrong key, a tampered ciphertext or a
// mismatched aad.
func (s *Sealer) Open(sealed, aad []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("secrets: sealed value too short")
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], aad)
	if err != nil {
		return "", errors.New("secrets: sealed value does not open")
	}
	return string(plain), nil
}

// OptionalSealer is NewSealer for a key that may be unset: no key means no
// sealer, and so nothing is ever kept or shown. A malformed key is an error,
// so a typo fails at startup rather than silently disabling the feature.
func OptionalSealer(hexKey string) (*Sealer, error) {
	if hexKey == "" {
		return nil, nil
	}
	return NewSealer(hexKey)
}
