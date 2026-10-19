package scan

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// pauseTTL bounds a forgotten pause flag. A job paused longer than this is
// resumed from Postgres state by an operator, not by the flag.
const pauseTTL = 24 * time.Hour

// Control is the shared pause/resume state between the api and the workers
// (plan.md §8). The api sets flags; workers check them between batches and
// persist their cursor before acking.
type Control struct {
	rdb goredis.Cmdable
}

// NewControl returns job control backed by Redis.
func NewControl(rdb goredis.Cmdable) *Control { return &Control{rdb: rdb} }

// RequestPause asks the worker holding jobID to stop at its next batch.
func (c *Control) RequestPause(ctx context.Context, jobID string) error {
	if err := c.rdb.Set(ctx, keyPause+jobID, "1", pauseTTL).Err(); err != nil {
		return fmt.Errorf("scan: request pause: %w", err)
	}
	return nil
}

// ClearPause removes the pause flag so a resumed job is not paused again.
func (c *Control) ClearPause(ctx context.Context, jobID string) error {
	if err := c.rdb.Del(ctx, keyPause+jobID).Err(); err != nil {
		return fmt.Errorf("scan: clear pause: %w", err)
	}
	return nil
}

// PauseRequested reports whether an operator asked jobID to pause.
func (c *Control) PauseRequested(ctx context.Context, jobID string) (bool, error) {
	n, err := c.rdb.Exists(ctx, keyPause+jobID).Result()
	if err != nil {
		return false, fmt.Errorf("scan: pause flag: %w", err)
	}
	return n > 0, nil
}

// SaveCursor stores a resumable position for jobID.
func (c *Control) SaveCursor(ctx context.Context, jobID, cursor string, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, keyCursor+jobID, cursor, ttl).Err(); err != nil {
		return fmt.Errorf("scan: save cursor: %w", err)
	}
	return nil
}

// Cursor reads the stored position. It returns "" when none exists, which
// means start from the beginning.
func (c *Control) Cursor(ctx context.Context, jobID string) (string, error) {
	v, err := c.rdb.Get(ctx, keyCursor+jobID).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("scan: read cursor: %w", err)
	}
	return v, nil
}
