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

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
