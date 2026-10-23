package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
)

// FindingWrite reports what InsertFinding did to the findings table.
type FindingWrite int

const (
	// FindingUnchanged means the row was already recorded as it stands. This is
	// the normal outcome of a redelivered task.
	FindingUnchanged FindingWrite = iota
	// FindingInserted means a new finding was stored.
	FindingInserted
	// FindingReclassified means an existing row was attributed to a named rule
	// it had not been matched by before.
	FindingReclassified
)

// InsertFinding writes a finding, or re-attributes one already stored.
//
// The conflict target (fingerprint, repository_id) is what makes RabbitMQ
// redelivery harmless (plan.md §6): a replayed scan re-inserts the same rows
// and reports FindingUnchanged. It never stores the plaintext — the caller has
// already reduced the secret to a fingerprint and a preview.
//
// The update arm exists because the alternative silently discards the result of
// improving a rule. A credential first seen by the generic entropy pass is
// stored with no provider; when a later rule learns to name that shape, a
// rescan produces the same fingerprint and DO NOTHING would drop the better
// classification on the floor, leaving the finding permanently anonymous. The
// guard only lets a named rule replace the unattributed catch-all, never one
// provider replace another, so this cannot flip a finding between providers.
//
// xmax distinguishes the two arms: it is zero only on a freshly inserted row.
func (db *DB) InsertFinding(ctx context.Context, f entity.Finding) (FindingWrite, error) {
	if err := f.Validate(); err != nil {
		return FindingUnchanged, fmt.Errorf("store: invalid finding: %w", err)
	}

	const q = `
		INSERT INTO findings (
			id, scan_job_id, repository_id, rule_id, provider_slug,
			fingerprint, masked_preview, blob_oid, commit_sha, file_path, line_number,
			author_name, author_email, committed_at, reachable_from_head,
			severity, status, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (fingerprint, repository_id) DO UPDATE SET
			rule_id       = EXCLUDED.rule_id,
			provider_slug = EXCLUDED.provider_slug,
			severity      = EXCLUDED.severity,
			metadata      = EXCLUDED.metadata
		WHERE findings.provider_slug = '' AND EXCLUDED.provider_slug <> ''
		RETURNING (xmax = 0)`

	var fresh bool
	err := db.pool.QueryRow(ctx, q,
		f.ID, f.ScanJobID, f.RepositoryID, f.RuleID, f.ProviderSlug,
		f.Fingerprint, f.MaskedPreview, f.BlobOID, f.CommitSHA, f.FilePath, f.LineNumber,
		f.AuthorName, f.AuthorEmail, nullTime(f.CommittedAt), f.ReachableFromHEAD,
		f.Severity.String(), string(f.Status), f.Metadata).Scan(&fresh)

	// A conflict whose WHERE excluded it updates nothing and so returns no row.
	if errors.Is(err, pgx.ErrNoRows) {
		return FindingUnchanged, nil
	}
	if err != nil {
		return FindingUnchanged, fmt.Errorf("store: insert finding: %w", err)
	}
	if fresh {
		return FindingInserted, nil
	}
	return FindingReclassified, nil
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
	rows, err := db.pool.Query(ctx,
		"SELECT "+findingColumns+" FROM findings WHERE scan_job_id = $1 ORDER BY detected_at DESC", jobID)
	if err != nil {
		return nil, fmt.Errorf("store: list findings: %w", err)
	}
	defer rows.Close()

	return collectFindings(rows)
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
