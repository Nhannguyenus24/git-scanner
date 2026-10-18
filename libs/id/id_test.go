package id

import (
	"testing"
	"time"
)

func TestNewFormat(t *testing.T) {
	v := New().String()

	if len(v) != 36 {
		t.Fatalf("length = %d, want 36: %q", len(v), v)
	}
	for _, pos := range []int{8, 13, 18, 23} {
		if v[pos] != '-' {
			t.Errorf("expected '-' at %d in %q", pos, v)
		}
	}
	if v[14] != '7' {
		t.Errorf("version nibble = %q, want '7' (UUIDv7): %s", v[14], v)
	}
	if c := v[19]; c != '8' && c != '9' && c != 'a' && c != 'b' {
		t.Errorf("variant nibble = %q, want one of 8,9,a,b: %s", c, v)
	}
}

func TestUniqueness(t *testing.T) {
	const n = 10000
	seen := make(map[string]struct{}, n)
	for range n {
		v := New().String()
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate id generated: %s", v)
		}
		seen[v] = struct{}{}
	}
}

// The whole reason for v7 over v4 is that ids sort by creation time, which keeps
// the findings primary-key index append-mostly instead of scattering writes.
func TestTimeOrdered(t *testing.T) {
	first := New().String()
	time.Sleep(2 * time.Millisecond)
	second := New().String()

	if first >= second {
		t.Errorf("ids must sort by creation time: %s >= %s", first, second)
	}
}
