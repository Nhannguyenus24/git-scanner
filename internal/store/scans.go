package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// ErrNotFound is returned when a looked-up row does not exist.
var ErrNotFound = errors.New("store: not found")

// CreateTarget inserts a scan target and returns it with its generated id.
func (db *DB) CreateTarget(ctx context.Context, t entity.Target) (entity.Target, error) {
	if err := t.Validate(); err != nil {
		return entity.Target{}, fmt.Errorf("store: invalid target: %w", err)
	}
	t.ID = id.New()
	// The column is NOT NULL DEFAULT '{}', and a nil slice marshals to SQL NULL,
	// so a repo/user target (no allowlist) must send an empty array.
	allowlist := t.Allowlist
	if allowlist == nil {
		allowlist = []string{}
	}
	_, err := db.pool.Exec(ctx,
		`INSERT INTO targets (id, kind, value, allowlist) VALUES ($1,$2,$3,$4)`,
		t.ID, string(t.Kind), t.Value, allowlist)
	if err != nil {
		return entity.Target{}, fmt.Errorf("store: create target: %w", err)
	}
	return t, nil
}

// CreateScanJob inserts a job in the queued state.
func (db *DB) CreateScanJob(ctx context.Context, targetID entity.ID) (entity.ScanJob, error) {
	j := entity.ScanJob{ID: id.New(), TargetID: targetID, State: entity.ScanQueued}
	_, err := db.pool.Exec(ctx,
		`INSERT INTO scan_jobs (id, target_id, state) VALUES ($1,$2,$3)`,
		j.ID, j.TargetID, string(j.State))
	if err != nil {
		return entity.ScanJob{}, fmt.Errorf("store: create scan job: %w", err)
	}
	return j, nil
}

// UpsertRepository inserts or updates a repository by its unique (owner, name),
// returning the stored row with its id. Discovery and direct repo scans both
// funnel through here so a repo scanned twice keeps one row.
func (db *DB) UpsertRepository(ctx context.Context, r entity.Repository) (entity.Repository, error) {
	if r.ID == "" {
		r.ID = id.New()
	}
	const q = `
		INSERT INTO repositories (id, target_id, owner, name, clone_url, default_branch, is_fork, is_private, stars, app_type, remote_created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (owner, name) DO UPDATE SET
			clone_url = EXCLUDED.clone_url,
			default_branch = EXCLUDED.default_branch,
			is_fork = EXCLUDED.is_fork,
			is_private = EXCLUDED.is_private,
			stars = EXCLUDED.stars,
			-- COALESCE so a caller that does not carry the remote date, such as a
			-- direct single-repo scan, cannot blank one discovery already stored.
			remote_created_at = COALESCE(EXCLUDED.remote_created_at, repositories.remote_created_at)
		RETURNING id`
	err := db.pool.QueryRow(ctx, q,
		r.ID, r.TargetID, r.Owner, r.Name, r.CloneURL, orDefault(r.DefaultBranch, "main"),
		r.IsFork, r.IsPrivate, r.Stars, r.AppType, r.RemoteCreatedAt).Scan(&r.ID)
	if err != nil {
		return entity.Repository{}, fmt.Errorf("store: upsert repository: %w", err)
	}
	return r, nil
}

// ExistingRepoFullNames returns which of the given "owner/name" identifiers
// already have a repository row. Random discovery uses it to skip repositories
// it has scanned before, so a long-running sample never scans the same repo
// twice even when two random pages overlap.
func (db *DB) ExistingRepoFullNames(ctx context.Context, fullNames []string) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(fullNames))
	if len(fullNames) == 0 {
		return seen, nil
	}
	rows, err := db.pool.Query(ctx,
		`SELECT owner || '/' || name FROM repositories WHERE owner || '/' || name = ANY($1)`,
		fullNames)
	if err != nil {
		return nil, fmt.Errorf("store: existing repo names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err != nil {
			return nil, fmt.Errorf("store: scan existing repo name: %w", err)
		}
		seen[fn] = struct{}{}
	}
	return seen, rows.Err()
}

// SetScanState moves a job to a new state, recording start and finish times.
// It rejects a move the state machine forbids (plan.md entity.CanTransition),
// so a buggy worker cannot resurrect a completed job.
func (db *DB) SetScanState(ctx context.Context, jobID entity.ID, to entity.ScanState, errMsg string) error {
	return db.InTx(ctx, func(tx pgx.Tx) error {
		var fromStr string
		if err := tx.QueryRow(ctx,
			`SELECT state FROM scan_jobs WHERE id = $1 FOR UPDATE`, jobID).Scan(&fromStr); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("store: load job state: %w", err)
		}

		from := entity.ScanState(fromStr)
		if from == to {
			return nil // idempotent: a redelivered task may set the same state twice
		}
		if !entity.CanTransition(from, to) {
			return fmt.Errorf("store: illegal scan transition %s -> %s", from, to)
		}

		_, err := tx.Exec(ctx, `
			UPDATE scan_jobs SET
				state = $2,
				error = $3,
				started_at  = CASE WHEN $2 = 'running'   AND started_at  IS NULL THEN now() ELSE started_at  END,
				finished_at = CASE WHEN $2 IN ('completed','failed','cancelled') THEN now() ELSE finished_at END,
				updated_at = now()
			WHERE id = $1`, jobID, string(to), errMsg)
		return err
	})
}

