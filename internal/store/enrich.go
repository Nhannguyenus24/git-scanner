package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// GetFinding loads one finding by id.
func (db *DB) GetFinding(ctx context.Context, findingID entity.ID) (entity.Finding, error) {
	const q = `
		SELECT id, scan_job_id, repository_id, rule_id, provider_slug,
		       fingerprint, masked_preview, blob_oid, commit_sha, file_path, line_number,
		       author_name, author_email, committed_at, reachable_from_head,
		       severity, status, metadata
		FROM findings WHERE id = $1`

	f, err := scanFinding(db.pool.QueryRow(ctx, q, findingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Finding{}, ErrNotFound
	}
	if err != nil {
		return entity.Finding{}, err
	}
	return f, nil
}

// GetProviderBySlug loads one catalog entry.
func (db *DB) GetProviderBySlug(ctx context.Context, slug string) (entity.Provider, error) {
	const q = `
		SELECT id, slug, name, category, current_version, created_at, updated_at
		FROM providers WHERE slug = $1`

	var p entity.Provider
	err := db.pool.QueryRow(ctx, q, slug).Scan(
		&p.ID, &p.Slug, &p.Name, &p.Category, &p.CurrentVersion, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Provider{}, ErrNotFound
	}
	if err != nil {
		return entity.Provider{}, fmt.Errorf("store: get provider %q: %w", slug, err)
	}
	return p, nil
}

// UpdateFindingEnrichment writes back what the enricher resolved.
//
// It touches only the enrichment columns. The detection columns are what the
// scan recorded and must not be rewritten here, so a replayed enrich task
// converges on the same row rather than editing history.
func (db *DB) UpdateFindingEnrichment(
	ctx context.Context, findingID entity.ID, slug string, sev entity.Severity, meta map[string]string,
) error {
	if meta == nil {
		meta = map[string]string{}
	}
	_, err := db.pool.Exec(ctx, `
		UPDATE findings SET provider_slug = $2, severity = $3, metadata = $4
		WHERE id = $1`, findingID, slug, sev.String(), meta)
	if err != nil {
		return fmt.Errorf("store: update finding enrichment: %w", err)
	}
	return nil
}

// UpsertProviderCandidate records an unmapped credential shape, or folds
// another sighting into the one already recorded (plan.md §5.4).
//
// The array unions happen in SQL rather than read-modify-write in Go: two
// enricher replicas seeing the same signature at once would otherwise each
// write the set they read, and one of the two new env var names would be lost.
func (db *DB) UpsertProviderCandidate(ctx context.Context, c entity.ProviderCandidate) error {
	if c.Signature == "" {
		return errors.New("store: provider candidate needs a signature")
	}
	if c.ID == "" {
		c.ID = id.New()
	}
	if c.EnvVarNames == nil {
		c.EnvVarNames = []string{}
	}
	if c.NearbyHosts == nil {
		c.NearbyHosts = []string{}
	}
	if c.Occurrences <= 0 {
		c.Occurrences = 1
	}

	// repo_count is left at its default: counting distinct repositories per
	// candidate needs a seen-set table this phase does not have, and a number
	// derived from occurrences would simply be wrong.
	const q = `
		INSERT INTO provider_candidates (
			id, signature, guessed_name, env_var_names, nearby_hosts,
			sample_masked, charset_class, length_min, length_max, occurrences)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (signature) DO UPDATE SET
			occurrences   = provider_candidates.occurrences + EXCLUDED.occurrences,
			last_seen_at  = now(),
			guessed_name  = CASE WHEN provider_candidates.guessed_name = ''
			                     THEN EXCLUDED.guessed_name
			                     ELSE provider_candidates.guessed_name END,
			sample_masked = CASE WHEN provider_candidates.sample_masked = ''
			                     THEN EXCLUDED.sample_masked
			                     ELSE provider_candidates.sample_masked END,
			length_min    = least(provider_candidates.length_min, EXCLUDED.length_min),
			length_max    = greatest(provider_candidates.length_max, EXCLUDED.length_max),
			env_var_names = ARRAY(SELECT DISTINCT unnest(
			                     provider_candidates.env_var_names || EXCLUDED.env_var_names)),
			nearby_hosts  = ARRAY(SELECT DISTINCT unnest(
			                     provider_candidates.nearby_hosts || EXCLUDED.nearby_hosts))`

	_, err := db.pool.Exec(ctx, q,
		c.ID, c.Signature, c.GuessedName, c.EnvVarNames, c.NearbyHosts,
		c.SampleMasked, c.CharsetClass, c.LengthMin, c.LengthMax, c.Occurrences)
	if err != nil {
		return fmt.Errorf("store: upsert provider candidate: %w", err)
	}
	return nil
}
