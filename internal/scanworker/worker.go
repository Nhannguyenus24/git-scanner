// Package scanworker is the scan-worker service: it consumes scan tasks, runs
// the scan engine over each repository, persists findings idempotently and
// publishes them to the finding stream (plan.md §4, §10).
//
// It lives apart from internal/scan so the engine stays a pure library with no
// queue or database dependency, and this package holds all the wiring.
package scanworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Enqueuer is the dispatch surface the worker needs, narrowed to an interface
// so the worker can be tested without a broker.
type Enqueuer interface {
	EnqueueEnrich(ctx context.Context, req dispatch.EnrichRequest) error
	PublishFinding(ctx context.Context, f entity.Finding, repoFullName, appType string) error
}

// RepoAttributes reads a repository's host attributes. Satisfied by the GitHub
// client; narrowed so the worker can be tested without the network.
type RepoAttributes interface {
	GetRepo(ctx context.Context, owner, name string) (github.Repo, error)
}

// attributesTimeout bounds the one attribute read a scan may make. The scan
// does not depend on it, so a slow API must not hold the clone back.
const attributesTimeout = 10 * time.Second

// Worker handles one scan task at a time within a RabbitMQ consumer.
type Worker struct {
	engine *scan.Engine
	db     *store.DB
	disp   Enqueuer
	log    *slog.Logger

	attrs RepoAttributes // nil: attributes are only what discovery stored

	// attrsPausedUntil is set when GitHub rate-limits an attribute read, so the
	// deliveries that follow skip the call instead of each spending a request to
	// learn the same thing. Guarded by mu: the consumer runs deliveries
	// concurrently.
	mu               sync.Mutex
	attrsPausedUntil time.Time
}

// New builds a worker.
func New(engine *scan.Engine, db *store.DB, disp Enqueuer, log *slog.Logger) *Worker {
	return &Worker{engine: engine, db: db, disp: disp, log: log}
}

// WithRepoAttributes lets the worker fill in host attributes for repositories
// discovered without them — the random sample and direct repo scans.
func (w *Worker) WithRepoAttributes(a RepoAttributes) *Worker {
	w.attrs = a
	return w
}

// Handle processes one ScanTask delivery. It is the rabbit.Handler for
// scan.tasks: returning an error retries the task, which is safe because
// findings are idempotent on their fingerprint (plan.md §6).
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var task gpb.ScanTask
	if err := proto.Unmarshal(d.Body, &task); err != nil {
		// A malformed task will never parse on retry, so drop it to the
		// dead-letter queue rather than spin.
		return fmt.Errorf("%w: unmarshal scan task: %v", rabbit.ErrDrop, err)
	}
	return w.scan(ctx, &task)
}

