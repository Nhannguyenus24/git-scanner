package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
)

// The catalog is the read side the dashboard browses: jobs with their targets,
// findings across every scan, repositories with their exposure, and the
// unmapped-provider backlog. It joins across tables the write path keeps
// separate, so its rows are view structs rather than entities.
//
// Nothing here returns a fingerprint, an author email or anything else that
// could help reconstruct or attribute a secret beyond what a finding's masked
// preview already shows.

// JobSummary is a scan job with the target it was submitted for.
type JobSummary struct {
	entity.ScanJob
	TargetKind  string
	TargetValue string
}

// ListScanJobs returns the most recent jobs first. The dashboard lists them
// from here rather than remembering the ids it submitted, so a scan started
// from the CLI, another browser or before a page reload is still visible.
func (db *DB) ListScanJobs(ctx context.Context, limit int) ([]JobSummary, error) {
	const q = `
		SELECT j.id, j.target_id, j.state, j.repos_total, j.repos_scanned,
		       j.blobs_scanned, j.findings_count, j.error,
		       j.started_at, j.finished_at, j.created_at, j.updated_at,
		       t.kind, t.value
		FROM scan_jobs j JOIN targets t ON t.id = j.target_id
		ORDER BY j.created_at DESC
		LIMIT $1`
	rows, err := db.pool.Query(ctx, q, clampLimit(limit, 50, 500))
	if err != nil {
		return nil, fmt.Errorf("store: list scan jobs: %w", err)
	}
	defer rows.Close()

	var out []JobSummary
	for rows.Next() {
		var s JobSummary
		var state string
		if err := rows.Scan(
			&s.ID, &s.TargetID, &state, &s.ReposTotal, &s.ReposScanned,
			&s.BlobsScanned, &s.FindingsCount, &s.Error,
			&s.StartedAt, &s.FinishedAt, &s.CreatedAt, &s.UpdatedAt,
			&s.TargetKind, &s.TargetValue,
		); err != nil {
			return nil, fmt.Errorf("store: scan job row: %w", err)
		}
		s.State = entity.ScanState(state)
		out = append(out, s)
	}
	return out, rows.Err()
}

// FindingFilter narrows SearchFindings. Zero values mean "any".
type FindingFilter struct {
	Severity     string
	Provider     string // "unmapped" selects findings no named rule attributed
	Status       string
	RepositoryID entity.ID
	ScanJobID    entity.ID
	Query        string // substring of path, rule, provider or repository
	Limit        int
	Offset       int
	// Reveal joins in the sealed credential, for rows whose owner is on the
	// unmask list. The caller sets it only while masking is switched off.
	Reveal bool
}

// FindingRow is a finding with the repository it was found in.
type FindingRow struct {
	ID                entity.ID
	ScanJobID         entity.ID
	RepositoryID      entity.ID
	RepoOwner         string
	RepoName          string
	RepoURL           string
	RuleID            string
	Provider          string
	Severity          string
	Status            string
	MaskedPreview     string
	FilePath          string
	LineNumber        int
	CommitSHA         string
	AuthorName        string
	CommittedAt       *time.Time
	ReachableFromHEAD bool
	DetectedAt        time.Time
	Metadata          map[string]string
	// Fingerprint and Sealed are set only under FindingFilter.Reveal, and only
	// for an unmasked owner's finding. Sealed is ciphertext; the api opens it.
	Fingerprint string
	Sealed      []byte
}

