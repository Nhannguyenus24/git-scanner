//go:build integration

package scan_test

import (
	"github.com/mservice/gitscanner/internal/scan"

	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mservice/gitscanner/libs/id"
	"github.com/mservice/gitscanner/libs/redis"
)

// Run with `make test-integration` against a real Redis (REDIS_URL).
func newRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	rc, err := redis.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	return rc
}

func TestBlobIndexReturnsOnlyUnseen(t *testing.T) {
	ctx := context.Background()
	idx := scan.NewBlobIndex(newRedis(t).Raw(), time.Minute)
	repo := id.New().String()

	first, err := idx.Unseen(ctx, repo, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, []string{"a", "b"}) {
		t.Fatalf("first run = %v, want every blob", first)
	}

	second, err := idx.Unseen(ctx, repo, []string{"b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, []string{"c"}) {
		t.Fatalf("second run = %v, want only the new blob", second)
	}
}

func TestControlPauseAndCursor(t *testing.T) {
	ctx := context.Background()
	c := scan.NewControl(newRedis(t).Raw())
	job := id.New().String()

	if paused, _ := c.PauseRequested(ctx, job); paused {
		t.Fatal("new job reports paused")
	}
	if err := c.RequestPause(ctx, job); err != nil {
		t.Fatal(err)
	}
	if paused, _ := c.PauseRequested(ctx, job); !paused {
		t.Fatal("pause flag not visible after RequestPause")
	}
	if err := c.ClearPause(ctx, job); err != nil {
		t.Fatal(err)
	}

	if cur, _ := c.Cursor(ctx, job); cur != "" {
		t.Fatalf("missing cursor = %q, want empty", cur)
	}
	if err := c.SaveCursor(ctx, job, "page-7", time.Minute); err != nil {
		t.Fatal(err)
	}
	if cur, _ := c.Cursor(ctx, job); cur != "page-7" {
		t.Fatalf("cursor = %q, want page-7", cur)
	}
}
