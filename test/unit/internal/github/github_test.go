package github_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/libs/entity"
)

const fullRepo = `{
	"name": "app", "full_name": "acme/app", "clone_url": "https://github.com/acme/app.git",
	"default_branch": "main", "fork": false, "private": false, "stargazers_count": 7,
	"created_at": "2020-01-02T03:04:05Z", "pushed_at": "2023-06-07T08:09:10Z",
	"language": "Go", "topics": ["api", "payments"], "archived": true, "size": 2048,
	"owner": {"login": "acme", "type": "Organization"}
}`

func server(t *testing.T, h http.HandlerFunc) *github.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return github.New("", github.WithBaseURL(srv.URL))
}

func TestGetRepoReadsAttributes(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/app" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(fullRepo))
	})
	got, err := c.GetRepo(context.Background(), "acme", "app")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Details || got.Language != "Go" || got.OwnerType != "Organization" ||
		!got.Archived || got.SizeKB != 2048 || len(got.Topics) != 2 || got.PushedAt.IsZero() {
		t.Errorf("attributes not read: %+v", got)
	}

	var repo entity.Repository
	fetched := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got.ApplyAttributes(&repo, fetched)
	if repo.Language != "Go" || repo.OwnerType != "Organization" || !repo.Archived ||
		repo.PushedAt == nil || repo.MetadataFetchedAt == nil || !repo.MetadataFetchedAt.Equal(fetched) {
		t.Errorf("attributes not applied: %+v", repo)
	}
}

// The random sample's /repositories object has none of the attributes. Its
// zero values must not be applied, or every sampled repository would be
// recorded as an unarchived, language-less, never-pushed one.
func TestRandomSampleCarriesNoAttributes(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name": "x", "full_name": "someone/x", "fork": false, "owner": {"login": "someone"}}]`))
	})
	page, err := c.ListRandomRepos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Repos) != 1 || page.Repos[0].Details {
		t.Fatalf("random page = %+v, want one repo without details", page.Repos)
	}
	repo := entity.Repository{Language: "kept"}
	page.Repos[0].ApplyAttributes(&repo, time.Now())
	if repo.Language != "kept" || repo.MetadataFetchedAt != nil {
		t.Errorf("minimal object overwrote attributes: %+v", repo)
	}
}

func TestListingCarriesAttributes(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[" + fullRepo + "]"))
	})
	page, err := c.ListOrgRepos(context.Background(), "acme", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Repos) != 1 || !page.Repos[0].Details || page.Repos[0].Language != "Go" {
		t.Errorf("listing = %+v, want full attributes", page.Repos)
	}
}

func TestGetRepoReportsRateLimit(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := c.GetRepo(context.Background(), "acme", "app")
	var rl *github.RateLimitError
	if !errors.As(err, &rl) || rl.RetryAfter != 30*time.Second {
		t.Errorf("err = %v, want RateLimitError retry 30s", err)
	}
}
