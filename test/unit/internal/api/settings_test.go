package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/api"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/internal/store"
)

const testSealKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestUnmaskOwnersAddAndRemove(t *testing.T) {
	srv, st, _ := newServer(t)

	if rec := do(srv, http.MethodPut, "/v1/settings/unmask-owners/Nhannguyenus24", ""); rec.Code != http.StatusOK {
		t.Fatalf("add = %d, body %s", rec.Code, rec.Body)
	}
	if len(st.owners) != 1 || st.owners[0] != "nhannguyenus24" {
		t.Fatalf("owners = %v, want [nhannguyenus24]", st.owners)
	}
	if rec := do(srv, http.MethodPut, "/v1/settings/unmask-owners/not%20a%20login", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad login = %d, want 400", rec.Code)
	}
	if rec := do(srv, http.MethodDelete, "/v1/settings/unmask-owners/NHANNGUYENUS24", ""); rec.Code != http.StatusOK {
		t.Fatalf("remove = %d", rec.Code)
	}
	if len(st.owners) != 0 {
		t.Errorf("owners after remove = %v", st.owners)
	}
}

func TestPutSettingsMasking(t *testing.T) {
	srv, st, _ := newServer(t)
	rec := do(srv, http.MethodPut, "/v1/settings", `{"masking_enabled":false}`)
	if rec.Code != http.StatusOK || !st.maskOff {
		t.Fatalf("status %d, maskOff %v", rec.Code, st.maskOff)
	}
	// Only the named switch changes.
	if st.notifyOff {
		t.Error("turning masking off also turned notifications off")
	}
	if !strings.Contains(rec.Body.String(), `"sealing_available":false`) {
		t.Errorf("body = %s, want sealing_available false without a sealer", rec.Body)
	}
}

// A credential is shown only when all three hold: a sealer is configured,
// masking is off, and the store returned a sealed value (which it does only
// for an unmask-listed owner).
func TestSearchRevealsOnlyWithMaskingOff(t *testing.T) {
	sealer, err := secrets.NewSealer(testSealKey)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal("sk_test_1234567890", store.SealedAAD("r1", "fp1"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		maskOff    bool
		withSealer bool
		want       string
	}{
		{"masking on", false, true, ""},
		{"no sealer", true, false, ""},
		{"masking off", true, true, "sk_test_1234567890"},
	} {
		st := newFakeStore()
		st.maskOff = tc.maskOff
		cat := &fakeCatalog{sealed: sealed}
		srv := api.NewServer(st, &fakeQueue{}, &fakeControl{}, slog.New(slog.NewTextHandler(io.Discard, nil))).WithCatalog(cat)
		if tc.withSealer {
			srv.WithSealer(sealer)
		}

		rec := do(srv, http.MethodGet, "/v1/findings", "")
		var resp struct {
			Findings []struct {
				Secret string `json:"secret"`
				Masked string `json:"masked_preview"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := resp.Findings[0].Secret; got != tc.want {
			t.Errorf("%s: secret = %q, want %q", tc.name, got, tc.want)
		}
		if resp.Findings[0].Masked == "" {
			t.Errorf("%s: masked preview missing", tc.name)
		}
	}
}
