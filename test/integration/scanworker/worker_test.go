//go:build integration

// Full Phase 1 path against real infrastructure: a ScanTask published to
// RabbitMQ is consumed by the worker, which clones a local repo, detects a
// secret, stores it in Postgres and publishes a FindingDetected for analytics.
//
// Run with: make test-integration (needs POSTGRES_URL, RABBITMQ_URL).
package scanworker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/scanworker"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/observability"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

func env(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Skipf("%s not set", k)
	}
	return v
}

func buildRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", a...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", a, err, out)
		}
	}
	stripeKey := "sk_live" + "_0123456789abcdefghijklmn"
	if err := os.WriteFile(filepath.Join(dir, "config.env"),
		[]byte("STRIPE_SECRET_KEY="+stripeKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q", "-b", "main")
	run("add", "-A")
	run("commit", "-qm", "c")
	return dir
}

func TestWorkerStoresAndPublishes(t *testing.T) {
	ctx := context.Background()
	log := observability.NewLogger("test", "error")

	db, err := store.Open(ctx, env(t, "POSTGRES_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	conn, err := rabbit.Connect(ctx, env(t, "RABBITMQ_URL"), log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Seed target, job, repository.
	target, err := db.CreateTarget(ctx, entity.Target{
		Kind: entity.TargetRepo, Value: "https://github.com/t/r.git"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateScanJob(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	repoDir := buildRepo(t)
	repo, err := db.UpsertRepository(ctx, entity.Repository{
		TargetID: target.ID, Owner: "t", Name: "r-" + job.ID.String()[:8],
		CloneURL: "file://" + repoDir})
	if err != nil {
		t.Fatal(err)
	}

	hasher, _ := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	engine := scan.NewEngine(
		&scan.GitCLI{ScratchDir: t.TempDir(), AllowProtocols: []string{"file"}},
		scan.NewDetector(scan.BootstrapRuleset()), hasher)
	workerPub, err := conn.NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	worker := scanworker.New(engine, db, dispatch.New(workerPub), log)

	consumer, err := conn.NewConsumer(rabbit.QueueScan, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { _ = consumer.Run(runCtx, worker.Handle) }()

	// Publish the scan task.
	pub, err := conn.NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := proto.Marshal(&gpb.ScanTask{
		JobId: job.ID.String(), RepositoryId: repo.ID.String(), CloneUrl: repo.CloneURL})
	if err := pub.Publish(ctx, rabbit.QueueScan, body, nil); err != nil {
		t.Fatal(err)
	}

	// Poll for the finding.
	deadline := time.Now().Add(30 * time.Second)
	for {
		findings, err := db.ListFindings(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) > 0 {
			f := findings[0]
			if f.ProviderSlug != "stripe" {
				t.Errorf("provider = %q, want stripe", f.ProviderSlug)
			}
			if strings.Contains(f.MaskedPreview, "456789abcdefghij") {
				t.Error("preview leaked the secret body")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("finding was not stored within 30s")
		}
		time.Sleep(500 * time.Millisecond)
	}

	job2, _ := db.GetScanJob(ctx, job.ID)
	if job2.State != entity.ScanRunning {
		t.Errorf("job state = %s, want running", job2.State)
	}
}

type failEnqueuer struct{ t *testing.T }

func (f failEnqueuer) EnqueueEnrich(context.Context, dispatch.EnrichRequest) error {
	f.t.Error("a skipped scan enqueued enrichment")
	return nil
}
func (f failEnqueuer) PublishFinding(context.Context, entity.Finding, string, string) error {
	f.t.Error("a skipped scan published a finding")
	return nil
}

// A task for a cancelled job is acked without cloning, so a cancelled job's
// backlog drains instead of holding every later job behind it. The clone URL
// points at a repository with a secret: had the scan run, a finding would exist.
func TestWorkerSkipsCancelledJob(t *testing.T) {
	ctx := context.Background()
	log := observability.NewLogger("test", "error")

	db, err := store.Open(ctx, env(t, "POSTGRES_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	target, err := db.CreateTarget(ctx, entity.Target{Kind: entity.TargetRepo, Value: "https://github.com/t/c.git"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CreateScanJob(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DeleteScanJob(context.Background(), job.ID) })
	if err := db.SetScanState(ctx, job.ID, entity.ScanCancelled, "cancelled by test"); err != nil {
		t.Fatal(err)
	}
	repo, err := db.UpsertRepository(ctx, entity.Repository{
		TargetID: target.ID, Owner: "t", Name: "c-" + job.ID.String()[:8], CloneURL: "file://" + buildRepo(t)})
	if err != nil {
		t.Fatal(err)
	}

	hasher, _ := secrets.NewHasher([]byte("0123456789abcdef0123456789abcdef"))
	engine := scan.NewEngine(
		&scan.GitCLI{ScratchDir: t.TempDir(), AllowProtocols: []string{"file"}},
		scan.NewDetector(scan.BootstrapRuleset()), hasher)
	worker := scanworker.New(engine, db, failEnqueuer{t}, log)

	body, _ := proto.Marshal(&gpb.ScanTask{
		JobId: job.ID.String(), RepositoryId: repo.ID.String(), CloneUrl: repo.CloneURL})
	if err := worker.Handle(ctx, rabbit.Delivery{Body: body}); err != nil {
		t.Fatalf("Handle = %v, want nil (acked)", err)
	}

	findings, err := db.ListFindings(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("%d findings stored for a cancelled job, want 0", len(findings))
	}
	got, err := db.GetScanJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != entity.ScanCancelled || got.ReposScanned != 0 {
		t.Errorf("job = %s with %d scanned, want cancelled with 0", got.State, got.ReposScanned)
	}
}
