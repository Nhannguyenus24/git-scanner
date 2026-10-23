//go:build integration

// Full Phase 1 path against real infrastructure: a ScanTask published to
// RabbitMQ is consumed by the worker, which clones a local repo, detects a
// secret, stores it in Postgres and emits a FindingDetected on Kafka.
//
// Run with: make test-integration (needs POSTGRES_URL, RABBITMQ_URL,
// KAFKA_BOOTSTRAP_SERVERS).
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
	"github.com/mservice/gitscanner/libs/kafka"
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

	brokers := strings.Split(env(t, "KAFKA_BOOTSTRAP_SERVERS"), ",")
	prod, err := kafka.NewProducer(t.Context(), brokers, log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prod.Close(ctx) }()

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
	enrichPub, err := conn.NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	worker := scanworker.New(engine, db, prod, dispatch.New(enrichPub), log)

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
