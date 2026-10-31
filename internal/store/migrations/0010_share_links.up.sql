-- Read-only links to one repository's results, for sending to its owner or to
-- whoever must act on them. The link carries a random token; only its SHA-256
-- is stored, so the table cannot be turned back into working links. Every link
-- expires, can be revoked, and counts its views, because a report that points at
-- a credential in public history is useful to an attacker too.
CREATE TABLE share_links (
    id               UUID        PRIMARY KEY,
    repository_id    UUID        NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    token_hash       BYTEA       NOT NULL UNIQUE,
    -- summary: counts only, no locations. full: locations and commit links, the
    -- same facts the disclosure mail gives the owner.
    detail           TEXT        NOT NULL CHECK (detail IN ('summary', 'full')),
    include_resolved BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    revoked_at       TIMESTAMPTZ,
    views            INTEGER     NOT NULL DEFAULT 0,
    last_viewed_at   TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE INDEX share_links_repository_idx ON share_links (repository_id, created_at DESC);
