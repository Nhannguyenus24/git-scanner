package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
)

// InsertFinding writes a finding, ignoring a redelivered duplicate.
//
// The ON CONFLICT on (fingerprint, repository_id) is what makes RabbitMQ
// redelivery harmless (plan.md §6): a replayed scan re-inserts the same rows
// and inserted reports false without error. It never stores the plaintext —
// the caller has already reduced the secret to a fingerprint and a preview.
func (db *DB) InsertFinding(ctx context.Context, f entity.Finding) (inserted bool, err error) {
	if err := f.Validate(); err != nil {
		return false, fmt.Errorf("store: invalid finding: %w", err)
	}

	const q = `
		INSERT INTO findings (
			id, scan_job_id, repository_id, rule_id, provider_slug,
			fingerprint, masked_preview, blob_oid, commit_sha, file_path, line_number,
			author_name, author_email, committed_at, reachable_from_head,
			severity, status, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (fingerprint, repository_id) DO NOTHING`

	tag, err := db.pool.Exec(ctx, q,
		f.ID, f.ScanJobID, f.RepositoryID, f.RuleID, f.ProviderSlug,
		f.Fingerprint, f.MaskedPreview, f.BlobOID, f.CommitSHA, f.FilePath, f.LineNumber,
		f.AuthorName, f.AuthorEmail, nullTime(f.CommittedAt), f.ReachableFromHEAD,
		f.Severity.String(), string(f.Status), f.Metadata)
	if err != nil {
		return false, fmt.Errorf("store: insert finding: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// CountFindings returns how many findings a job has recorded.
func (db *DB) CountFindings(ctx context.Context, jobID entity.ID) (int, error) {
	var n int
	err := db.pool.QueryRow(ctx,
		"SELECT count(*) FROM findings WHERE scan_job_id = $1", jobID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: count findings: %w", err)
	}
	return n, nil
}

// ListFindings returns a job's findings for the API and tests, newest first.
func (db *DB) ListFindings(ctx context.Context, jobID entity.ID) ([]entity.Finding, error) {
	const q = `
		SELECT id, scan_job_id, repository_id, rule_id, provider_slug,
		       fingerprint, masked_preview, blob_oid, commit_sha, file_path, line_number,
		       author_name, author_email, committed_at, reachable_from_head,
		       severity, status, metadata
		FROM findings WHERE scan_job_id = $1 ORDER BY detected_at DESC`

	rows, err := db.pool.Query(ctx, q, jobID)
	if err != nil {
		return nil, fmt.Errorf("store: list findings: %w", err)
	}
	defer rows.Close()

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

func scanFinding(row pgx.Row) (entity.Finding, error) {
	var f entity.Finding
	var sev, status string
	var committed *time.Time
	if err := row.Scan(
		&f.ID, &f.ScanJobID, &f.RepositoryID, &f.RuleID, &f.ProviderSlug,
		&f.Fingerprint, &f.MaskedPreview, &f.BlobOID, &f.CommitSHA, &f.FilePath, &f.LineNumber,
		&f.AuthorName, &f.AuthorEmail, &committed, &f.ReachableFromHEAD,
		&sev, &status, &f.Metadata,
	); err != nil {
		return entity.Finding{}, fmt.Errorf("store: scan finding: %w", err)
	}
	parsed, err := entity.ParseSeverity(sev)
	if err != nil {
		return entity.Finding{}, err
	}
	f.Severity = parsed
	f.Status = entity.FindingStatus(status)
	if committed != nil {
		f.CommittedAt = *committed
	}
	return f, nil
}

// nullTime maps a zero time to SQL NULL so a finding with no commit date does
// not store a year-1 timestamp.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
