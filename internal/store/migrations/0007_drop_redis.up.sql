-- State that lived in Redis, moved here so the system runs on Postgres and
-- RabbitMQ alone.

-- Set by the api, read by discovery between pages (plan.md §8). A column rather
-- than a state, because the job keeps running until a worker observes the
-- request; the state only becomes 'paused' then.
ALTER TABLE scan_jobs ADD COLUMN pause_requested BOOLEAN NOT NULL DEFAULT false;

-- Token buckets shared by every replica (plan.md §13.5). One row per bucket;
-- Take locks the row, so the read, refill and decrement are one atomic step
-- across processes, as the Lua script made them in Redis.
CREATE TABLE rate_buckets (
    key        TEXT             PRIMARY KEY,
    tokens     DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ      NOT NULL
);
