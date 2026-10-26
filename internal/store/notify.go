package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// CreateNotification inserts a disclosure record in the pending state.
//
// It does not check first whether the repository was already contacted. The
// unique partial index on (repository_id) for the pending and sent states is
// the authority (plan.md §0.3), so the caller distinguishes a duplicate by the
// unique violation this returns rather than by a racy pre-read.
func (db *DB) CreateNotification(ctx context.Context, n entity.Notification) (entity.Notification, error) {
	if err := n.Validate(); err != nil {
		return entity.Notification{}, fmt.Errorf("store: invalid notification: %w", err)
	}

	if n.ID == "" {
		n.ID = id.New()
	}
	if n.State == "" {
		n.State = entity.NotificationPending
	}

	// The column is NOT NULL DEFAULT '{}', and a nil slice marshals to SQL NULL.
	findingIDs := make([]string, 0, len(n.FindingIDs))
	for _, f := range n.FindingIDs {
		findingIDs = append(findingIDs, f.String())
	}

	const q = `
		INSERT INTO notifications (id, repository_id, recipient, finding_ids, state)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at`

	err := db.pool.QueryRow(ctx, q,
		n.ID, n.RepositoryID, n.Recipient, findingIDs, string(n.State)).Scan(&n.CreatedAt)
	if err != nil {
		// Returned unwrapped so the caller can inspect the SQLSTATE: a unique
		// violation here is the one-per-repository cap doing its job, not a fault.
		return entity.Notification{}, err
	}
	return n, nil
}

// SetNotificationState records the outcome of a send attempt.
func (db *DB) SetNotificationState(ctx context.Context, nID entity.ID, state entity.NotificationState, errMsg string) error {
	const q = `
		UPDATE notifications SET
			state   = $2,
			error   = $3,
			sent_at = CASE WHEN $2 = 'sent' THEN now() ELSE sent_at END
		WHERE id = $1`

	tag, err := db.pool.Exec(ctx, q, nID, string(state), errMsg)
	if err != nil {
		return fmt.Errorf("store: set notification state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSuppressions returns every do-not-contact pattern. The set is small and
// read once per delivery, so it is fetched whole rather than matched in SQL;
// that also keeps the matching rule in one place (entity.SuppressionMatches).
func (db *DB) ListSuppressions(ctx context.Context) ([]string, error) {
	rows, err := db.pool.Query(ctx, "SELECT pattern FROM suppressions")
	if err != nil {
		return nil, fmt.Errorf("store: list suppressions: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("store: scan suppression: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetRepository loads one repository.
func (db *DB) GetRepository(ctx context.Context, repoID entity.ID) (entity.Repository, error) {
	const q = `
		SELECT id, target_id, owner, name, clone_url, default_branch,
		       is_fork, is_private, stars, app_type, head_sha,
		       remote_created_at, last_scanned_at, created_at,
		       language, topics, owner_type, archived, size_kb, pushed_at, metadata_fetched_at
		FROM repositories WHERE id = $1`

	var r entity.Repository
	err := db.pool.QueryRow(ctx, q, repoID).Scan(
		&r.ID, &r.TargetID, &r.Owner, &r.Name, &r.CloneURL, &r.DefaultBranch,
		&r.IsFork, &r.IsPrivate, &r.Stars, &r.AppType, &r.HeadSHA,
		&r.RemoteCreatedAt, &r.LastScannedAt, &r.CreatedAt,
		&r.Language, &r.Topics, &r.OwnerType, &r.Archived, &r.SizeKB, &r.PushedAt, &r.MetadataFetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Repository{}, ErrNotFound
	}
	if err != nil {
		return entity.Repository{}, fmt.Errorf("store: get repository: %w", err)
	}
	return r, nil
}

// findingColumns is the select list scanFinding expects, kept beside the
// queries that use it so the two cannot drift.
const findingColumns = `
	id, scan_job_id, repository_id, rule_id, provider_slug,
	fingerprint, masked_preview, blob_oid, commit_sha, file_path, line_number,
	author_name, author_email, committed_at, reachable_from_head,
	severity, status, metadata`

// ListFindingsByIDs loads the findings a disclosure refers to, newest first.
func (db *DB) ListFindingsByIDs(ctx context.Context, ids []entity.ID) ([]entity.Finding, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	strs := make([]string, 0, len(ids))
	for _, i := range ids {
		strs = append(strs, i.String())
	}

	rows, err := db.pool.Query(ctx,
		"SELECT "+findingColumns+" FROM findings WHERE id = ANY($1) ORDER BY detected_at DESC", strs)
	if err != nil {
		return nil, fmt.Errorf("store: list findings by ids: %w", err)
	}
	defer rows.Close()

	return collectFindings(rows)
}

// ListOpenFindingsByRepository returns the unresolved findings for a repository,
// which is what a disclosure covers when the caller does not name ids itself.
func (db *DB) ListOpenFindingsByRepository(ctx context.Context, repoID entity.ID) ([]entity.Finding, error) {
	rows, err := db.pool.Query(ctx,
		"SELECT "+findingColumns+" FROM findings WHERE repository_id = $1 AND status = 'open' ORDER BY detected_at DESC",
		repoID)
	if err != nil {
		return nil, fmt.Errorf("store: list open findings: %w", err)
	}
	defer rows.Close()

	return collectFindings(rows)
}

func collectFindings(rows pgx.Rows) ([]entity.Finding, error) {
	var out []entity.Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
