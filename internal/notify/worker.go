// Package notify is the notifier service: it turns a disclosure request into
// one responsible-disclosure mail, under a cap shared by every replica and at
// most once per repository (plan.md §0.3).
//
// The order of the steps in Handle is the design. Suppression is checked before
// anything is recorded, the rate token is taken before the notification row
// exists, and the row's unique index — not an application check — is what makes
// the one-mail-per-repository promise hold under concurrency.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
	"github.com/mservice/gitscanner/libs/redis"
)

// uniqueViolation is the Postgres SQLSTATE for a unique constraint breach. Here
// it is an expected outcome, not a fault: it means another delivery already
// claimed this repository's single disclosure.
const uniqueViolation = "23505"

// Store is the database surface the notifier needs.
type Store interface {
	GetRepository(ctx context.Context, id entity.ID) (entity.Repository, error)
	ListFindingsByIDs(ctx context.Context, ids []entity.ID) ([]entity.Finding, error)
	ListSuppressions(ctx context.Context) ([]string, error)
	CreateNotification(ctx context.Context, n entity.Notification) (entity.Notification, error)
	SetNotificationState(ctx context.Context, id entity.ID, state entity.NotificationState, errMsg string) error
}

// Limiter is the shared send-rate cap.
type Limiter interface {
	Take(ctx context.Context, n float64) (retryAfter time.Duration, err error)
}

// Worker handles one disclosure request per delivery.
type Worker struct {
	store  Store
	sender Sender
	limit  Limiter
	log    *slog.Logger
}

// New builds a notifier worker.
func New(store Store, sender Sender, limit Limiter, log *slog.Logger) *Worker {
	return &Worker{store: store, sender: sender, limit: limit, log: log}
}

// Handle processes one NotifyTask delivery (the rabbit.Handler for notify.tasks).
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var task gpb.NotifyTask
	if err := proto.Unmarshal(d.Body, &task); err != nil {
		return fmt.Errorf("%w: unmarshal notify task: %v", rabbit.ErrDrop, err)
	}

	findingIDs := make([]entity.ID, 0, len(task.FindingIds))
	for _, f := range task.FindingIds {
		findingIDs = append(findingIDs, entity.ID(f))
	}

	n := entity.Notification{
		RepositoryID: entity.ID(task.RepositoryId),
		Recipient:    task.Recipient,
		FindingIDs:   findingIDs,
		State:        entity.NotificationPending,
	}
	// A malformed address or an empty finding set cannot become valid on a
	// retry, so this is a drop rather than a failure.
	if err := n.Validate(); err != nil {
		return fmt.Errorf("%w: %v", rabbit.ErrDrop, err)
	}

	w.log.Info("disclosure requested",
		"repo", task.RepositoryId, "recipient", task.Recipient, "findings", len(findingIDs))

	suppressed, err := w.suppressed(ctx, task.Recipient)
	if err != nil {
		return err
	}
	if suppressed {
		return w.recordSuppressed(ctx, n)
	}

	// The token is taken before the notification row is created, and the order
	// is load-bearing. A row written first would occupy the one-per-repository
	// unique index; when the rate limiter then sent the delivery back for a
	// retry, that retry would see its own earlier row as a duplicate, conclude
	// the repository had already been contacted, and silently drop the mail.
	// Taking the token first means a rate-limited delivery leaves no trace.
	if retryAfter, err := w.limit.Take(ctx, 1); err != nil {
		if errors.Is(err, redis.ErrRateLimited) {
			w.log.Info("mail rate cap reached, deferring",
				"repo", task.RepositoryId, "retry_after", retryAfter)
			return fmt.Errorf("notify: rate limited, retry after %s", retryAfter)
		}
		return err
	}

	stored, err := w.store.CreateNotification(ctx, n)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			// Another delivery, or an earlier run, already contacted this
			// repository. One mail per repository is the promise (plan.md §0.3).
			w.log.Info("disclosure skipped, repository already contacted", "repo", task.RepositoryId)
			return nil
		}
		return fmt.Errorf("notify: create notification: %w", err)
	}

	return w.send(ctx, stored)
}

// suppressed reports whether the recipient is on the do-not-contact list.
func (w *Worker) suppressed(ctx context.Context, recipient string) (bool, error) {
	patterns, err := w.store.ListSuppressions(ctx)
	if err != nil {
		return false, fmt.Errorf("notify: load suppressions: %w", err)
	}
	for _, p := range patterns {
		if entity.SuppressionMatches(p, recipient) {
			return true, nil
		}
	}
	return false, nil
}

// recordSuppressed stores the decision so the backlog shows the request was
// handled rather than lost. A duplicate here is harmless and is ignored.
func (w *Worker) recordSuppressed(ctx context.Context, n entity.Notification) error {
	n.State = entity.NotificationSuppressed

	if _, err := w.store.CreateNotification(ctx, n); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil
		}
		return fmt.Errorf("notify: record suppression: %w", err)
	}

	w.log.Info("disclosure suppressed", "repo", n.RepositoryID.String(), "recipient", n.Recipient)
	return nil
}

// send renders and delivers the mail, recording the outcome either way.
func (w *Worker) send(ctx context.Context, n entity.Notification) error {
	repo, err := w.store.GetRepository(ctx, n.RepositoryID)
	if err != nil {
		return fmt.Errorf("notify: load repository: %w", err)
	}
	findings, err := w.store.ListFindingsByIDs(ctx, n.FindingIDs)
	if err != nil {
		return fmt.Errorf("notify: load findings: %w", err)
	}

	msg := RenderDisclosure(repo, findings)
	msg.To = n.Recipient

	if err := w.sender.Send(ctx, msg); err != nil {
		// Record the failure before returning: a 'failed' row sits outside the
		// partial unique index, so the retry can create a fresh pending row.
		if setErr := w.store.SetNotificationState(ctx, n.ID, entity.NotificationFailed, err.Error()); setErr != nil {
			return errors.Join(err, setErr)
		}
		w.log.Error("disclosure send failed", "repo", repo.FullName(), "error", err)
		return fmt.Errorf("notify: send: %w", err)
	}

	if err := w.store.SetNotificationState(ctx, n.ID, entity.NotificationSent, ""); err != nil {
		return fmt.Errorf("notify: mark sent: %w", err)
	}

	w.log.Info("disclosure sent",
		"repo", repo.FullName(), "recipient", n.Recipient, "findings", len(findings))
	return nil
}
