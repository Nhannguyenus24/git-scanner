package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Insights is the analyst's read of the corpus: where leaks live, how long they
// stay exposed, how owners respond, and how precise each rule is. Like the
// catalog it reads the findings table, the system of record, rather than the
// rollups; and like the catalog it never returns a fingerprint or an author
// email — secret reuse is reported by masked preview and counts only.
type Insights struct {
	// Repository attributes, over open findings.
	ByLanguage   []Count
	ByOwnerType  []Count
	RepoActivity []ActivityBucket
	// Repositories whose host attributes are known, out of all scanned; the
	// two breakdowns above are only as complete as this.
	AttributesKnown, Repositories int

	// FilePaths is open findings per path. Classifying a path into a kind is
	// detection knowledge, so the caller does it rather than SQL.
	FilePaths []Count

	RulePrecision []RuleTriage
	Reuse         ReuseSummary

	KeyTypes []KeyTypeCount
	Modes    []Count

	Exposure             []ExposureBucket
	MedianDaysToRemoval  *float64
	RemovedButOpen       int
	CommitContext        []Count
	Disclosure           DisclosureFunnel
	DetectionStats       []Count
	DetectionScannedRepo int // repositories whose latest scan recorded stats
}

// ActivityBucket groups exposed repositories by maintenance state.
type ActivityBucket struct {
	Key      string // active, stale, archived, unknown
	Repos    int
	Findings int
	AtHead   int
}

// RuleTriage is one rule's findings by triage outcome. Rotated and ignored
// findings were judged real; false positives were not; open ones are unjudged.
type RuleTriage struct {
	Rule                                    string
	Total, Open, Rotated, Ignored, FalsePos int
}

// ReuseSummary describes credentials found in more than one repository.
type ReuseSummary struct {
	Secrets int // distinct credentials seen in two or more repositories
	Repos   int // repository sightings of those credentials
	Top     []ReusedSecret
}

// ReusedSecret is one credential seen in several repositories, identified only
// by its masked preview.
type ReusedSecret struct {
	Preview  string
	Provider string
	Repos    int
	Forks    int // of those repositories, how many are forks
}

// KeyTypeCount counts open findings by the credential type extracted offline.
type KeyTypeCount struct {
	Provider, KeyType string
	Count             int
}

// ExposureBucket counts open findings by how long they were exposed: until the
// commit that removed them from the default branch, or until now for one still
// there.
type ExposureBucket struct {
	Key     string // 1d, 7d, 30d, 90d, 1y, over_1y, unknown
	AtHead  int
	Removed int
}

// DisclosureFunnel follows disclosed findings through later liveness probes.
type DisclosureFunnel struct {
	Repos, Findings  int
	LiveAtDisclosure int // last probe before the mail said live
	WentDead         int // ...and a later probe found it revoked
	StillLive        int // ...and the latest probe since still found it live
	Rotated          int // triaged as rotated, probed or not
	MedianDaysToDead *float64
}

