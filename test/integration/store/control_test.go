//go:build integration

// The pause flag and the shared token bucket, which used to live in Redis.
//
// Run with: make test-integration (needs POSTGRES_URL).
package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

func openDB(t *testing.T) *store.DB {
	t.Helper()
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		t.Skip("POSTGRES_URL not set")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPauseFlag(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)

	target, err := db.CreateTarget(ctx, entity.Target{Kind: entity.TargetUser, Value: "octocat"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateScanJob(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DeleteScanJob(context.Background(), job.ID) })

	if paused, _ := db.PauseRequested(ctx, job.ID); paused {
		t.Fatal("new job reports paused")
	}
	if err := db.RequestPause(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if paused, _ := db.PauseRequested(ctx, job.ID); !paused {
		t.Fatal("pause flag not visible after RequestPause")
	}
	if err := db.ClearPause(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if paused, _ := db.PauseRequested(ctx, job.ID); paused {
		t.Fatal("pause flag still set after ClearPause")
	}

	if err := db.RequestPause(ctx, id.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("RequestPause on a missing job = %v, want ErrNotFound", err)
	}
}

// The cap must hold across replicas, so the takes run concurrently and on
// separate Bucket values sharing one key.
func TestBucketCapsConcurrentTakes(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	key := "test:" + id.New().String()
	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), `DELETE FROM rate_buckets WHERE key = $1`, key)
	})

	const capacity, attempts = 5, 20
	var (
		mu      sync.Mutex
		granted int
		wg      sync.WaitGroup
	)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retryAfter, err := db.NewBucket(key, capacity, time.Hour).Take(ctx, 1)
			switch {
			case err == nil:
				mu.Lock()
				granted++
				mu.Unlock()
			case errors.Is(err, store.ErrRateLimited):
				if retryAfter <= 0 {
					t.Errorf("rate limited with retryAfter %v, want positive", retryAfter)
				}
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if granted != capacity {
		t.Fatalf("granted %d of %d takes, want exactly the capacity %d", granted, attempts, capacity)
	}
}
