-- Repository attributes from the GitHub repository object. The user and org
-- listings carry them already; the random sample and direct repo scans do not,
-- so the scan worker fetches them once per repository. metadata_fetched_at
-- records that it has, and is what stops it asking again on every rescan.
ALTER TABLE repositories
    ADD COLUMN language            TEXT        NOT NULL DEFAULT '',
    ADD COLUMN topics              TEXT[]      NOT NULL DEFAULT '{}',
    ADD COLUMN owner_type          TEXT        NOT NULL DEFAULT '',
    ADD COLUMN archived            BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN size_kb             INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN pushed_at           TIMESTAMPTZ,
    ADD COLUMN metadata_fetched_at TIMESTAMPTZ,
    -- Detection counters from the latest scan: blobs skipped and candidates each
    -- filter dropped. One row per repository, overwritten each scan; the totals
    -- across repositories are what tuning reads.
    ADD COLUMN scan_stats          JSONB       NOT NULL DEFAULT '{}';

-- The commit that took a credential out of the default branch, for a finding no
-- longer reachable from HEAD. Empty while it is still there, or when it was
-- never on the default branch at all.
ALTER TABLE findings
    ADD COLUMN removed_commit_sha TEXT NOT NULL DEFAULT '',
    ADD COLUMN removed_at         TIMESTAMPTZ;

-- Secret reuse is a GROUP BY fingerprint across repositories.
CREATE INDEX findings_fingerprint_idx ON findings (fingerprint);

-- Every liveness probe, not just the latest. A finding's metadata holds the
-- current outcome; this keeps the history, which is what shows whether a
-- credential went dead after its owner was told about it. One row per finding
-- per job, so a redelivered scan overwrites rather than duplicates.
CREATE TABLE probe_results (
    finding_id  UUID        NOT NULL REFERENCES findings (id) ON DELETE CASCADE,
    scan_job_id UUID        NOT NULL REFERENCES scan_jobs (id) ON DELETE CASCADE,
    outcome     TEXT        NOT NULL,
    probed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (finding_id, scan_job_id)
);
CREATE INDEX probe_results_finding_idx ON probe_results (finding_id, probed_at);
