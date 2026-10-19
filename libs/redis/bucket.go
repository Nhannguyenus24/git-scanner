package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

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
