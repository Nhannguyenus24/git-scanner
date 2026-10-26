ALTER TABLE targets DROP CONSTRAINT targets_kind_check;
ALTER TABLE targets ADD CONSTRAINT targets_kind_check
    CHECK (kind IN ('repo', 'user', 'org', 'allowlist'));
