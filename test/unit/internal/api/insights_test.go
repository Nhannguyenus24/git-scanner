package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/store"
)

type fakeInsights struct {
	minRepos, limit int
	keys            []store.SharedKey
}

func (f *fakeInsights) SharedKeys(_ context.Context, minRepos int, page store.Page) ([]store.SharedKey, int, error) {
	f.minRepos, f.limit = minRepos, page.Limit
	return f.keys, 42, nil
}

func (f *fakeInsights) LeakCalendar(context.Context) ([]store.LeakDay, error) {
	return []store.LeakDay{{Day: "2024-02-29", Findings: 3, Critical: 1}}, nil
}

func TestLeakCalendarRoute(t *testing.T) {
	srv, _, _ := newServer(t)
	srv.WithInsights(&fakeInsights{})
	rec := do(srv, http.MethodGet, "/v1/analytics/calendar", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Days []struct {
			Day                string
			Findings, Critical int
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Days) != 1 || got.Days[0].Day != "2024-02-29" || got.Days[0].Findings != 3 || got.Days[0].Critical != 1 {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestSharedKeysRoute(t *testing.T) {
	srv, _, _ := newServer(t)
	ins := &fakeInsights{keys: []store.SharedKey{{
		KeyID: "0123456789abcdef", Provider: "stripe", RuleID: "stripe-secret-key", Severity: "critical",
		MaskedPreview: "sk_l…9c1d",
		Repos: []store.SharedKeyRepo{
			{FindingID: "f1", RepoID: "r1", Owner: "acme", Name: "billing", Status: "open", AtHead: true},
			{FindingID: "f2", RepoID: "r2", Owner: "other", Name: "fork", Status: "rotated"},
		},
	}}}
	srv.WithInsights(ins)

	rec := do(srv, http.MethodGet, "/v1/keys/shared?min_repos=3&limit=20", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if ins.minRepos != 3 || ins.limit != 20 {
		t.Errorf("query params not passed on: min_repos=%d limit=%d", ins.minRepos, ins.limit)
	}
	var got struct {
		Total int `json:"total"`
		Keys  []struct {
			KeyID string `json:"key_id"`
			Repos []struct {
				Owner  string `json:"owner"`
				AtHead bool   `json:"reachable_from_head"`
			} `json:"repositories"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 42 {
		t.Errorf("total %d, want the store's 42", got.Total)
	}
	if len(got.Keys) != 1 || len(got.Keys[0].Repos) != 2 || !got.Keys[0].Repos[0].AtHead {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	// The view describes a key by preview and fingerprint prefix only.
	for _, field := range []string{"fingerprint", "secret", "sealed"} {
		if strings.Contains(rec.Body.String(), `"`+field+`"`) {
			t.Errorf("response carries a %q field", field)
		}
	}
}

func TestSharedKeysRouteAbsentWithoutInsights(t *testing.T) {
	srv, _, _ := newServer(t)
	if rec := do(srv, http.MethodGet, "/v1/keys/shared", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 when insights are not wired", rec.Code)
	}
}
