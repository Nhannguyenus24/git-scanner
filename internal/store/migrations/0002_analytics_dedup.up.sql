-- Exactly-once bookkeeping for the analytics rollups (plan.md §10).
--
-- The rollup columns are counters, and a counter is not idempotent: a Kafka
-- batch that is replayed after a crash between the handler and the offset
-- commit would add the same findings twice. These two tables make the replay a
-- no-op instead.

-- Which findings have already been counted. The consumer inserts the batch's
-- ids first and aggregates only the rows the insert actually claimed, so a
-- redelivered record contributes nothing.
CREATE TABLE analytics_seen_findings (
    finding_id UUID PRIMARY KEY,
    seen_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which repositories have already been counted into a given rollup bucket.
-- `analytics_rollups.repos` is a distinct count, so it may only advance the
-- first time a repository appears under that dimension on that date.
CREATE TABLE analytics_seen_repos (
    bucket_date   DATE NOT NULL,
    dimension     TEXT NOT NULL,
    dimension_key TEXT NOT NULL,
    repository_id UUID NOT NULL,
    PRIMARY KEY (bucket_date, dimension, dimension_key, repository_id)
);