// GetInsights gathers every insight in one call. Each query is small and
// reads indexed columns; they run in sequence because the dashboard polls this
// rarely and a failure in any one should fail the request plainly.
func (db *DB) GetInsights(ctx context.Context) (Insights, error) {
	var in Insights
	var err error

	if in.ByLanguage, err = db.counts(ctx, `
		SELECT CASE WHEN r.metadata_fetched_at IS NULL THEN 'unknown'
		            WHEN r.language = '' THEN 'none' ELSE r.language END, count(f.id)
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		WHERE f.status = 'open'
		GROUP BY 1 ORDER BY 2 DESC LIMIT 15`); err != nil {
		return in, err
	}
	if in.ByOwnerType, err = db.counts(ctx, `
		SELECT CASE WHEN r.metadata_fetched_at IS NULL OR r.owner_type = '' THEN 'unknown'
		            ELSE r.owner_type END, count(f.id)
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		WHERE f.status = 'open'
		GROUP BY 1 ORDER BY 2 DESC`); err != nil {
		return in, err
	}
	if in.RepoActivity, err = db.repoActivity(ctx); err != nil {
		return in, err
	}
	if err = db.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE metadata_fetched_at IS NOT NULL), count(*) FROM repositories`,
	).Scan(&in.AttributesKnown, &in.Repositories); err != nil {
		return in, fmt.Errorf("store: attribute coverage: %w", err)
	}

	if in.FilePaths, err = db.counts(ctx, `
		SELECT file_path, count(*) FROM findings WHERE status = 'open' GROUP BY 1`); err != nil {
		return in, err
	}
	if in.RulePrecision, err = db.rulePrecision(ctx); err != nil {
		return in, err
	}
	if in.Reuse, err = db.reuse(ctx); err != nil {
		return in, err
	}

	if in.KeyTypes, err = db.keyTypes(ctx); err != nil {
		return in, err
	}
	if in.Modes, err = db.counts(ctx, `
		SELECT coalesce(metadata->>'mode', metadata->>'stripe_mode'), count(*) FROM findings
		WHERE status = 'open' AND metadata ?| array['mode', 'stripe_mode']
		GROUP BY 1 ORDER BY 2 DESC`); err != nil {
		return in, err
	}

	if in.Exposure, err = db.exposure(ctx); err != nil {
		return in, err
	}
	if err = db.pool.QueryRow(ctx, `
		SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM removed_at - committed_at) / 86400)
		           FILTER (WHERE removed_at IS NOT NULL AND committed_at IS NOT NULL),
		       count(*) FILTER (WHERE NOT reachable_from_head AND status = 'open')
		FROM findings`,
	).Scan(&in.MedianDaysToRemoval, &in.RemovedButOpen); err != nil {
		return in, fmt.Errorf("store: removal summary: %w", err)
	}
	if in.CommitContext, err = db.counts(ctx, `
		SELECT 'intro_' || (metadata->>'intro_commit_kind'), count(*) FROM findings
		WHERE metadata ? 'intro_commit_kind' GROUP BY 1
		UNION ALL
		SELECT 'removal_' || (metadata->>'removal_commit_kind'), count(*) FROM findings
		WHERE metadata ? 'removal_commit_kind' GROUP BY 1
		ORDER BY 1`); err != nil {
		return in, err
	}
	if in.Disclosure, err = db.disclosure(ctx); err != nil {
		return in, err
	}

	if in.DetectionStats, err = db.counts(ctx, `
		SELECT s.key, sum(s.value::bigint)::int
		FROM repositories r, jsonb_each_text(r.scan_stats) s
		GROUP BY 1 ORDER BY 1`); err != nil {
		return in, err
	}
	if err = db.pool.QueryRow(ctx,
		`SELECT count(*) FROM repositories WHERE scan_stats <> '{}'::jsonb`,
	).Scan(&in.DetectionScannedRepo); err != nil {
		return in, fmt.Errorf("store: detection stats coverage: %w", err)
	}
	return in, nil
}

func (db *DB) repoActivity(ctx context.Context) ([]ActivityBucket, error) {
	// Stale is a year without a push: long enough that nobody is watching the
	// repository, which is when a leaked key in it is least likely to be noticed.
	rows, err := db.pool.Query(ctx, `
		SELECT CASE WHEN r.metadata_fetched_at IS NULL THEN 'unknown'
		            WHEN r.archived THEN 'archived'
		            WHEN r.pushed_at IS NULL OR r.pushed_at < now() - interval '365 days' THEN 'stale'
		            ELSE 'active' END,
		       count(DISTINCT r.id), count(f.id), count(f.id) FILTER (WHERE f.reachable_from_head)
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		WHERE f.status = 'open'
		GROUP BY 1 ORDER BY 3 DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: repository activity: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ActivityBucket, error) {
		var b ActivityBucket
		err := r.Scan(&b.Key, &b.Repos, &b.Findings, &b.AtHead)
		return b, err
	})
}

func (db *DB) rulePrecision(ctx context.Context) ([]RuleTriage, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT rule_id, count(*),
		       count(*) FILTER (WHERE status = 'open'),
		       count(*) FILTER (WHERE status = 'rotated'),
		       count(*) FILTER (WHERE status = 'ignored'),
		       count(*) FILTER (WHERE status = 'false_positive')
		FROM findings GROUP BY 1 ORDER BY 2 DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: rule precision: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (RuleTriage, error) {
		var t RuleTriage
		err := r.Scan(&t.Rule, &t.Total, &t.Open, &t.Rotated, &t.Ignored, &t.FalsePos)
		return t, err
	})
}

func (db *DB) reuse(ctx context.Context) (ReuseSummary, error) {
	var s ReuseSummary
	// (fingerprint, repository_id) is unique, so a fingerprint's row count is
	// the number of repositories it was found in.
	if err := db.pool.QueryRow(ctx, `
		SELECT count(*), coalesce(sum(n), 0)
		FROM (SELECT count(*) AS n FROM findings GROUP BY fingerprint HAVING count(*) > 1) x`,
	).Scan(&s.Secrets, &s.Repos); err != nil {
		return s, fmt.Errorf("store: secret reuse: %w", err)
	}
	rows, err := db.pool.Query(ctx, `
		SELECT min(f.masked_preview), max(f.provider_slug), count(*), count(*) FILTER (WHERE r.is_fork)
		FROM findings f JOIN repositories r ON r.id = f.repository_id
		GROUP BY f.fingerprint HAVING count(*) > 1
		ORDER BY 3 DESC, 1 LIMIT 20`)
	if err != nil {
		return s, fmt.Errorf("store: reused secrets: %w", err)
	}
	s.Top, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ReusedSecret, error) {
		var x ReusedSecret
		err := r.Scan(&x.Preview, &x.Provider, &x.Repos, &x.Forks)
		return x, err
	})
	if err != nil {
		return s, fmt.Errorf("store: reused secrets: %w", err)
	}
	return s, nil
}