// SearchFindings returns one page of findings, most severe and most recent
// first, and the total matching count so the caller can page.
func (db *DB) SearchFindings(ctx context.Context, f FindingFilter) ([]FindingRow, int, error) {
	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if f.Severity != "" {
		where = append(where, "f.severity = "+arg(f.Severity))
	}
	switch f.Provider {
	case "":
	case "unmapped":
		where = append(where, "f.provider_slug = ''")
	default:
		where = append(where, "f.provider_slug = "+arg(f.Provider))
	}
	if f.Status != "" {
		where = append(where, "f.status = "+arg(f.Status))
	}
	if f.RepositoryID != "" {
		where = append(where, "f.repository_id = "+arg(f.RepositoryID))
	}
	if f.ScanJobID != "" {
		where = append(where, "f.scan_job_id = "+arg(f.ScanJobID))
	}
	if f.Query != "" {
		p := arg("%" + escapeLike(f.Query) + "%")
		where = append(where, "(f.file_path ILIKE "+p+" OR f.rule_id ILIKE "+p+
			" OR f.provider_slug ILIKE "+p+" OR (r.owner || '/' || r.name) ILIKE "+p+")")
	}
	cond := ""
	if len(where) > 0 {
		cond = "WHERE " + strings.Join(where, " AND ")
	}

	limit, offset := arg(clampLimit(f.Limit, 50, 500)), arg(max(f.Offset, 0))
	q := `
		SELECT f.id, f.scan_job_id, f.repository_id, r.owner, r.name, r.clone_url,
		       f.rule_id, f.provider_slug, f.severity, f.status, f.masked_preview,
		       f.file_path, f.line_number, f.commit_sha, f.author_name, f.committed_at,
		       f.reachable_from_head, f.detected_at, f.metadata,
		       f.fingerprint, s.sealed,
		       count(*) OVER ()
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		LEFT JOIN sealed_secrets s ON ` + arg(f.Reveal) + `
			AND s.repository_id = f.repository_id AND s.fingerprint = f.fingerprint
			AND s.expires_at > now()
			AND EXISTS (SELECT 1 FROM unmask_owners u WHERE u.owner = lower(r.owner))
		` + cond + `
		ORDER BY ` + severityRank("f.severity") + ` DESC, f.detected_at DESC, f.id
		LIMIT ` + limit + ` OFFSET ` + offset

	rows, err := db.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: search findings: %w", err)
	}
	defer rows.Close()

	var out []FindingRow
	total := 0
	for rows.Next() {
		var r FindingRow
		if err := rows.Scan(
			&r.ID, &r.ScanJobID, &r.RepositoryID, &r.RepoOwner, &r.RepoName, &r.RepoURL,
			&r.RuleID, &r.Provider, &r.Severity, &r.Status, &r.MaskedPreview,
			&r.FilePath, &r.LineNumber, &r.CommitSHA, &r.AuthorName, &r.CommittedAt,
			&r.ReachableFromHEAD, &r.DetectedAt, &r.Metadata,
			&r.Fingerprint, &r.Sealed, &total,
		); err != nil {
			return nil, 0, fmt.Errorf("store: finding row: %w", err)
		}
		if r.Sealed == nil {
			r.Fingerprint = ""
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// SetFindingStatus records a triage decision on one finding.
func (db *DB) SetFindingStatus(ctx context.Context, id entity.ID, status entity.FindingStatus) error {
	tag, err := db.pool.Exec(ctx, `UPDATE findings SET status = $2 WHERE id = $1`, id, string(status))
	if err != nil {
		return fmt.Errorf("store: set finding status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RepositorySummary is a repository with its open exposure.
type RepositorySummary struct {
	ID            entity.ID
	Owner         string
	Name          string
	CloneURL      string
	AppType       string
	Stars         int
	IsFork        bool
	LastScannedAt *time.Time
	Findings      int
	Open          int
	Critical      int
	High          int
	Medium        int
	Low           int
	Notice        *RepositoryNotice // latest disclosure; nil when never contacted
}

// RepositoryNotice is the most recent disclosure attempt for a repository.
type RepositoryNotice struct {
	State     string
	Recipient string
	Error     string
	CreatedAt time.Time
	SentAt    *time.Time
}

// ListRepositories returns every repository, the most exposed first.
func (db *DB) ListRepositories(ctx context.Context) ([]RepositorySummary, error) {
	const q = `
		SELECT r.id, r.owner, r.name, r.clone_url, r.app_type, r.stars, r.is_fork,
		       r.last_scanned_at,
		       count(f.id),
		       count(f.id) FILTER (WHERE f.status = 'open'),
		       count(f.id) FILTER (WHERE f.status = 'open' AND f.severity = 'critical'),
		       count(f.id) FILTER (WHERE f.status = 'open' AND f.severity = 'high'),
		       count(f.id) FILTER (WHERE f.status = 'open' AND f.severity = 'medium'),
		       count(f.id) FILTER (WHERE f.status = 'open' AND f.severity IN ('low', 'info')),
		       n.state, n.recipient, n.error, n.created_at, n.sent_at
		FROM repositories r
		LEFT JOIN findings f ON f.repository_id = r.id
		LEFT JOIN LATERAL (
			SELECT state, recipient, error, created_at, sent_at FROM notifications
			WHERE repository_id = r.id ORDER BY created_at DESC LIMIT 1
		) n ON true
		GROUP BY r.id, n.state, n.recipient, n.error, n.created_at, n.sent_at
		ORDER BY 11 DESC, 12 DESC, 10 DESC, r.owner, r.name`
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: list repositories: %w", err)
	}
	defer rows.Close()

	var out []RepositorySummary
	for rows.Next() {
		var r RepositorySummary
		var state, recipient, nerr *string
		var created, sent *time.Time
		if err := rows.Scan(
			&r.ID, &r.Owner, &r.Name, &r.CloneURL, &r.AppType, &r.Stars, &r.IsFork,
			&r.LastScannedAt, &r.Findings, &r.Open, &r.Critical, &r.High, &r.Medium, &r.Low,
			&state, &recipient, &nerr, &created, &sent,
		); err != nil {
			return nil, fmt.Errorf("store: repository row: %w", err)
		}
		if state != nil {
			r.Notice = &RepositoryNotice{
				State: *state, Recipient: *recipient, Error: *nerr, CreatedAt: *created, SentAt: sent,
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Count is one bucket of a breakdown.
type Count struct {
	Key   string
	Count int
}

// Overview is the dashboard's headline numbers.
type Overview struct {
	Findings          int
	OpenFindings      int
	Repositories      int
	ExposedRepos      int // repositories with at least one open finding
	ActiveJobs        int
	Candidates        int
	BySeverity        []Count // open findings
	ByProvider        []Count // open findings; "unmapped" for the catch-all
	ByStatus          []Count
	ByState           []Count // scan jobs
	ReachableFromHEAD int     // open findings still present at HEAD
	NotifiedRepos     int     // repositories with a disclosure sent or pending
	AwaitingNotice    int     // exposed repositories never successfully contacted
}

// GetOverview computes the dashboard totals in one round trip per breakdown.
// The tables are small enough that live aggregates beat maintaining rollups for
// this; the analytics service's rollups serve the time-series view instead.
func (db *DB) GetOverview(ctx context.Context) (Overview, error) {
	var o Overview
	const totals = `
		SELECT (SELECT count(*) FROM findings),
		       (SELECT count(*) FROM findings WHERE status = 'open'),
		       (SELECT count(*) FROM findings WHERE status = 'open' AND reachable_from_head),
		       (SELECT count(*) FROM repositories),
		       (SELECT count(DISTINCT repository_id) FROM findings WHERE status = 'open'),
		       (SELECT count(*) FROM scan_jobs WHERE state IN ('queued', 'running', 'paused')),
		       (SELECT count(*) FROM provider_candidates WHERE status IN ('new', 'triaged')),
		       (SELECT count(DISTINCT repository_id) FROM notifications WHERE state IN ('pending', 'sent')),
		       (SELECT count(DISTINCT f.repository_id) FROM findings f
		         WHERE f.status = 'open' AND NOT EXISTS (
		           SELECT 1 FROM notifications n
		           WHERE n.repository_id = f.repository_id AND n.state IN ('pending', 'sent', 'suppressed')))`
	if err := db.pool.QueryRow(ctx, totals).Scan(
		&o.Findings, &o.OpenFindings, &o.ReachableFromHEAD, &o.Repositories,
		&o.ExposedRepos, &o.ActiveJobs, &o.Candidates, &o.NotifiedRepos, &o.AwaitingNotice,
	); err != nil {
		return Overview{}, fmt.Errorf("store: overview totals: %w", err)
	}

	breakdowns := []struct {
		dst *[]Count
		q   string
	}{
		{&o.BySeverity, `SELECT severity, count(*) FROM findings WHERE status = 'open'
			GROUP BY 1 ORDER BY ` + severityRank("severity") + ` DESC`},
		{&o.ByProvider, `SELECT CASE provider_slug WHEN '' THEN 'unmapped' ELSE provider_slug END, count(*)
			FROM findings WHERE status = 'open' GROUP BY 1 ORDER BY 2 DESC, 1`},
		{&o.ByStatus, `SELECT status, count(*) FROM findings GROUP BY 1 ORDER BY 2 DESC`},
		{&o.ByState, `SELECT state, count(*) FROM scan_jobs GROUP BY 1 ORDER BY 2 DESC`},
	}
	for _, b := range breakdowns {
		counts, err := db.counts(ctx, b.q)
		if err != nil {
			return Overview{}, err
		}
		*b.dst = counts
	}
	return o, nil
}

func (db *DB) counts(ctx context.Context, q string) ([]Count, error) {
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: overview breakdown: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Count, error) {
		var c Count
		err := row.Scan(&c.Key, &c.Count)
		return c, err
	})
}

// ListProviderCandidates returns the unmapped-provider backlog, most frequent
// first. The sample is already masked when it is stored.
func (db *DB) ListProviderCandidates(ctx context.Context) ([]entity.ProviderCandidate, error) {
	const q = `
		SELECT id, signature, guessed_name, env_var_names, nearby_hosts, sample_masked,
		       charset_class, length_min, length_max, occurrences, repo_count, status,
		       promoted_to_slug, first_seen_at, last_seen_at
		FROM provider_candidates
		ORDER BY status IN ('mapped', 'ignored'), occurrences DESC, last_seen_at DESC`
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: list candidates: %w", err)
	}
	defer rows.Close()

	var out []entity.ProviderCandidate
	for rows.Next() {
		var c entity.ProviderCandidate
		var status string
		if err := rows.Scan(
			&c.ID, &c.Signature, &c.GuessedName, &c.EnvVarNames, &c.NearbyHosts, &c.SampleMasked,
			&c.CharsetClass, &c.LengthMin, &c.LengthMax, &c.Occurrences, &c.RepoCount, &status,
			&c.PromotedToSlug, &c.FirstSeenAt, &c.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("store: candidate row: %w", err)
		}
		c.Status = entity.CandidateStatus(status)
		out = append(out, c)
	}
	return out, rows.Err()
}

// severityRank orders severities as text sorts them wrong: "medium" > "high".
func severityRank(col string) string {
	return `CASE ` + col + ` WHEN 'critical' THEN 5 WHEN 'high' THEN 4 WHEN 'medium' THEN 3
		WHEN 'low' THEN 2 ELSE 1 END`
}

// escapeLike makes a user's search text literal inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func clampLimit(n, def, maxN int) int {
	if n <= 0 {
		return def
	}
	return min(n, maxN)
}

// TimePoint is one period of the leak timeline.
type TimePoint struct {
	Period   string // YYYY-MM
	Findings int
}

// FindingsTimeline buckets findings by the month the credential was committed,
// which answers "when were these secrets introduced" — the question a team
// actually asks. Findings with no commit date (not yet attributed) are grouped
// under an empty period and the caller can drop them.
func (db *DB) FindingsTimeline(ctx context.Context) ([]TimePoint, error) {
	const q = `
		SELECT to_char(date_trunc('month', committed_at), 'YYYY-MM') AS period, count(*)
		FROM findings
		WHERE committed_at IS NOT NULL
		GROUP BY 1 ORDER BY 1`
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: findings timeline: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (TimePoint, error) {
		var p TimePoint
		err := r.Scan(&p.Period, &p.Findings)
		return p, err
	})
}

// FindingsByAppType breaks open findings down by the kind of application the
// repository was detected to be.
func (db *DB) FindingsByAppType(ctx context.Context) ([]Count, error) {
	return db.counts(ctx, `
		SELECT CASE r.app_type WHEN '' THEN 'unknown' ELSE r.app_type END, count(f.id)
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		WHERE f.status = 'open'
		GROUP BY 1 ORDER BY 2 DESC`)
}

// FindingsByRule is the most-triggered detection rules, a read on what the
// engine actually catches.
func (db *DB) FindingsByRule(ctx context.Context, limit int) ([]Count, error) {
	return db.counts(ctx, `
		SELECT rule_id, count(*) FROM findings
		GROUP BY 1 ORDER BY 2 DESC LIMIT `+strconv.Itoa(clampLimit(limit, 12, 50)))
}

// LiveProbeCounts summarises what live validation found, for the analytics view.
// Keyed by probe_status in metadata; absent when probing was never run.
func (db *DB) LiveProbeCounts(ctx context.Context) ([]Count, error) {
	return db.counts(ctx, `
		SELECT metadata->>'probe_status', count(*) FROM findings
		WHERE metadata ? 'probe_status'
		GROUP BY 1 ORDER BY 2 DESC`)
}

// NotificationCounts breaks disclosure attempts down by outcome, for the
// analytics view. Every attempt counts, so a failed send that later succeeded
// appears under both.
func (db *DB) NotificationCounts(ctx context.Context) ([]Count, error) {
	return db.counts(ctx, `SELECT state, count(*) FROM notifications GROUP BY 1 ORDER BY 2 DESC`)
}
