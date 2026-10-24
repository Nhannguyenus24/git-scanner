package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
)

// Rollup dimension names. The dimension column is free text, so these constants
// are what keeps the writer and the dashboard queries agreeing on its values.
const (
	DimensionProvider = "provider"
	DimensionSeverity = "severity"
	DimensionRule     = "rule"
	DimensionAppType  = "app_type"
)

// RollupInput is one finding's contribution to the analytics rollups.
type RollupInput struct {
	FindingID    entity.ID
	RepositoryID entity.ID

	// BucketDate is the day this finding counts under. Only its UTC calendar
	// date is used.
	BucketDate time.Time

	// Dimensions maps a dimension name to this finding's key under it, e.g.
	// {"provider": "aws", "severity": "high"}.
	Dimensions map[string]string
}

// bucketKey identifies one analytics_rollups row.
type bucketKey struct {
	date      string // YYYY-MM-DD
	dimension string
	key       string
}

// repoClaim is one (bucket, repository) pair whose distinct-count contribution
// has not been claimed yet.
type repoClaim struct {
	bucketKey
	repo string
}

// RecordFindingRollups folds a batch of findings into analytics_rollups,
// counting each finding exactly once however often it is delivered.
//
// Delivery is at-least-once: the consumer acks only after its handler succeeds,
// so a crash in between redelivers the event. Because the rollup columns are
// counters, a plain `findings + n` would double-count that redelivery. Both
// dedupe tables exist to prevent it: the batch first *claims* its finding ids,
// and only the ids this call actually won are aggregated. The same trick guards
// repos, which is a distinct count and so may only advance the first time a
// repository appears in a bucket.
//
// Claim and counter bump share one transaction. Split across two, a crash
// between them would leave the ids claimed and the counters never incremented,
// and the lost findings could never be recovered — the replay would find them
// already claimed.
func (db *DB) RecordFindingRollups(ctx context.Context, batch []RollupInput) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	// Collapse duplicates inside the batch. ON CONFLICT DO NOTHING would absorb
	// them too, but the rollup upsert below cannot touch one row twice in a
	// single statement, so the aggregation has to be exact before it runs.
	byFinding := make(map[entity.ID]RollupInput, len(batch))
	ids := make([]string, 0, len(batch))
	for _, in := range batch {
		if in.FindingID == "" {
			continue
		}
		if _, dup := byFinding[in.FindingID]; dup {
			continue
		}
		byFinding[in.FindingID] = in
		ids = append(ids, in.FindingID.String())
	}
	if len(ids) == 0 {
		return 0, nil
	}

	var counted int
	err := db.InTx(ctx, func(tx pgx.Tx) error {
		claimed, err := claimFindings(ctx, tx, ids)
		if err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil // the whole batch is a replay
		}

		findings := map[bucketKey]int64{}
		var claims []repoClaim
		seen := map[repoClaim]struct{}{}

		for id := range claimed {
			in := byFinding[id]
			date := in.BucketDate.UTC().Format(time.DateOnly)

			for dim, key := range in.Dimensions {
				bk := bucketKey{date: date, dimension: dim, key: key}
				findings[bk]++

				if in.RepositoryID == "" {
					continue
				}
				rc := repoClaim{bucketKey: bk, repo: in.RepositoryID.String()}
				if _, dup := seen[rc]; dup {
					continue
				}
				seen[rc] = struct{}{}
				claims = append(claims, rc)
			}
		}

		repos, err := claimRepos(ctx, tx, claims)
		if err != nil {
			return err
		}
		if err := upsertRollups(ctx, tx, findings, repos); err != nil {
			return err
		}

		counted = len(claimed)
		return nil
	})
	return counted, err
}

// claimFindings inserts the batch's ids and returns the subset this call won,
// which is exactly the set of findings not counted by an earlier delivery.
func claimFindings(ctx context.Context, tx pgx.Tx, ids []string) (map[entity.ID]struct{}, error) {
	rows, err := tx.Query(ctx, `
		INSERT INTO analytics_seen_findings (finding_id)
		SELECT unnest($1::uuid[])
		ON CONFLICT DO NOTHING
		RETURNING finding_id`, ids)
	if err != nil {
		return nil, fmt.Errorf("store: claim findings: %w", err)
	}
	defer rows.Close()

	out := make(map[entity.ID]struct{}, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan claimed finding: %w", err)
		}
		out[entity.ID(id)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: claim findings: %w", err)
	}
	return out, nil
}

// claimRepos claims each (bucket, repository) pair and reports how many were
// new per bucket, which is how much the distinct repo count may grow.
func claimRepos(ctx context.Context, tx pgx.Tx, claims []repoClaim) (map[bucketKey]int64, error) {
	out := map[bucketKey]int64{}
	if len(claims) == 0 {
		return out, nil
	}

	dates := make([]string, len(claims))
	dims := make([]string, len(claims))
	keys := make([]string, len(claims))
	repos := make([]string, len(claims))
	for i, c := range claims {
		dates[i], dims[i], keys[i], repos[i] = c.date, c.dimension, c.key, c.repo
	}

	// to_char rather than a bare column so the returned date is parsed back
	// under this function's format regardless of the session's DateStyle.
	rows, err := tx.Query(ctx, `
		INSERT INTO analytics_seen_repos (bucket_date, dimension, dimension_key, repository_id)
		SELECT * FROM unnest($1::date[], $2::text[], $3::text[], $4::uuid[])
		ON CONFLICT DO NOTHING
		RETURNING to_char(bucket_date, 'YYYY-MM-DD'), dimension, dimension_key`,
		dates, dims, keys, repos)
	if err != nil {
		return nil, fmt.Errorf("store: claim rollup repos: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var date, dim, key string
		if err := rows.Scan(&date, &dim, &key); err != nil {
			return nil, fmt.Errorf("store: scan claimed repo: %w", err)
		}
		out[bucketKey{date: date, dimension: dim, key: key}]++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: claim rollup repos: %w", err)
	}
	return out, nil
}

// upsertRollups applies the per-bucket increments. Each bucket appears once in
// the statement, which ON CONFLICT DO UPDATE requires: Postgres refuses to let
// one command affect the same row twice.
func upsertRollups(ctx context.Context, tx pgx.Tx, findings, repos map[bucketKey]int64) error {
	buckets := make(map[bucketKey]struct{}, len(findings)+len(repos))
	for b := range findings {
		buckets[b] = struct{}{}
	}
	for b := range repos {
		buckets[b] = struct{}{}
	}
	if len(buckets) == 0 {
		return nil
	}

	n := len(buckets)
	dates := make([]string, 0, n)
	dims := make([]string, 0, n)
	keys := make([]string, 0, n)
	fCounts := make([]int64, 0, n)
	rCounts := make([]int64, 0, n)
	for b := range buckets {
		dates = append(dates, b.date)
		dims = append(dims, b.dimension)
		keys = append(keys, b.key)
		fCounts = append(fCounts, findings[b])
		rCounts = append(rCounts, repos[b])
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO analytics_rollups (bucket_date, dimension, dimension_key, findings, repos)
		SELECT * FROM unnest($1::date[], $2::text[], $3::text[], $4::bigint[], $5::bigint[])
		ON CONFLICT (bucket_date, dimension, dimension_key) DO UPDATE SET
			findings = analytics_rollups.findings + EXCLUDED.findings,
			repos    = analytics_rollups.repos    + EXCLUDED.repos`,
		dates, dims, keys, fCounts, rCounts)
	if err != nil {
		return fmt.Errorf("store: upsert rollups: %w", err)
	}
	return nil
}