func (w *Worker) scan(ctx context.Context, task *gpb.ScanTask) error {
	jobID := entity.ID(task.JobId)
	repoID := entity.ID(task.RepositoryId)

	// Loaded once and used twice: to name the repository in the log lines that
	// bracket the scan, and to stamp the finding events. A log line carrying
	// only ids is unreadable while a scan is in flight, which is exactly when
	// it is read.
	repo, repoErr := w.db.GetRepository(ctx, repoID)
	if repoErr != nil {
		w.log.Debug("repository metadata unavailable", "repo_id", task.RepositoryId, "error", repoErr)
	} else {
		w.fillAttributes(ctx, &repo)
	}
	meta := repoLogFields(task, repo, repoErr)
	w.log.Info("scan started", append([]any{"job", task.JobId}, meta...)...)

	// A single-repo job starts running on its first (only) task. For a bulk job
	// the discovery worker has already moved it to running, and SetScanState is
	// a no-op when the state is unchanged.
	_ = w.db.SetScanState(ctx, jobID, entity.ScanRunning, "")

	sink := &storeSink{w: w, task: task}
	res, err := w.engine.Scan(ctx, task.JobId, task.RepositoryId, task.CloneUrl, sink)
	if err != nil {
		w.log.Error("scan failed", append([]any{"job", task.JobId, "error", err}, meta...)...)
		return fmt.Errorf("scan repository %s: %w", task.RepositoryId, err)
	}

	if res.AttributionErr != nil {
		w.log.Warn("findings stored without commit attribution",
			append([]any{"job", task.JobId, "error", res.AttributionErr}, meta...)...)
	}
	if res.RemovalErr != nil {
		w.log.Warn("findings stored without their removal commit",
			append([]any{"job", task.JobId, "error", res.RemovalErr}, meta...)...)
	}
	// Counters for tuning; losing them is not worth failing a scan over.
	if err := w.db.SetRepositoryScanStats(ctx, repoID, res.Stats); err != nil {
		w.log.Warn("scan stats not stored", append([]any{"job", task.JobId, "error", err}, meta...)...)
	}
	w.log.Info("scan done", append([]any{
		"job", task.JobId, "blobs", res.BlobsScanned,
		"findings", sink.inserted, "reclassified", sink.reclassified,
		"attributed", sink.attributed, "app_type", res.AppType,
	}, meta...)...)

	// The detected stack is worth keeping: it feeds severity scoring (plan.md
	// §4.3) and is a rollup dimension, and only a completed walk can establish
	// it. Stored before the events go out so both carry the same answer.
	if err := w.db.SetRepositoryAppType(ctx, repoID, res.AppType); err != nil {
		return err
	}
	if err := w.publishFindings(ctx, sink, repoName(repo, repoErr), res.AppType); err != nil {
		return err
	}

	if err := w.db.RecordScanResult(ctx, jobID, res.BlobsScanned, sink.inserted); err != nil {
		return err
	}

	// A single-repo job, or the last repo of a bulk job, completes here. The
	// check is transactional and no-ops when more repos are still pending
	// (plan.md §8), so concurrent workers finishing different repos race safely.
	if _, err := w.db.CompleteIfAllScanned(ctx, jobID); err != nil {
		return err
	}
	return nil
}

// storeSink persists each finding, queues it for enrichment, and holds the ones
// analytics needs told about until the walk ends.
//
// It counts only the rows it actually inserted, so a redelivered task does not
// inflate the total, and only those are held for publishing — a finding an
// earlier delivery already announced is not announced twice.
type storeSink struct {
	w            *Worker
	task         *gpb.ScanTask
	inserted     int
	reclassified int
	attributed   int // existing rows given their introducing commit

	// announce holds the findings stored by this delivery. They cannot be
	// published as they are found: the event carries the repository's app type,
	// and that is only known once every blob has been seen.
	announce []entity.Finding
}

func (s *storeSink) Emit(ctx context.Context, f entity.Finding) error {
	write, err := s.w.db.InsertFinding(ctx, f)
	if err != nil {
		return err
	}

	// A row that already existed may predate commit attribution; give it the
	// commit this scan found. A fresh insert carried it already.
	if write != store.FindingInserted {
		filled, err := s.w.db.BackfillAttribution(ctx, f)
		if err != nil {
			return err
		}
		if filled {
			s.attributed++
		}
		// And its time-varying state — still at HEAD, removed by which commit,
		// live or not — is this scan's, not the first scan's.
		if err := s.w.db.RefreshFinding(ctx, f); err != nil {
			return err
		}
	}
	if err := s.w.db.RecordProbe(ctx, entity.ID(s.task.JobId), f); err != nil {
		return err
	}

	switch write {
	case store.FindingUnchanged:
		return nil

	case store.FindingReclassified:
		// The row existed but has just been attributed to a named rule, so its
		// severity needs recomputing against the provider it now names. It is
		// not a new finding and must not be counted or announced as one:
		// analytics has already counted this finding id and would reject the
		// replay anyway.
		s.reclassified++
		return s.w.enqueueEnrich(ctx, s.task, f)
	}

	s.inserted++
	s.announce = append(s.announce, f)
	return s.w.enqueueEnrich(ctx, s.task, f)
}

// publishFindings announces to analytics everything this scan stored.
//
// It runs after the walk because the app type the event carries is a verdict on
// the whole repository. The cost is a wider window in which a crash leaves
// findings in the database that analytics never counted; the same hole existed
// between the insert and the publish before, and closing it properly means
// re-announcing on redelivery, which belongs in its own change.
func (w *Worker) publishFindings(ctx context.Context, sink *storeSink, repoFullName, appType string) error {
	for _, f := range sink.announce {
		if err := w.disp.PublishFinding(ctx, f, repoFullName, appType); err != nil {
			return fmt.Errorf("scanworker: announce finding %s: %w", f.ID, err)
		}
	}
	return nil
}

