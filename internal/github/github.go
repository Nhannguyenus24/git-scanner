// Package github enumerates the repositories a scan target expands into
// (plan.md §9). It is deliberately an interface plus one HTTP implementation so
// the discovery worker is testable without network access and so the REST
// client can later be swapped for GraphQL without touching callers.
//
// It only ever lists repositories for an authorized target — a user, an org, or
// repos granted to an installation (plan.md §0.1). It never crawls all of
// GitHub.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Repo is the subset of a GitHub repository the scanner needs.
type Repo struct {
	Owner         string
	Name          string
	CloneURL      string
	DefaultBranch string
	IsFork        bool
	IsPrivate     bool
	Stars         int
	// CreatedAt is when the repository was created on GitHub, which is not the
	// same as when this system first discovered it.
	CreatedAt time.Time
}

// Page is one page of repositories plus the cursor to fetch the next one.
// NextCursor is empty when the listing is exhausted, which is how the discovery
// worker knows the target is fully expanded.
type Page struct {
	Repos      []Repo
	NextCursor string
}

// Lister enumerates repositories for a target, one page at a time. cursor is
// opaque to the caller and is what pause/resume checkpoints (plan.md §8); an
// empty cursor starts at the beginning.
type Lister interface {
	ListUserRepos(ctx context.Context, user, cursor string) (Page, error)
	ListOrgRepos(ctx context.Context, org, cursor string) (Page, error)
}

// Client is the REST-backed Lister.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	perPage    int
}

// Option configures the client.
type Option func(*Client)

// WithBaseURL points the client at a different API host (GitHub Enterprise, or
// a test server).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient sets the HTTP client, e.g. one with a tighter timeout.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// New builds a client. An empty token still works for public repositories but
// at the much lower unauthenticated rate limit.
func New(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		token:      token,
		perPage:    100,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ListUserRepos lists a user's repositories.
func (c *Client) ListUserRepos(ctx context.Context, user, cursor string) (Page, error) {
	return c.listRepos(ctx, "/users/"+user+"/repos", cursor)
}

// ListOrgRepos lists an organization's repositories.
func (c *Client) ListOrgRepos(ctx context.Context, org, cursor string) (Page, error) {
	return c.listRepos(ctx, "/orgs/"+org+"/repos", cursor)
}

// apiRepo mirrors the fields of the REST repository object we read.
type apiRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Fork          bool   `json:"fork"`
	Private       bool   `json:"private"`
	Stargazers    int       `json:"stargazers_count"`
	CreatedAt     time.Time `json:"created_at"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// listRepos fetches one page. The cursor is the page number, so resuming a
// paused discovery continues from the exact page it stopped on.
func (c *Client) listRepos(ctx context.Context, path, cursor string) (Page, error) {
	page := 1
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil {
			return Page{}, fmt.Errorf("github: bad cursor %q: %w", cursor, err)
		}
		page = n
	}

	url := fmt.Sprintf("%s%s?per_page=%d&page=%d&type=owner&sort=updated",
		c.baseURL, path, c.perPage, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Page{}, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("github: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp); err != nil {
		return Page{}, err
	}

	var raw []apiRepo
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Page{}, fmt.Errorf("github: decode: %w", err)
	}

	out := Page{Repos: make([]Repo, 0, len(raw))}
	for _, r := range raw {
		owner := r.Owner.Login
		if owner == "" && r.FullName != "" {
			owner, _, _ = splitFullName(r.FullName)
		}
		out.Repos = append(out.Repos, Repo{
			Owner: owner, Name: r.Name, CloneURL: r.CloneURL,
			DefaultBranch: r.DefaultBranch, IsFork: r.Fork,
			IsPrivate: r.Private, Stars: r.Stargazers, CreatedAt: r.CreatedAt,
		})
	}

	// A full page means there is probably another; a short page ends the walk.
	if len(raw) == c.perPage {
		out.NextCursor = strconv.Itoa(page + 1)
	}
	return out, nil
}

func splitFullName(full string) (owner, name string, ok bool) {
	for i := 0; i < len(full); i++ {
		if full[i] == '/' {
			return full[:i], full[i+1:], true
		}
	}
	return "", "", false
}

// ParseRepoURL extracts owner and name from a github.com clone URL. It is the
// one place that knows the URL shape, shared by the API and the discovery
// worker's allowlist path.
func ParseRepoURL(raw string) (owner, name string, err error) {
	trimmed := raw
	if len(trimmed) > 4 && trimmed[len(trimmed)-4:] == ".git" {
		trimmed = trimmed[:len(trimmed)-4]
	}
	const host = "github.com/"
	i := indexOf(trimmed, host)
	if i < 0 {
		return "", "", fmt.Errorf("github: only github.com repositories are supported: %q", raw)
	}
	rest := trimmed[i+len(host):]
	for len(rest) > 0 && rest[len(rest)-1] == '/' {
		rest = rest[:len(rest)-1]
	}
	owner, name, ok := splitFullName(rest)
	if !ok || owner == "" || name == "" || indexOf(name, "/") >= 0 {
		return "", "", fmt.Errorf("github: URL must be https://github.com/<owner>/<repo>: %q", raw)
	}
	return owner, name, nil
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
