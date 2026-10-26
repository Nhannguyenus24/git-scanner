-- The random target samples arbitrary public repositories to exercise the
-- detector while tuning precision (see entity.TargetRandom). It is deliberately
-- not a bounded, authorized set, so it is admitted only by widening the kind
-- constraint; nothing else about targets changes.
ALTER TABLE targets DROP CONSTRAINT targets_kind_check;
ALTER TABLE targets ADD CONSTRAINT targets_kind_check
    CHECK (kind IN ('repo', 'user', 'org', 'allowlist', 'random'));
