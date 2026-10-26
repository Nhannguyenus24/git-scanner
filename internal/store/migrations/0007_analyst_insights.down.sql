DROP TABLE IF EXISTS probe_results;
DROP INDEX IF EXISTS findings_fingerprint_idx;
ALTER TABLE findings
    DROP COLUMN IF EXISTS removed_commit_sha,
    DROP COLUMN IF EXISTS removed_at;
ALTER TABLE repositories
    DROP COLUMN IF EXISTS language,
    DROP COLUMN IF EXISTS topics,
    DROP COLUMN IF EXISTS owner_type,
    DROP COLUMN IF EXISTS archived,
    DROP COLUMN IF EXISTS size_kb,
    DROP COLUMN IF EXISTS pushed_at,
    DROP COLUMN IF EXISTS metadata_fetched_at,
    DROP COLUMN IF EXISTS scan_stats;
