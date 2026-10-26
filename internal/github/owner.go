package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// OwnerEmail finds an address for the account that owns a repository, so a
// disclosure reaches the person who can rotate the credential rather than
// whichever contributor happened to commit it.
//
// It tries the owner's public profile email first, which is the address the
// owner chose to publish. Failing that, it reads the author email of the
// owner's own commits in that repository, matched by GitHub login rather than
// by name, so a teammate's commit is never mistaken for the owner's. It
// returns "" when the owner publishes no reachable address; GitHub's noreply
// privacy addresses are not inboxes and are skipped.
func (c *Client) OwnerEmail(ctx context.Context, owner, repo string) (string, error) {
	var profile struct {
		Email string `json:"email"`
	}
	if err := c.getJSON(ctx, "/users/"+url.PathEscape(owner), &profile); err != nil {
		return "", err
	}
	if usableEmail(profile.Email) {
		return strings.ToLower(profile.Email), nil
	}

	var commits []struct {
		Commit struct {
			Author struct {
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}
	path := fmt.Sprintf("/repos/%s/%s/commits?author=%s&per_page=30",
		url.PathEscape(owner), url.PathEscape(repo), url.QueryEscape(owner))
	if err := c.getJSON(ctx, path, &commits); err != nil {
		return "", err
	}
	for _, cm := range commits {
		if e := cm.Commit.Author.Email; usableEmail(e) {
			return strings.ToLower(e), nil
		}
	}
	return "", nil
}

func usableEmail(e string) bool {
	e = strings.ToLower(strings.TrimSpace(e))
	return e != "" && strings.Contains(e, "@") && !strings.Contains(e, "noreply")
}

// getJSON issues one authenticated GET and decodes the body.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: decode: %w", err)
	}
	return nil
}
