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
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/progress"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/secrets"
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

// Worker handles one scan task at a time within a RabbitMQ consumer.
type Worker struct {
	engine *scan.Engine
	db     *store.DB
	disp   Enqueuer
	log    *slog.Logger
	sealer *secrets.Sealer // nil unless SECRET_SEAL_KEY is configured

	// progress shows each repository's current step on the dashboard. Nil
	// publishes nothing, and nothing here waits on it.
	progress *progress.Publisher
}

// WithProgress publishes live progress for every scan this worker runs.
func (w *Worker) WithProgress(p *progress.Publisher) *Worker {
	w.progress = p
	return w
}

// WithSealer lets the worker keep, sealed, the credentials found in
// repositories whose owner is on the unmask list. Without it nothing is kept,
// whatever the list says.
func (w *Worker) WithSealer(s *secrets.Sealer) *Worker {
	w.sealer = s
	return w
}

// New builds a worker.
func New(engine *scan.Engine, db *store.DB, disp Enqueuer, log *slog.Logger) *Worker {
	return &Worker{engine: engine, db: db, disp: disp, log: log}
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
	err := w.scan(ctx, &task, d.Attempt)
	if err != nil && d.Final {
		w.giveUp(ctx, &task, d.Attempt, err)
	}
	return err
}

// event builds a progress event for task's repository. Attempt is shown from 1:
// the delivery counts prior attempts, a person counts this one.
func event(task *gpb.ScanTask, repo string, attempt int, stage, status string) progress.Event {
	return progress.Event{
		JobID: task.JobId, RepoID: task.RepositoryId, Repo: repo,
		Stage: stage, Status: status, Attempt: attempt + 1,
	}
}

// giveUp counts a repository that will never be scanned as done, so its job can
// still complete.
//
// The task is about to be dead-lettered. Without this, repos_scanned never
// reaches repos_total and the job reports "running" forever for a repository
// nobody is working on — one unclonable repository holding up a whole
// organisation's result. The count may overshoot if an earlier delivery got as
// far as RecordScanResult before failing; completion tests scanned >= total,
// so that ends the job, which is the right outcome for a task that is gone.
func (w *Worker) giveUp(ctx context.Context, task *gpb.ScanTask, attempt int, cause error) {
	jobID := entity.ID(task.JobId)
	w.log.Error("repository skipped after final attempt",
		"job", task.JobId, "repo_id", task.RepositoryId, "attempt", attempt, "error", cause)
	ev := event(task, "", attempt, progress.StageRepo, progress.StatusSkipped)
	ev.Error = cause.Error()
	w.progress.Publish(ctx, ev)
	if err := w.db.RecordScanResult(ctx, jobID, 0, 0); err != nil {
		w.log.Error("could not count skipped repository", "job", task.JobId, "error", err)
		return
	}
	// The task is being given up on either way; completeIfAllScanned has
	// logged any failure, and there is nothing left to return it to.
	_ = w.completeIfAllScanned(ctx, jobID)
}

// completeIfAllScanned ends the job when its last repository is counted and
// says so on the live view.
func (w *Worker) completeIfAllScanned(ctx context.Context, jobID entity.ID) error {
	done, err := w.db.CompleteIfAllScanned(ctx, jobID)
	if err != nil {
		w.log.Error("could not complete job", "job", jobID.String(), "error", err)
		return err
	}
	if done {
		w.progress.Publish(ctx, progress.Event{JobID: jobID.String(), Stage: progress.StageJob, Status: progress.StatusDone})
	}
	return nil
}

