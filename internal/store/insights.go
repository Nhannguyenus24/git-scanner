package store

import (
	"context"
	"fmt"
)

// SharedKey is one credential found in more than one repository. Findings are
// unique per (fingerprint, repository), so the same fingerprint on several rows
// is the same secret pasted into several places — the spread a table of
// findings cannot show, because each row only knows its own repository.
type SharedKey struct {
	// KeyID is a prefix of the fingerprint, long enough to tell keys apart in
	// one response and short enough not to read as a credential. The full
	// fingerprint never leaves the store through this view.
	KeyID         string
	Provider      string
	RuleID        string
	Severity      string
	MaskedPreview string
	Repos         []SharedKeyRepo
}

// SharedKeyRepo is one repository a shared key was found in, with the state of
// that repository's copy.
type SharedKeyRepo struct {
	FindingID string
	RepoID    string
	Owner     string
	Name      string
	Status    string
	AtHead    bool
}

// keyIDLen is the fingerprint prefix exposed as a key's id: 64 bits, so two
// distinct keys in one response colliding is not a practical concern.
const keyIDLen = 16

// SharedKeys returns the credentials found in at least minRepos repositories,
// most widely spread first, at most limit of them, and how many qualify in all
// so a caller showing the first page can say it is one. Severity and preview
// come from the most severe copy, so a key that is critical anywhere reads
// critical.
func (db *DB) SharedKeys(ctx context.Context, minRepos int, page Page) ([]SharedKey, int, error) {
	const q = `
		WITH shared AS (
			SELECT fingerprint, count(DISTINCT repository_id) AS repos,
			       count(*) OVER () AS total
			FROM findings
			GROUP BY fingerprint
			HAVING count(DISTINCT repository_id) >= $1
			ORDER BY repos DESC, fingerprint
			LIMIT $2 OFFSET $3
		)
		SELECT f.fingerprint, f.provider_slug, f.rule_id, f.severity, f.masked_preview,
		       f.id, r.id, r.owner, r.name, f.status, f.reachable_from_head, s.total
		FROM findings f
		JOIN shared s ON s.fingerprint = f.fingerprint
		JOIN repositories r ON r.id = f.repository_id
		ORDER BY s.repos DESC, f.fingerprint,
		         array_position(ARRAY['critical','high','medium','low','info'], f.severity),
		         r.owner, r.name`
	limit, offset := clampLimit(page.Limit, 100, MaxPageSize), max(page.Offset, 0)
	rows, err := db.pool.Query(ctx, q, max(minRepos, 2), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store: shared keys: %w", err)
	}
	defer rows.Close()

	var out []SharedKey
	var total int
	last := ""
	for rows.Next() {
		var k SharedKey
		var r SharedKeyRepo
		var fp string
		if err := rows.Scan(&fp, &k.Provider, &k.RuleID, &k.Severity, &k.MaskedPreview,
			&r.FindingID, &r.RepoID, &r.Owner, &r.Name, &r.Status, &r.AtHead, &total); err != nil {
			return nil, 0, fmt.Errorf("store: shared keys: %w", err)
		}
		// Rows arrive grouped by fingerprint, most severe copy first, so the
		// first row of a group carries the key's own fields. Grouping is on the
		// whole fingerprint; only the prefix is handed on.
		if fp == last {
			out[len(out)-1].Repos = append(out[len(out)-1].Repos, r)
			continue
		}
		last = fp
		k.KeyID = fp[:min(len(fp), keyIDLen)]
		k.Repos = []SharedKeyRepo{r}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: shared keys: %w", err)
	}
	return out, total, nil
}

// LeakDay is one UTC calendar day on which credentials were committed.
type LeakDay struct {
	Day      string // YYYY-MM-DD, UTC
	Findings int
	Critical int // of which critical: the calendar marks the days that matter most
}

// LeakCalendar counts findings by the UTC day their credential was committed,
// across all time, for the dashboard's calendar. It is day-granular on
// purpose: committed_at is stored as an instant, the author's own offset is
// gone, and an hour-of-day reading would be a reading of UTC, not of habit.
// Findings not yet traced to a commit have no day and are left out.
func (db *DB) LeakCalendar(ctx context.Context) ([]LeakDay, error) {
	const q = `
		SELECT to_char(committed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
		       count(*), count(*) FILTER (WHERE severity = 'critical')
		FROM findings
		WHERE committed_at IS NOT NULL
		GROUP BY 1 ORDER BY 1`
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: leak calendar: %w", err)
	}
	defer rows.Close()
	var out []LeakDay
	for rows.Next() {
		var d LeakDay
		if err := rows.Scan(&d.Day, &d.Findings, &d.Critical); err != nil {
			return nil, fmt.Errorf("store: leak calendar: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: leak calendar: %w", err)
	}
	return out, nil
}
