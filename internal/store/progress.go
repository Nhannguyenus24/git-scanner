package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Notify sends payload on a LISTEN/NOTIFY channel. Delivery is at-most-once
// and only to sessions listening at that moment, which is what live progress
// wants and nothing else should rely on.
func (db *DB) Notify(ctx context.Context, channel, payload string) error {
	if _, err := db.pool.Exec(ctx, `SELECT pg_notify($1, $2)`, channel, payload); err != nil {
		return fmt.Errorf("store: notify %s: %w", channel, err)
	}
	return nil
}

// Listen holds one pooled connection on channel and calls fn with each payload
// until ctx ends or the connection fails. The connection is taken out of the
// pool for as long as it listens, so a service runs one listener, not one per
// client.
func (db *DB) Listen(ctx context.Context, channel string, fn func(payload string)) error {
	pooled, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("store: listen %s: %w", channel, err)
	}
	// Taken out of the pool for good: a session left subscribed would go on
	// receiving, and buffering, every event for whichever caller got it next.
	conn := pooled.Hijack()
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
		return fmt.Errorf("store: listen %s: %w", channel, err)
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("store: listen %s: %w", channel, err)
		}
		fn(n.Payload)
	}
}
