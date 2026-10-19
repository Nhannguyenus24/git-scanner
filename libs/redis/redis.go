// Package redis wraps go-redis with the few primitives this system needs.
//
// It is deliberately thin (plan.md §3.4): connection setup, plus the four
// operations that must behave identically in every service. It does not re-export
// the driver. Code needing something else should use Client.Raw and talk to
// go-redis directly.
package redis

import (
	"context"
	"errors"
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

// ErrRateLimited is returned when a token bucket has no tokens left.
var ErrRateLimited = errors.New("rate limited")

// takeTokenScript is a token bucket in Lua so the read, refill and decrement are
// one atomic operation.
//
// This is what makes the mail rate cap survive horizontal scaling (plan.md
// §13.5): a per-process limiter multiplies the send rate by the replica count,
// which is exactly the failure the cap exists to prevent. The same script backs
// the per-token GitHub API buckets (plan.md §9).
//
// KEYS[1] bucket key
// ARGV[1] capacity, ARGV[2] refill tokens per second,
// ARGV[3] now (unix seconds, float), ARGV[4] requested tokens
const takeTokenScript = `
local key      = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate     = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local want     = tonumber(ARGV[4])

local state   = redis.call('HMGET', key, 'tokens', 'ts')
local tokens  = tonumber(state[1])
local ts      = tonumber(state[2])

if tokens == nil then
  tokens = capacity
  ts = now
end

tokens = math.min(capacity, tokens + (now - ts) * rate)

if tokens < want then
  -- seconds until enough tokens accumulate, for the caller's backoff
  local wait = (want - tokens) / rate
  redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
  redis.call('EXPIRE', key, math.ceil(capacity / rate) * 2)
  return {0, tostring(wait)}
end

tokens = tokens - want
redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', key, math.ceil(capacity / rate) * 2)
return {1, "0"}
`

var takeToken = goredis.NewScript(takeTokenScript)

// Bucket is a distributed token bucket.
type Bucket struct {
	client   *Client
	key      string
	capacity float64
	rate     float64 // tokens per second
}

// NewBucket returns a bucket shared by every process using the same key.
func (c *Client) NewBucket(key string, capacity float64, per time.Duration) *Bucket {
	return &Bucket{
		client:   c,
		key:      "bucket:" + key,
		capacity: capacity,
		rate:     capacity / per.Seconds(),
	}
}

// Take consumes n tokens. It returns ErrRateLimited together with how long the
// caller should wait before retrying.
func (b *Bucket) Take(ctx context.Context, n float64) (retryAfter time.Duration, err error) {
	now := float64(time.Now().UnixNano()) / float64(time.Second)

	res, err := takeToken.Run(ctx, b.client.rdb, []string{b.key},
		b.capacity, b.rate, now, n).Slice()
	if err != nil {
		return 0, fmt.Errorf("redis: token bucket %s: %w", b.key, err)
	}
	if len(res) != 2 {
		return 0, fmt.Errorf("redis: token bucket %s: unexpected reply", b.key)
	}

	allowed, _ := res[0].(int64)
	if allowed == 1 {
		return 0, nil
	}

	var wait float64
	if s, ok := res[1].(string); ok {
		_, _ = fmt.Sscanf(s, "%f", &wait)
	}
	return time.Duration(wait * float64(time.Second)), ErrRateLimited
}

// SeenBlobs filters blob OIDs down to those not scanned before and records the
// new ones, in one round trip per batch.
//
// This is the incremental-rescan optimisation from plan.md §4.1: a repo rescanned
// after a few new commits only pays for the blobs those commits introduced.
func (c *Client) SeenBlobs(ctx context.Context, repoID string, oids []string, ttl time.Duration) ([]string, error) {
	if len(oids) == 0 {
		return nil, nil
	}

	key := "blobs:" + repoID
	pipe := c.rdb.Pipeline()

	cmds := make([]*goredis.BoolCmd, len(oids))
	for i, oid := range oids {
		cmds[i] = pipe.SIsMember(ctx, key, oid)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis: blob dedupe check: %w", err)
	}

	fresh := make([]string, 0, len(oids))
	toAdd := make([]any, 0, len(oids))
	for i, cmd := range cmds {
		if !cmd.Val() {
			fresh = append(fresh, oids[i])
			toAdd = append(toAdd, oids[i])
		}
	}

	if len(toAdd) > 0 {
		addPipe := c.rdb.Pipeline()
		addPipe.SAdd(ctx, key, toAdd...)
		addPipe.Expire(ctx, key, ttl)
		if _, err := addPipe.Exec(ctx); err != nil {
			// Recording failed, so these blobs will be rescanned next run. That
			// costs time but cannot lose a finding, so it is not fatal.
			return fresh, fmt.Errorf("redis: blob dedupe record: %w", err)
		}
	}

	return fresh, nil
}

// SetCursor stores a resumable position for a scan job.
func (c *Client) SetCursor(ctx context.Context, jobID, cursor string, ttl time.Duration) error {
	return c.rdb.Set(ctx, "cursor:"+jobID, cursor, ttl).Err()
}

// GetCursor reads a stored position. It returns "" when none exists.
func (c *Client) GetCursor(ctx context.Context, jobID string) (string, error) {
	v, err := c.rdb.Get(ctx, "cursor:"+jobID).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis: get cursor: %w", err)
	}
	return v, nil
}

// PauseRequested reports whether an operator asked this job to pause. Workers
// check it between batches (plan.md §8).
func (c *Client) PauseRequested(ctx context.Context, jobID string) (bool, error) {
	n, err := c.rdb.Exists(ctx, "pause:"+jobID).Result()
	if err != nil {
		return false, fmt.Errorf("redis: pause flag: %w", err)
	}
	return n > 0, nil
}

// RequestPause sets the pause flag for a job.
func (c *Client) RequestPause(ctx context.Context, jobID string) error {
	return c.rdb.Set(ctx, "pause:"+jobID, "1", 24*time.Hour).Err()
}

// ClearPause removes the pause flag.
func (c *Client) ClearPause(ctx context.Context, jobID string) error {
	return c.rdb.Del(ctx, "pause:"+jobID).Err()
}
