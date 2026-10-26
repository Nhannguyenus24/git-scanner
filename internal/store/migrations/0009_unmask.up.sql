-- Detection testing against the operator's own repositories: when an owner is
-- on unmask_owners, scan-worker keeps each credential it finds there, sealed
-- with a key that lives in configuration (internal/secrets.Sealer). The api
-- shows the value only while masking is switched off. Repositories of anyone
-- not on the list are never stored this way.

ALTER TABLE app_settings ADD COLUMN masking_enabled BOOLEAN NOT NULL DEFAULT true;

CREATE TABLE unmask_owners (
    owner    TEXT        PRIMARY KEY CHECK (owner = lower(owner)),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Kept apart from findings so the findings table stays secret-free, and keyed
-- by (repository, fingerprint) because a rescan keeps the original finding row.
-- expires_at bounds how long a plaintext-recoverable value exists.
CREATE TABLE sealed_secrets (
    repository_id UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    fingerprint   TEXT        NOT NULL,
    sealed        BYTEA       NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (repository_id, fingerprint)
);
