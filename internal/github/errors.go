package github

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ErrNotFound is returned for a target that does not exist (404). The discovery
// worker treats it as permanent and does not retry.
var ErrNotFound = errors.New("github: target not found")

// RateLimitError reports that GitHub asked the caller to back off. RetryAfter
// is how long to wait before the next request (plan.md §9).
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("github: rate limited, retry after %s", e.RetryAfter)
}

// checkStatus turns a non-2xx response into a typed error. Rate limiting is
// distinguished from other failures so the worker can requeue with the broker's
// backoff instead of dead-lettering a target that is merely throttled.
func checkStatus(resp *http.Response) error {
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		// Primary limit: Retry-After. Secondary limit: x-ratelimit-reset is an
		// absolute unix time. Fall back to a minute when neither is present.
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				return &RateLimitError{RetryAfter: time.Duration(secs) * time.Second}
			}
		}
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
				if d := time.Until(time.Unix(ts, 0)); d > 0 {
					return &RateLimitError{RetryAfter: d}
				}
			}
		}
		return &RateLimitError{RetryAfter: time.Minute}
	default:
		return fmt.Errorf("github: unexpected status %d", resp.StatusCode)
	}
}
