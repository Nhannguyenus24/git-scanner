package scan

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// BlobIndex remembers which blobs of a repository have been scanned.
//
// This is the incremental-rescan optimisation from plan.md §4.1: a repo
// rescanned after a few new commits only pays for the blobs those commits
// introduced.
type BlobIndex struct {
	rdb goredis.Cmdable
	ttl time.Duration
}

// NewBlobIndex returns an index whose per-repository sets expire after ttl, so
// a repository nobody rescans stops costing memory.
func NewBlobIndex(rdb goredis.Cmdable, ttl time.Duration) *BlobIndex {
	return &BlobIndex{rdb: rdb, ttl: ttl}
}

// Unseen filters oids down to those not scanned before and records the new
// ones, in two round trips per batch.
//
// When recording fails it still returns the unseen set together with the
// error: those blobs will be rescanned next run, which costs time but cannot
// lose a finding, so callers may log and continue.
func (b *BlobIndex) Unseen(ctx context.Context, repoID string, oids []string) ([]string, error) {
	if len(oids) == 0 {
		return nil, nil
	}

	key := keyBlobs + repoID

	pipe := b.rdb.Pipeline()
	cmds := make([]*goredis.BoolCmd, len(oids))
	for i, oid := range oids {
		cmds[i] = pipe.SIsMember(ctx, key, oid)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("scan: blob index check: %w", err)
	}

	fresh := make([]string, 0, len(oids))
	toAdd := make([]any, 0, len(oids))
	for i, cmd := range cmds {
		if !cmd.Val() {
			fresh = append(fresh, oids[i])
			toAdd = append(toAdd, oids[i])
		}
	}
	if len(toAdd) == 0 {
		return fresh, nil
	}

	add := b.rdb.Pipeline()
	add.SAdd(ctx, key, toAdd...)
	add.Expire(ctx, key, b.ttl)
	if _, err := add.Exec(ctx); err != nil {
		return fresh, fmt.Errorf("scan: blob index record: %w", err)
	}
	return fresh, nil
}