func (db *DB) keyTypes(ctx context.Context) ([]KeyTypeCount, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT provider_slug, metadata->>'key_type', count(*) FROM findings
		WHERE status = 'open' AND metadata ? 'key_type'
		GROUP BY 1, 2 ORDER BY 3 DESC LIMIT 25`)
	if err != nil {
		return nil, fmt.Errorf("store: key types: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (KeyTypeCount, error) {
		var k KeyTypeCount
		err := r.Scan(&k.Provider, &k.KeyType, &k.Count)
		return k, err
	})
}

func (db *DB) exposure(ctx context.Context) ([]ExposureBucket, error) {
	// A finding gone from HEAD with no recorded removal was either never on the
	// default branch or scanned before removals were tracked; its window is not
	// known, and guessing "until now" would overstate it.
	rows, err := db.pool.Query(ctx, `
		SELECT bucket,
		       count(*) FILTER (WHERE reachable_from_head),
		       count(*) FILTER (WHERE NOT reachable_from_head)
		FROM (
			SELECT reachable_from_head,
			       CASE WHEN NOT reachable_from_head AND removed_at IS NULL THEN 'unknown'
			            WHEN d < interval '1 day'    THEN '1d'
			            WHEN d < interval '7 days'   THEN '7d'
			            WHEN d < interval '30 days'  THEN '30d'
			            WHEN d < interval '90 days'  THEN '90d'
			            WHEN d < interval '365 days' THEN '1y'
			            ELSE 'over_1y' END AS bucket
			FROM (SELECT reachable_from_head, removed_at,
			             coalesce(removed_at, now()) - committed_at AS d
			      FROM findings
			      WHERE status = 'open' AND committed_at IS NOT NULL) f
		) b
		GROUP BY 1`)
	if err != nil {
		return nil, fmt.Errorf("store: exposure windows: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ExposureBucket, error) {
		var b ExposureBucket
		err := r.Scan(&b.Key, &b.AtHead, &b.Removed)
		return b, err
	})
}

func (db *DB) disclosure(ctx context.Context) (DisclosureFunnel, error) {
	// For each finding named in a sent disclosure: its last probe outcome before
	// the mail, its latest after, and the first time a probe after the mail
	// found it dead. Only findings live at disclosure can show a response —
	// one already dead, or never probed, says nothing about the mail.
	var d DisclosureFunnel
	err := db.pool.QueryRow(ctx, `
		WITH d AS (
			SELECT n.repository_id, n.sent_at, x.finding_id
			FROM notifications n CROSS JOIN LATERAL unnest(n.finding_ids) AS x(finding_id)
			WHERE n.state = 'sent' AND n.sent_at IS NOT NULL
		), p AS (
			SELECT d.repository_id, d.sent_at, f.status,
			       (SELECT pr.outcome FROM probe_results pr
			        WHERE pr.finding_id = d.finding_id AND pr.probed_at <= d.sent_at
			        ORDER BY pr.probed_at DESC LIMIT 1) AS before,
			       (SELECT pr.outcome FROM probe_results pr
			        WHERE pr.finding_id = d.finding_id AND pr.probed_at > d.sent_at
			        ORDER BY pr.probed_at DESC LIMIT 1) AS after,
			       (SELECT min(pr.probed_at) FROM probe_results pr
			        WHERE pr.finding_id = d.finding_id AND pr.probed_at > d.sent_at
			          AND pr.outcome = 'dead') AS dead_at
			FROM d JOIN findings f ON f.id = d.finding_id
		)
		SELECT count(DISTINCT repository_id), count(*),
		       count(*) FILTER (WHERE before = 'live'),
		       count(*) FILTER (WHERE before = 'live' AND dead_at IS NOT NULL),
		       count(*) FILTER (WHERE before = 'live' AND after = 'live'),
		       count(*) FILTER (WHERE status = 'rotated'),
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM dead_at - sent_at) / 86400)
		           FILTER (WHERE before = 'live' AND dead_at IS NOT NULL)
		FROM p`,
	).Scan(&d.Repos, &d.Findings, &d.LiveAtDisclosure, &d.WentDead, &d.StillLive, &d.Rotated, &d.MedianDaysToDead)
	if err != nil {
		return d, fmt.Errorf("store: disclosure funnel: %w", err)
	}
	return d, nil
}
