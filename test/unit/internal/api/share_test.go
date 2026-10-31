package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/share"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

type fakeShares struct {
	authorized map[entity.ID]bool
	links      map[string]entity.ShareLink // by token hash
	findings   map[entity.ID][]entity.Finding
}

func newFakeShares() *fakeShares {
	return &fakeShares{authorized: map[entity.ID]bool{}, links: map[string]entity.ShareLink{}, findings: map[entity.ID][]entity.Finding{}}
}

func (f *fakeShares) RepositoryAuthorized(_ context.Context, id entity.ID) (bool, error) {
	return f.authorized[id], nil
}
func (f *fakeShares) CreateShareLink(_ context.Context, l entity.ShareLink, hash []byte) error {
	if err := l.Validate(); err != nil {
		return err
	}
	f.links[string(hash)] = l
	return nil
}
func (f *fakeShares) ListShareLinks(_ context.Context, repo entity.ID) ([]entity.ShareLink, error) {
	var out []entity.ShareLink
	for _, l := range f.links {
		if l.RepositoryID == repo {
			out = append(out, l)
		}
	}
	return out, nil
}
func (f *fakeShares) RevokeShareLink(_ context.Context, id entity.ID) error {
	for h, l := range f.links {
		if l.ID == id {
			now := time.Now()
			l.RevokedAt = &now
			f.links[h] = l
			return nil
		}
	}
	return store.ErrNotFound
}
func (f *fakeShares) OpenShareLink(_ context.Context, hash []byte, now time.Time) (entity.ShareLink, error) {
	l, ok := f.links[string(hash)]
	if !ok || l.State(now) != entity.ShareActive {
		return entity.ShareLink{}, store.ErrNotFound
	}
	l.Views++
	f.links[string(hash)] = l
	return l, nil
}
func (f *fakeShares) ListFindingsByRepository(_ context.Context, repo entity.ID) ([]entity.Finding, error) {
	return f.findings[repo], nil
}

func shareServer(t *testing.T) (*fakeShares, entity.ID, func(method, path, body string) (int, http.Header, string)) {
	t.Helper()
	srv, st, _ := newServer(t)
	sh := newFakeShares()
	srv.WithShares(sh)
	repoID := id.New()
	st.repos[repoID] = entity.Repository{ID: repoID, Owner: "acme", Name: "billing"}
	sh.authorized[repoID] = true
	sh.findings[repoID] = []entity.Finding{{
		RuleID: "stripe-secret-key", ProviderSlug: "stripe", Severity: entity.SeverityCritical, Status: entity.FindingOpen,
		FilePath: ".env", LineNumber: 2, MaskedPreview: "sk_l…9c1d", CommitSHA: "abc123", AuthorEmail: "dev@example.com",
		Metadata: map[string]string{},
	}}
	return sh, repoID, func(method, path, body string) (int, http.Header, string) {
		rec := do(srv, method, path, body)
		return rec.Code, rec.Header(), rec.Body.String()
	}
}