// RequestPause asks the worker holding jobID to stop at its next page
// (plan.md §8). The state stays as it is until a worker observes the request.
func (db *DB) RequestPause(ctx context.Context, jobID entity.ID) error {
	return db.setPauseRequested(ctx, jobID, true)
}

// ClearPause withdraws a pause request so a resumed job is not paused again.
func (db *DB) ClearPause(ctx context.Context, jobID entity.ID) error {
	return db.setPauseRequested(ctx, jobID, false)
}

func (db *DB) setPauseRequested(ctx context.Context, jobID entity.ID, v bool) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE scan_jobs SET pause_requested = $2, updated_at = now() WHERE id = $1`, jobID, v)
	if err != nil {
		return fmt.Errorf("store: set pause requested: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PauseRequested reports whether an operator asked jobID to pause. A job that no
// longer exists reports false, so discovery for a deleted job is not retried
// into the dead-letter queue over a missing row.
func (db *DB) PauseRequested(ctx context.Context, jobID entity.ID) (bool, error) {
	var paused bool
	err := db.pool.QueryRow(ctx,
		`SELECT pause_requested FROM scan_jobs WHERE id = $1`, jobID).Scan(&paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: pause requested: %w", err)
	}
	return paused, nil
}

// SetRepositoryAppType records the stack a scan detected in a repository.
//
// It is written here rather than at discovery because nothing short of walking
// the files can tell: the detector accumulates evidence across the whole blob
// walk and only reaches a verdict once it ends. An empty verdict is not stored,
// so a scan that recognised nothing leaves an earlier scan's answer standing.
func (db *DB) SetRepositoryAppType(ctx context.Context, repoID entity.ID, appType string) error {
	if appType == "" {
		return nil
	}
	_, err := db.pool.Exec(ctx,
		`UPDATE repositories SET app_type = $2 WHERE id = $1`, repoID, appType)
	if err != nil {
		return fmt.Errorf("store: set repository app type: %w", err)
	}
	return nil
}

// RecordScanResult updates a job's progress counters after a repository scan.
func (db *DB) RecordScanResult(ctx context.Context, jobID entity.ID, blobs, findings int) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE scan_jobs SET
			repos_scanned = repos_scanned + 1,
			blobs_scanned = blobs_scanned + $2,
			findings_count = findings_count + $3,
			updated_at = now()
		WHERE id = $1`, jobID, blobs, findings)
	if err != nil {
		return fmt.Errorf("store: record scan result: %w", err)
	}
	return nil
}

// GetScanJob loads one job.
func (db *DB) GetScanJob(ctx context.Context, jobID entity.ID) (entity.ScanJob, error) {
	const q = `
		SELECT id, target_id, state, cursor, repos_total, repos_scanned,
		       blobs_scanned, findings_count, registry_ver, error,
		       started_at, finished_at, created_at, updated_at
		FROM scan_jobs WHERE id = $1`
	var j entity.ScanJob
	var state string
	var started, finished *time.Time
	err := db.pool.QueryRow(ctx, q, jobID).Scan(
		&j.ID, &j.TargetID, &state, &j.Cursor, &j.ReposTotal, &j.ReposScanned,
		&j.BlobsScanned, &j.FindingsCount, &j.RegistryVer, &j.Error,
		&started, &finished, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ScanJob{}, ErrNotFound
	}
	if err != nil {
		return entity.ScanJob{}, fmt.Errorf("store: get scan job: %w", err)
	}
	j.State = entity.ScanState(state)
	j.StartedAt, j.FinishedAt = started, finished
	return j, nil
}

// GetTarget loads one scan target, so a rescan can re-dispatch the same kind of
// work (a repo URL, or a user/org to enumerate) without the caller resupplying it.
func (db *DB) GetTarget(ctx context.Context, targetID entity.ID) (entity.Target, error) {
	const q = `SELECT id, kind, value, allowlist, created_at FROM targets WHERE id = $1`
	var t entity.Target
	var kind string
	err := db.pool.QueryRow(ctx, q, targetID).Scan(&t.ID, &kind, &t.Value, &t.Allowlist, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Target{}, ErrNotFound
	}
	if err != nil {
		return entity.Target{}, fmt.Errorf("store: get target: %w", err)
	}
	t.Kind = entity.TargetKind(kind)
	return t, nil
}

// DeleteScanJob removes a scan job and, by the ON DELETE CASCADE on
// findings.scan_job_id, the findings it recorded. The repository and target
// rows are left untouched: they are shared across scans and owned by the target.
//
// The analytics rollups are counters decoupled from findings (no foreign key),
// so a delete does not decrement them — the same already-documented limitation
// as a reclassification; a clean rollup needs a data wipe, not a single delete.
func (db *DB) DeleteScanJob(ctx context.Context, jobID entity.ID) error {
	tag, err := db.pool.Exec(ctx, `DELETE FROM scan_jobs WHERE id = $1`, jobID)
	if err != nil {
		return fmt.Errorf("store: delete scan job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
