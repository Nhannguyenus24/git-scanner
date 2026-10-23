-- When the repository was created on its host, as distinct from created_at,
-- which records when this system first discovered it. Scan logs and triage both
-- want the former: a credential in a repository created years ago has had that
-- much longer to be found by someone else.
ALTER TABLE repositories ADD COLUMN remote_created_at TIMESTAMPTZ;
