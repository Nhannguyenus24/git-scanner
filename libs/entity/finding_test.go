package entity

import "testing"

// The masked-preview guard exists to catch a masking bug writing plaintext into
// a column that gets displayed, logged and emailed.
func TestFindingRejectsUnmaskedPreview(t *testing.T) {
	base := Finding{
		RuleID:      "stripe-secret-key",
		Fingerprint: "deadbeef",
		BlobOID:     "abc123",
	}

	unmasked := base
	unmasked.MaskedPreview = "sk_live_51H8xKpLmNqRsTuVwXyZ"
	if err := unmasked.Validate(); err == nil {
		t.Error("a preview with no ellipsis must be rejected as possibly unmasked")
	}

	masked := base
	masked.MaskedPreview = "sk_live_51H8…XyZ"
	if err := masked.Validate(); err != nil {
		t.Errorf("a correctly masked preview was rejected: %v", err)
	}

	empty := base
	empty.MaskedPreview = ""
	if err := empty.Validate(); err != nil {
		t.Errorf("an empty preview is not a leak and must pass: %v", err)
	}
}

func TestFindingRequiresFingerprint(t *testing.T) {
	f := Finding{RuleID: "r", BlobOID: "b"}
	if err := f.Validate(); err == nil {
		t.Error("fingerprint is the dedupe and idempotency key; it cannot be empty")
	}
}

func TestSeverityRoundTrip(t *testing.T) {
	for _, s := range []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical} {
		parsed, err := ParseSeverity(s.String())
		if err != nil {
			t.Fatalf("ParseSeverity(%q): %v", s.String(), err)
		}
		if parsed != s {
			t.Errorf("round trip changed %v into %v", s, parsed)
		}
	}

	if _, err := ParseSeverity("catastrophic"); err == nil {
		t.Error("unknown severity must error rather than default silently")
	}
}

// Ordering is relied on when picking the worst finding in a repository.
func TestSeverityOrdering(t *testing.T) {
	ordered := []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Errorf("severity constants must be ordered ascending: %v >= %v", ordered[i-1], ordered[i])
		}
	}
}
