-- Operator settings, one row. The disclosure switch starts off: mail reaches
-- strangers' inboxes, so sending is something an operator turns on, never a
-- side effect of a deploy (plan.md §0.3).
CREATE TABLE app_settings (
    id                    BOOLEAN     PRIMARY KEY DEFAULT true CHECK (id),
    notifications_enabled BOOLEAN     NOT NULL DEFAULT false,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO app_settings (id) VALUES (true);

-- The repository list shows each repository's latest disclosure.
CREATE INDEX notifications_repo_latest_idx ON notifications (repository_id, created_at DESC);
