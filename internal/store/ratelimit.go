package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrRateLimited is returned when a token bucket has no tokens left.
var ErrRateLimited = errors.New("rate limited")

// Bucket is a token bucket shared by every process using the same key.
//
// This is what makes the mail rate cap survive horizontal scaling (plan.md
// §13.5): a per-process limiter multiplies the send rate by the replica count,
// which is exactly the failure the cap exists to prevent. Take holds the
// bucket's row lock across the read, refill and decrement, so concurrent
// replicas serialise on it.
type Bucket struct {
	db       *DB
	key      string
	capacity float64
	rate     float64 // tokens per second
}

// NewBucket returns a bucket holding at most capacity tokens, refilled at
// capacity per period. The row is created on first Take, full.
func (db *DB) NewBucket(key string, capacity float64, per time.Duration) *Bucket {
	return &Bucket{db: db, key: key, capacity: capacity, rate: capacity / per.Seconds()}
}

// Take consumes n tokens. It returns ErrRateLimited together with how long the
// caller should wait before retrying.
func (b *Bucket) Take(ctx context.Context, n float64) (retryAfter time.Duration, err error) {
	err = b.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rate_buckets (key, tokens, updated_at) VALUES ($1, $2, clock_timestamp())
			ON CONFLICT (key) DO NOTHING`, b.key, b.capacity); err != nil {
			return err
		}

		// clock_timestamp, not now(): now() is fixed at transaction start, so a
		// Take that waited on the lock would measure its refill from before the
		// holder's write and could see time run backwards. The database clock
		// rather than the caller's also keeps replica clock skew out of the rate.
		var tokens, elapsed float64
		if err := tx.QueryRow(ctx, `
			SELECT tokens, EXTRACT(EPOCH FROM clock_timestamp() - updated_at)::float8
			FROM rate_buckets WHERE key = $1 FOR UPDATE`, b.key).Scan(&tokens, &elapsed); err != nil {
			return err
		}

		tokens = min(b.capacity, tokens+max(elapsed, 0)*b.rate)
		if tokens >= n {
			tokens -= n
		} else {
			retryAfter = time.Duration((n - tokens) / b.rate * float64(time.Second))
		}

		_, err := tx.Exec(ctx,
			`UPDATE rate_buckets SET tokens = $2, updated_at = clock_timestamp() WHERE key = $1`,
			b.key, tokens)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("store: token bucket %s: %w", b.key, err)
	}
	if retryAfter > 0 {
		return retryAfter, ErrRateLimited
	}
	return 0, nil
}
