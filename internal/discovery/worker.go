// Package discovery is the discovery-worker service: it expands an authorized
// target (a user, an org, or an explicit allowlist) into per-repository scan
// tasks, one page per delivery, with pause/resume checkpointing (plan.md §2, §8).
//
// Processing one page per delivery, then republishing a continuation task with
// the advanced cursor, is what keeps each delivery short: it respects prefetch
// backpressure and makes pause a matter of simply not republishing.
package discovery

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Pauser reports whether a job has been asked to pause. Backed by the Redis
// control flags the API sets (plan.md §8).
type Pauser interface {
	PauseRequested(ctx context.Context, jobID string) (bool, error)
}

// Store is the database surface discovery needs.
type Store interface {
	UpsertRepository(ctx context.Context, r entity.Repository) (entity.Repository, error)
	AddReposTotal(ctx context.Context, jobID entity.ID, n int) error
	SetScanCursor(ctx context.Context, jobID entity.ID, cursor string) error
	SetScanState(ctx context.Context, jobID entity.ID, to entity.ScanState, errMsg string) error
}

// Dispatcher enqueues the scan tasks discovery produces and the continuation
// tasks it schedules for itself.
type Dispatcher interface {
	EnqueueScan(ctx context.Context, job entity.ScanJob, repo entity.Repository) error
	EnqueueDiscovery(ctx context.Context, req dispatch.DiscoveryRequest) error
}

// Worker expands one target page per delivery.
type Worker struct {
	gh    github.Lister
	store Store
	disp  Dispatcher
	pause Pauser
}

// New builds a discovery worker.
func New(gh github.Lister, st Store, disp Dispatcher, pause Pauser) *Worker {
	return &Worker{gh: gh, store: st, disp: disp, pause: pause}
}

// Handle processes one DiscoveryTask delivery (the rabbit.Handler for
// discovery.tasks).
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var task gpb.DiscoveryTask
	if err := proto.Unmarshal(d.Body, &task); err != nil {
		return fmt.Errorf("%w: unmarshal discovery task: %v", rabbit.ErrDrop, err)
	}

	jobID := entity.ID(task.JobId)
	_ = w.store.SetScanState(ctx, jobID, entity.ScanRunning, "")

	// An operator may pause between pages; honour it before doing more work.
	paused, err := w.pause.PauseRequested(ctx, task.JobId)
	if err != nil {
		return err
	}
	if paused {
		if err := w.store.SetScanCursor(ctx, jobID, task.Cursor); err != nil {
			return err
		}
		return w.store.SetScanState(ctx, jobID, entity.ScanPaused, "")
	}

	if task.Kind == string(entity.TargetAllowlist) {
		return w.expandAllowlist(ctx, &task)
	}
	return w.expandPage(ctx, &task)
}

// expandPage lists one page of a user's or org's repositories, enqueues a scan
// per repo, and schedules the next page unless the listing is exhausted.
func (w *Worker) expandPage(ctx context.Context, task *gpb.DiscoveryTask) error {
	jobID := entity.ID(task.JobId)

	page, err := w.list(ctx, task)
	if err != nil {
		if errors.Is(err, github.ErrNotFound) {
			return fmt.Errorf("%w: target %q not found", rabbit.ErrDrop, task.Value)
		}
		// A rate-limit or transient error returns unacked, so the broker retries
		// it with backoff (plan.md §9). Progress already made is durable.
		return err
	}

	enqueued, err := w.enqueueRepos(ctx, task, page.Repos)
	if err != nil {
		return err
	}
	if err := w.store.AddReposTotal(ctx, jobID, enqueued); err != nil {
		return err
	}
	if err := w.store.SetScanCursor(ctx, jobID, page.NextCursor); err != nil {
		return err
	}

	if page.NextCursor == "" {
		return nil // target fully expanded; completion follows when scans finish
	}
	// Continue from the next page in a fresh delivery.
	next := *task
	next.Cursor = page.NextCursor
	return w.disp.EnqueueDiscovery(ctx, requestFromTask(&next))
}

// expandAllowlist enqueues a scan for each explicitly listed repository. The
// set is bounded and already authorized, so there is no pagination.
func (w *Worker) expandAllowlist(ctx context.Context, task *gpb.DiscoveryTask) error {
	jobID := entity.ID(task.JobId)
	repos := make([]github.Repo, 0, len(task.Allowlist))
	for _, u := range task.Allowlist {
		owner, name, err := github.ParseRepoURL(u)
		if err != nil {
			return fmt.Errorf("%w: %v", rabbit.ErrDrop, err)
		}
		repos = append(repos, github.Repo{Owner: owner, Name: name, CloneURL: u})
	}

	enqueued, err := w.enqueueRepos(ctx, task, repos)
	if err != nil {
		return err
	}
	return w.store.AddReposTotal(ctx, jobID, enqueued)
}

// enqueueRepos upserts each repository and enqueues a scan task for it,
// skipping forks unless the task asked to include them. It returns how many
// scans it enqueued so the caller can grow the job's total.
func (w *Worker) enqueueRepos(ctx context.Context, task *gpb.DiscoveryTask, repos []github.Repo) (int, error) {
	job := entity.ScanJob{ID: entity.ID(task.JobId)}
	n := 0
	for _, r := range repos {
		if r.IsFork && !task.IncludeForks {
			continue
		}
		stored, err := w.store.UpsertRepository(ctx, entity.Repository{
			TargetID: entity.ID(task.TargetId), Owner: r.Owner, Name: r.Name,
			CloneURL: r.CloneURL, DefaultBranch: r.DefaultBranch,
			IsFork: r.IsFork, IsPrivate: r.IsPrivate, Stars: r.Stars,
		})
		if err != nil {
			return n, err
		}
		if err := w.disp.EnqueueScan(ctx, job, stored); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (w *Worker) list(ctx context.Context, task *gpb.DiscoveryTask) (github.Page, error) {
	switch task.Kind {
	case string(entity.TargetUser):
		return w.gh.ListUserRepos(ctx, task.Value, task.Cursor)
	case string(entity.TargetOrg):
		return w.gh.ListOrgRepos(ctx, task.Value, task.Cursor)
	default:
		return github.Page{}, fmt.Errorf("%w: unknown discovery kind %q", rabbit.ErrDrop, task.Kind)
	}
}

func requestFromTask(task *gpb.DiscoveryTask) dispatch.DiscoveryRequest {
	return dispatch.DiscoveryRequest{
		JobID: entity.ID(task.JobId), TargetID: entity.ID(task.TargetId),
		Kind: task.Kind, Value: task.Value, Allowlist: task.Allowlist,
		IncludeForks: task.IncludeForks, Cursor: task.Cursor,
	}
}