func (w *Worker) scan(ctx context.Context, task *gpb.ScanTask, attempt int) error {
	jobID := entity.ID(task.JobId)
	repoID := entity.ID(task.RepositoryId)
	started := time.Now()

	// Tasks for a job that has ended are acked without cloning. The queue is
	// shared and first-in first-out, so a cancelled job's backlog otherwise
	// sits in front of every job submitted after it — 79k tasks once, which
	// held a seven-repository scan for more than a day. One indexed lookup per
	// task is the price of a cancel that takes effect.
	if skip, err := w.jobEnded(ctx, task); skip || err != nil {
		return err
	}

	// Loaded once and used twice: to name the repository in the log lines that
	// bracket the scan, and to stamp the finding events. A log line carrying
	// only ids is unreadable while a scan is in flight, which is exactly when
	// it is read.
	repo, repoErr := w.db.GetRepository(ctx, repoID)
	if repoErr != nil {
		w.log.Debug("repository metadata unavailable", "repo_id", task.RepositoryId, "error", repoErr)
	}
	meta := repoLogFields(task, repo, repoErr)
	w.log.Info("scan started", append([]any{"job", task.JobId}, meta...)...)
	name := repoName(repo, repoErr)
	w.progress.Publish(ctx, event(task, name, attempt, progress.StageRepo, progress.StatusStart))

	// A single-repo job starts running on its first (only) task. For a bulk job
	// the discovery worker has already moved it to running, and SetScanState is
	// a no-op when the state is unchanged.
	_ = w.db.SetScanState(ctx, jobID, entity.ScanRunning, "")

	sink := &storeSink{w: w, task: task, repo: name, attempt: attempt}
	if w.sealer != nil && repoErr == nil {
		keep, err := w.db.IsUnmaskOwner(ctx, repo.Owner)
		if err != nil {
			return err
		}
		sink.keep = keep
	}
	res, err := w.engine.Scan(ctx, task.JobId, task.RepositoryId, task.CloneUrl, sink)
	if err != nil {
		w.log.Error("scan failed", append([]any{"job", task.JobId, "error", err}, meta...)...)
		ev := event(task, name, attempt, progress.StageRepo, progress.StatusFailed)
		ev.Error, ev.ElapsedMs = err.Error(), time.Since(started).Milliseconds()
		w.progress.Publish(ctx, ev)
		return fmt.Errorf("scan repository %s: %w", task.RepositoryId, err)
	}

	if res.AttributionErr != nil {
		w.log.Warn("findings stored without commit attribution",
			append([]any{"job", task.JobId, "error", res.AttributionErr}, meta...)...)
	}
	w.log.Info("scan done", append([]any{
		"job", task.JobId, "blobs", res.BlobsScanned,
		"findings", sink.inserted, "reclassified", sink.reclassified,
		"attributed", sink.attributed, "kept", sink.kept, "app_type", res.AppType,
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
	ev := event(task, name, attempt, progress.StageRepo, progress.StatusDone)
	ev.Blobs, ev.Findings, ev.Reclassified, ev.Live = res.BlobsScanned, sink.inserted, sink.reclassified, res.ProbesLive
	ev.AppType, ev.ElapsedMs = res.AppType, time.Since(started).Milliseconds()
	w.progress.Publish(ctx, ev)

	// A single-repo job, or the last repo of a bulk job, completes here. The
	// check is transactional and no-ops when more repos are still pending
	// (plan.md §8), so concurrent workers finishing different repos race safely.
	return w.completeIfAllScanned(ctx, jobID)
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
	repo         string // owner/name, for progress events
	attempt      int
	inserted     int
	reclassified int
	attributed   int // existing rows given their introducing commit

	// keep is set for an unmask-listed owner's repository; kept counts the
	// values sealed, for the log line. Never the values themselves.
	keep bool
	kept int

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

// jobEnded reports whether task belongs to a job that no longer wants work:
// deleted, or in a terminal state.
func (w *Worker) jobEnded(ctx context.Context, task *gpb.ScanTask) (bool, error) {
	job, err := w.db.GetScanJob(ctx, entity.ID(task.JobId))
	if errors.Is(err, store.ErrNotFound) {
		w.log.Debug("scan skipped, job no longer exists", "job", task.JobId, "repo_id", task.RepositoryId)
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if job.State.Terminal() {
		w.log.Debug("scan skipped, job has ended",
			"job", task.JobId, "state", string(job.State), "repo_id", task.RepositoryId)
		return true, nil
	}
	return false, nil
}

// ScanProgress implements scan.ProgressReporter.
func (s *storeSink) ScanProgress(ctx context.Context, p scan.Progress) {
	ev := event(s.task, s.repo, s.attempt, p.Stage, p.Status)
	ev.Blobs, ev.Matches, ev.Findings, ev.Live, ev.Path = p.Blobs, p.Matches, p.Findings, p.Live, p.Path
	ev.ElapsedMs = p.Elapsed.Milliseconds()
	if p.Err != nil {
		ev.Error = p.Err.Error()
	}
	s.w.progress.Publish(ctx, ev)
}

// KeepsSecrets implements scan.SecretKeeper.
func (s *storeSink) KeepsSecrets() bool { return s.keep }

// KeepSecret seals the credential under its repository and fingerprint and
// stores it. The plaintext goes no further than the sealer.
func (s *storeSink) KeepSecret(ctx context.Context, f entity.Finding, secret string) error {
	sealed, err := s.w.sealer.Seal(secret, store.SealedAAD(f.RepositoryID, f.Fingerprint))
	if err != nil {
		return err
	}
	if err := s.w.db.KeepSealedSecret(ctx, f.RepositoryID, f.Fingerprint, sealed); err != nil {
		return err
	}
	s.kept++
	return nil
}