func TestShareLinkLifecycle(t *testing.T) {
	_, repoID, call := shareServer(t)
	base := "/v1/repositories/" + repoID.String() + "/shares"

	code, _, body := call(http.MethodPost, base, `{"detail":"full","expires_in_days":3}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var created struct {
		ID, Token, Detail, State string
		ExpiresAt                time.Time `json:"expires_at"`
		CreatedAt                time.Time `json:"created_at"`
	}
	_ = json.Unmarshal([]byte(body), &created)
	if created.Token == "" || created.Detail != "full" || created.State != "active" ||
		created.ExpiresAt.Sub(created.CreatedAt) != 72*time.Hour {
		t.Fatalf("created = %+v", created)
	}

	// The list never shows the token again.
	code, _, body = call(http.MethodGet, base, "")
	if code != http.StatusOK || strings.Contains(body, created.Token) || !strings.Contains(body, created.ID) {
		t.Errorf("list: %d %s", code, body)
	}

	code, hdr, body := call(http.MethodGet, "/v1/shared/"+created.Token, "")
	if code != http.StatusOK || !strings.Contains(body, `"location":".env:2"`) || strings.Contains(body, "dev@example.com") {
		t.Errorf("open: %d %s", code, body)
	}
	if hdr.Get("Cache-Control") != "no-store" || hdr.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("headers = %v", hdr)
	}

	code, hdr, body = call(http.MethodGet, "/v1/shared/"+created.Token+"/report?format=md", "")
	if code != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Disposition"), "attachment;") ||
		!strings.Contains(hdr.Get("Content-Disposition"), "secret-scan-acme-billing-") || !strings.HasPrefix(body, "# Secret scan report: acme/billing") {
		t.Errorf("markdown: %d %v %.60s", code, hdr, body)
	}
	code, hdr, _ = call(http.MethodGet, "/v1/shared/"+created.Token+"/report?format=html", "")
	if code != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("html: %d %v", code, hdr)
	}
	if code, _, _ = call(http.MethodGet, "/v1/shared/"+created.Token+"/report?format=pdf", ""); code != http.StatusBadRequest {
		t.Errorf("unknown format: %d, want 400", code)
	}

	if code, _, _ = call(http.MethodDelete, "/v1/shares/"+created.ID, ""); code != http.StatusNoContent {
		t.Errorf("revoke: %d", code)
	}
	if code, _, _ = call(http.MethodGet, "/v1/shared/"+created.Token, ""); code != http.StatusNotFound {
		t.Errorf("revoked link opened: %d", code)
	}
	if code, _, _ = call(http.MethodDelete, "/v1/shares/"+id.New().String(), ""); code != http.StatusNotFound {
		t.Errorf("revoke unknown: %d, want 404", code)
	}
}

// Summary is the default: a link made without saying otherwise shows no locations.
func TestShareDefaultsToSummary(t *testing.T) {
	_, repoID, call := shareServer(t)
	_, _, body := call(http.MethodPost, "/v1/repositories/"+repoID.String()+"/shares", `{}`)
	var c struct{ Token, Detail string }
	_ = json.Unmarshal([]byte(body), &c)
	if c.Detail != "summary" {
		t.Fatalf("detail = %q, want summary", c.Detail)
	}
	_, _, body = call(http.MethodGet, "/v1/shared/"+c.Token, "")
	if strings.Contains(body, ".env") || !strings.Contains(body, `"total":1`) {
		t.Errorf("summary report = %s", body)
	}
}

func TestShareRefusals(t *testing.T) {
	sh, repoID, call := shareServer(t)
	base := "/v1/repositories/" + repoID.String() + "/shares"
	for name, c := range map[string]struct {
		body string
		want int
	}{
		"unknown detail": {`{"detail":"everything"}`, http.StatusBadRequest},
		"too long":       {`{"expires_in_days":31}`, http.StatusBadRequest},
		"negative":       {`{"expires_in_days":-1}`, http.StatusBadRequest},
		"unknown field":  {`{"public":true}`, http.StatusBadRequest},
	} {
		if code, _, body := call(http.MethodPost, base, c.body); code != c.want {
			t.Errorf("%s: %d %s, want %d", name, code, body, c.want)
		}
	}
	if code, _, _ := call(http.MethodPost, "/v1/repositories/"+id.New().String()+"/shares", `{}`); code != http.StatusNotFound {
		t.Errorf("unknown repository: %d, want 404", code)
	}
	// Reached only by a random sample: sharing it would be disclosure.
	sh.authorized[repoID] = false
	if code, _, body := call(http.MethodPost, base, `{}`); code != http.StatusConflict || !strings.Contains(body, "random sample") {
		t.Errorf("random-only repository: %d %s, want 409", code, body)
	}
}

// Malformed, unknown and expired tokens get the same answer.
func TestSharedLinkFailuresLookAlike(t *testing.T) {
	sh, repoID, call := shareServer(t)
	expiredTok, expiredHash, _ := share.NewToken()
	past := time.Now().Add(-48 * time.Hour)
	sh.links[string(expiredHash)] = entity.ShareLink{ID: id.New(), RepositoryID: repoID, Detail: entity.ShareFull,
		CreatedAt: past, ExpiresAt: past.Add(time.Hour)}
	unknown, _, _ := share.NewToken()

	var bodies []string
	for _, tok := range []string{"not-a-token", unknown, expiredTok} {
		code, _, body := call(http.MethodGet, "/v1/shared/"+tok, "")
		if code != http.StatusNotFound {
			t.Errorf("%.10s…: %d, want 404", tok, code)
		}
		bodies = append(bodies, body)
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("answers differ: %q", bodies)
	}
}
