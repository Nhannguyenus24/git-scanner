package secrets_test

import (
	"testing"

	"github.com/mservice/gitscanner/internal/secrets"
)

const key = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestSealRoundTripAndBinding(t *testing.T) {
	s, err := secrets.NewSealer(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := s.Seal("AKIAIOSFODNN7EXAMPLE", []byte("repo/fp"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sealed) == "AKIAIOSFODNN7EXAMPLE" || len(sealed) <= len("AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("sealed value is not ciphertext")
	}
	got, err := s.Open(sealed, []byte("repo/fp"))
	if err != nil || got != "AKIAIOSFODNN7EXAMPLE" {
		t.Fatalf("Open = %q, %v", got, err)
	}

	// Bound to its row: moved elsewhere, it does not open.
	if _, err := s.Open(sealed, []byte("repo/other")); err == nil {
		t.Error("opened under the wrong aad")
	}
	// A different key does not open it either.
	other, _ := secrets.NewSealer("ff" + key[2:])
	if _, err := other.Open(sealed, []byte("repo/fp")); err == nil {
		t.Error("opened under the wrong key")
	}
	// Two seals of the same value differ (fresh nonce).
	again, _ := s.Seal("AKIAIOSFODNN7EXAMPLE", []byte("repo/fp"))
	if string(again) == string(sealed) {
		t.Error("nonce reused")
	}
}

func TestOptionalSealer(t *testing.T) {
	if s, err := secrets.OptionalSealer(""); s != nil || err != nil {
		t.Errorf("empty key = %v, %v; want nil, nil", s, err)
	}
	if _, err := secrets.OptionalSealer("not-hex"); err == nil {
		t.Error("malformed key accepted")
	}
}
