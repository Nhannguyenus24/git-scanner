-- Initial schema (plan.md §7).

CREATE TABLE targets (
    id          UUID PRIMARY KEY,
    kind        TEXT        NOT NULL CHECK (kind IN ('repo', 'user', 'org', 'allowlist')),
    value       TEXT        NOT NULL,
    allowlist   TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE scan_jobs (
    id              UUID PRIMARY KEY,
    target_id       UUID        NOT NULL REFERENCES targets (id) ON DELETE CASCADE,
    state           TEXT        NOT NULL CHECK (state IN
                                  ('queued', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    cursor          TEXT        NOT NULL DEFAULT '',
    repos_total     INTEGER     NOT NULL DEFAULT 0,
    repos_scanned   INTEGER     NOT NULL DEFAULT 0,
    blobs_scanned   BIGINT      NOT NULL DEFAULT 0,
    findings_count  INTEGER     NOT NULL DEFAULT 0,
    registry_ver    BIGINT      NOT NULL DEFAULT 0,
    error           TEXT        NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index: the scheduler and the stuck-job sweeper only ever look at
-- non-terminal jobs, which stay a small fraction of the table.
CREATE INDEX scan_jobs_active_idx ON scan_jobs (state, updated_at)
    WHERE state IN ('queued', 'running', 'paused');

CREATE TABLE repositories (
    id              UUID PRIMARY KEY,
    target_id       UUID        NOT NULL REFERENCES targets (id) ON DELETE CASCADE,
    owner           TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    clone_url       TEXT        NOT NULL,
    default_branch  TEXT        NOT NULL DEFAULT 'main',
    is_fork         BOOLEAN     NOT NULL DEFAULT false,
    is_private      BOOLEAN     NOT NULL DEFAULT false,
    stars           INTEGER     NOT NULL DEFAULT 0,
    app_type        TEXT        NOT NULL DEFAULT '',
    head_sha        TEXT        NOT NULL DEFAULT '',
    last_scanned_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner, name)
);

-- Provider catalog (plan.md §5).
CREATE TABLE providers (
    id              UUID PRIMARY KEY,
    slug            TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    category        TEXT        NOT NULL,
    current_version INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE provider_versions (
    id            UUID PRIMARY KEY,
    provider_id   UUID        NOT NULL REFERENCES providers (id) ON DELETE CASCADE,
    version       INTEGER     NOT NULL,
    state         TEXT        NOT NULL CHECK (state IN ('draft', 'published', 'deprecated')),
    definition    JSONB       NOT NULL,
    -- Hashed separately so publishing can tell a reclassification from a change
    -- that needs blobs walked again (plan.md §5.7).
    detect_hash   TEXT        NOT NULL,
    classify_hash TEXT        NOT NULL,
    author        TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    UNIQUE (provider_id, version)
);

-- Only one version per provider may be published at a time.
CREATE UNIQUE INDEX provider_versions_one_published_idx
    ON provider_versions (provider_id)
    WHERE state = 'published';

CREATE TABLE provider_fixtures (
    id              UUID PRIMARY KEY,
    provider_id     UUID        NOT NULL REFERENCES providers (id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    -- Synthetic credentials only. A real leaked value must never be copied here
    -- from findings: this table is dumped into CI logs on failure.
    input           TEXT        NOT NULL,
    should_match    BOOLEAN     NOT NULL,
    expected_fields JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_id, name)
);

CREATE TABLE findings (
    id                  UUID PRIMARY KEY,
    scan_job_id         UUID        NOT NULL REFERENCES scan_jobs (id) ON DELETE CASCADE,
    repository_id       UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    rule_id             TEXT        NOT NULL,
    provider_slug       TEXT        NOT NULL DEFAULT '',

    -- HMAC-SHA256 of the secret. The plaintext is never stored (plan.md §6).
    fingerprint         TEXT        NOT NULL,
    masked_preview      TEXT        NOT NULL,

    blob_oid            TEXT        NOT NULL,
    commit_sha          TEXT        NOT NULL,
    file_path           TEXT        NOT NULL,
    line_number         INTEGER     NOT NULL DEFAULT 0,

    author_name         TEXT        NOT NULL DEFAULT '',
    author_email        TEXT        NOT NULL DEFAULT '',
    committed_at        TIMESTAMPTZ,
    reachable_from_head BOOLEAN     NOT NULL DEFAULT false,

    severity            TEXT        NOT NULL CHECK (severity IN
                                      ('info', 'low', 'medium', 'high', 'critical')),
    status              TEXT        NOT NULL DEFAULT 'open' CHECK (status IN
                                      ('open', 'rotated', 'false_positive', 'ignored')),
    metadata            JSONB       NOT NULL DEFAULT '{}',
    detected_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- The idempotency key. A redelivered RabbitMQ task re-inserts the same rows,
    -- and ON CONFLICT DO NOTHING makes that a no-op (plan.md §6, §13.3).
    UNIQUE (fingerprint, repository_id)
);

CREATE INDEX findings_repo_idx     ON findings (repository_id, detected_at DESC);
CREATE INDEX findings_provider_idx ON findings (provider_slug, severity)
    WHERE status = 'open';
CREATE INDEX findings_job_idx      ON findings (scan_job_id);

-- The unmapped backlog (plan.md §5.4).
CREATE TABLE provider_candidates (
    id               UUID PRIMARY KEY,
    signature        TEXT        NOT NULL UNIQUE,
    guessed_name     TEXT        NOT NULL DEFAULT '',
    env_var_names    TEXT[]      NOT NULL DEFAULT '{}',
    nearby_hosts     TEXT[]      NOT NULL DEFAULT '{}',
    sample_masked    TEXT        NOT NULL DEFAULT '',
    charset_class    TEXT        NOT NULL DEFAULT '',
    length_min       INTEGER     NOT NULL DEFAULT 0,
    length_max       INTEGER     NOT NULL DEFAULT 0,
    occurrences      BIGINT      NOT NULL DEFAULT 0,
    repo_count       INTEGER     NOT NULL DEFAULT 0,
    status           TEXT        NOT NULL DEFAULT 'new' CHECK (status IN
                                   ('new', 'triaged', 'mapped', 'ignored')),
    promoted_to_slug TEXT        NOT NULL DEFAULT '',
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The backlog is read ranked by impact, so the index carries the ordering.
CREATE INDEX provider_candidates_backlog_idx
    ON provider_candidates (occurrences DESC, last_seen_at DESC)
    WHERE status IN ('new', 'triaged');

CREATE TABLE notifications (
    id            UUID PRIMARY KEY,
    repository_id UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    recipient     TEXT        NOT NULL,
    finding_ids   UUID[]      NOT NULL DEFAULT '{}',
    state         TEXT        NOT NULL DEFAULT 'pending' CHECK (state IN
                                ('pending', 'sent', 'failed', 'suppressed')),
    error         TEXT        NOT NULL DEFAULT '',
    sent_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One disclosure per repository (plan.md §0.3). The constraint, not just the
-- application, enforces it: a notifier replica that loses the Redis race still
-- cannot send a duplicate.
CREATE UNIQUE INDEX notifications_one_per_repo_idx
    ON notifications (repository_id)
    WHERE state IN ('pending', 'sent');

CREATE TABLE suppressions (
    id         UUID PRIMARY KEY,
    -- Either an email address or a domain; both are matched before sending.
    pattern    TEXT        NOT NULL UNIQUE,
    reason     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE analytics_rollups (
    bucket_date   DATE    NOT NULL,
    dimension     TEXT    NOT NULL,  -- provider | severity | rule | app_type
    dimension_key TEXT    NOT NULL,
    findings      BIGINT  NOT NULL DEFAULT 0,
    repos         BIGINT  NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_date, dimension, dimension_key)
);

-- Global registry version. A single row whose bump signals workers to reload
-- (plan.md §5.6).
CREATE TABLE registry_state (
    id      BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    version BIGINT  NOT NULL DEFAULT 0
);

INSERT INTO registry_state (id, version) VALUES (true, 0);