// fillAttributes reads and stores a repository's host attributes if they are
// not known yet. It is best-effort throughout: the scan does not need them,
// so a failure is logged and the scan goes ahead.
//
// Without a token GitHub allows 60 requests an hour, which a random sample uses
// up quickly. A rate-limit answer therefore pauses attribute reads for as long
// as GitHub asks, rather than every following scan spending a request to be
// told the same.
func (w *Worker) fillAttributes(ctx context.Context, repo *entity.Repository) {
	if w.attrs == nil || repo.MetadataFetchedAt != nil {
		return
	}
	w.mu.Lock()
	paused := time.Now().Before(w.attrsPausedUntil)
	w.mu.Unlock()
	if paused {
		return
	}

	actx, cancel := context.WithTimeout(ctx, attributesTimeout)
	defer cancel()
	gh, err := w.attrs.GetRepo(actx, repo.Owner, repo.Name)
	var rl *github.RateLimitError
	switch {
	case errors.As(err, &rl):
		w.mu.Lock()
		w.attrsPausedUntil = time.Now().Add(rl.RetryAfter)
		w.mu.Unlock()
		w.log.Info("repository attribute reads paused: rate limited",
			"repo", repo.FullName(), "retry_after", rl.RetryAfter.String())
		return
	case err != nil:
		w.log.Warn("repository attributes unavailable", "repo", repo.FullName(), "error", err)
		return
	}

	gh.ApplyAttributes(repo, time.Now())
	if err := w.db.SetRepositoryAttributes(ctx, *repo); err != nil {
		w.log.Warn("repository attributes not stored", "repo", repo.FullName(), "error", err)
	}
}

// repoName is the owner/name the finding events carry, or empty when the
// repository could not be read. Empty is better than the "/" a zero-valued
// Repository would format to, which reads like a real name and is not one.
func repoName(r entity.Repository, lookupErr error) string {
	if lookupErr != nil {
		return ""
	}
	return r.FullName()
}

// repoLogFields describes the repository being scanned, for the log lines that
// bracket the scan.
//
// A lookup failure degrades to the ids and the URL the task already carries
// rather than failing: this is context for a human reading the log, and losing
// it is never a reason to abandon a scan that would otherwise succeed.
func repoLogFields(task *gpb.ScanTask, r entity.Repository, lookupErr error) []any {
	fields := []any{"repo_id", task.RepositoryId, "url", task.CloneUrl}
	if lookupErr != nil {
		return fields
	}

	fields = append(fields,
		"repo", r.FullName(),
		"branch", r.DefaultBranch,
		"stars", r.Stars,
		"fork", r.IsFork,
		"private", r.IsPrivate,
		"discovered_at", r.CreatedAt.UTC().Format(time.RFC3339),
	)
	if r.RemoteCreatedAt != nil {
		fields = append(fields, "repo_created_at", r.RemoteCreatedAt.UTC().Format(time.RFC3339))
	}
	return fields
}

// enqueueEnrich hands the finding to the enricher for registry resolution
// (plan.md §5). The detection context enrichment clusters on does not have
// columns of its own on the findings row; the engine records it in the
// finding's metadata, which is where these fields are read back from.
func (w *Worker) enqueueEnrich(ctx context.Context, task *gpb.ScanTask, f entity.Finding) error {
	// A missing or malformed length is simply unknown; it weakens the clustering
	// signature but is not a reason to fail the scan that produced the finding.
	length, _ := strconv.Atoi(f.Metadata["secret_length"])

	return w.disp.EnqueueEnrich(ctx, dispatch.EnrichRequest{
		FindingID:    f.ID,
		JobID:        entity.ID(task.JobId),
		RuleID:       f.RuleID,
		FilePath:     f.FilePath,
		NearbyEnvVar: f.Metadata["env_var"],
		NearbyHost:   f.Metadata["host"],
		CharsetClass: f.Metadata["charset_class"],
		SecretLength: length,
	})
}
