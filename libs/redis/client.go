// Package redis wraps go-redis with the few primitives this system needs.
//
// It is deliberately thin (plan.md §3.4): connection setup and the distributed
// token bucket, which must behave identically in every service. Domain uses of
// Redis (blob dedupe, scan cursors, pause flags) live with the domain in
// internal/, built on Client.Raw. It does not re-export the driver.
//
// Files:
//
//	client.go  connection setup and readiness
//	bucket.go  distributed token bucket (mail cap, GitHub token pool)
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client is the shared Redis handle.
type Client struct {
	rdb *goredis.Client
}

// New connects using a redis:// URL and verifies the connection eagerly, so a
// bad address fails at startup rather than on first use.
func New(ctx context.Context, url string) (*Client, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}

	// Without this go-redis ignores context deadlines and falls back to its own
	// socket timeouts, so callers' deadlines (readiness, drain) would not hold.
	opt.ContextTimeoutEnabled = true

	rdb := goredis.NewClient(opt)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Raw exposes the driver for operations this package does not cover.
func (c *Client) Raw() *goredis.Client { return c.rdb }

// Ping is the readiness check.
func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Close releases the connection pool.
func (c *Client) Close() error { return c.rdb.Close() }
