package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
)

// SetScanCursor checkpoints a job's resume position (plan.md §8). The discovery
// worker writes it after each page so a pause or crash resumes from there.
func (db *DB) SetScanCursor(ctx context.Context, jobID entity.ID, cursor string) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE scan_jobs SET cursor = $2, updated_at = now() WHERE id = $1`, jobID, cursor)
	if err != nil {
		return fmt.Errorf("store: set cursor: %w", err)
	}
	return nil
}

// AddReposTotal grows a job's known repository count as discovery paginates.
// It is additive because the total is not known until the last page.
func (db *DB) AddReposTotal(ctx context.Context, jobID entity.ID, n int) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE scan_jobs SET repos_total = repos_total + $2, updated_at = now() WHERE id = $1`,
		jobID, n)
	if err != nil {
		return fmt.Errorf("store: add repos total: %w", err)
	}
	return nil
}

// SetReposTotal fixes a job's repository count, used for a single-repo scan
// where the total is known to be one up front.
func (db *DB) SetReposTotal(ctx context.Context, jobID entity.ID, n int) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE scan_jobs SET repos_total = $2, updated_at = now() WHERE id = $1`, jobID, n)
	if err != nil {
		return fmt.Errorf("store: set repos total: %w", err)
	}
	return nil
}

// CompleteIfAllScanned marks a running job completed once every discovered
// repository has been scanned. It is safe under concurrency: the row is locked,
// and only a running job with a known, met total transitions, so two workers
// finishing the last two repos cannot both complete it or complete it twice.
func (db *DB) CompleteIfAllScanned(ctx context.Context, jobID entity.ID) (bool, error) {
	var completed bool
	err := db.InTx(ctx, func(tx pgx.Tx) error {
		var state string
		var total, scanned int
		if err := tx.QueryRow(ctx,
			`SELECT state, repos_total, repos_scanned FROM scan_jobs WHERE id = $1 FOR UPDATE`,
			jobID).Scan(&state, &total, &scanned); err != nil {
			return fmt.Errorf("store: load job for completion: %w", err)
		}
		if entity.ScanState(state) != entity.ScanRunning || total <= 0 || scanned < total {
			return nil
		}
		if _, err := tx.Exec(ctx,
			`UPDATE scan_jobs SET state = 'completed', finished_at = now(), updated_at = now() WHERE id = $1`,
			jobID); err != nil {
			return err
		}
		completed = true
		return nil
	})
	return completed, err
}
