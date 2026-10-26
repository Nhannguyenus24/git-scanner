package store

import (
	"context"
	"fmt"
)

// NotificationsEnabled reports whether the operator has turned disclosure mail
// on. Both the api and the notifier ask, so a request queued before the switch
// was turned off is not sent after it.
func (db *DB) NotificationsEnabled(ctx context.Context) (bool, error) {
	var on bool
	if err := db.pool.QueryRow(ctx,
		`SELECT notifications_enabled FROM app_settings`).Scan(&on); err != nil {
		return false, fmt.Errorf("store: notifications enabled: %w", err)
	}
	return on, nil
}

// SetNotificationsEnabled flips the disclosure switch.
func (db *DB) SetNotificationsEnabled(ctx context.Context, on bool) error {
	if _, err := db.pool.Exec(ctx,
		`UPDATE app_settings SET notifications_enabled = $1, updated_at = now()`, on); err != nil {
		return fmt.Errorf("store: set notifications enabled: %w", err)
	}
	return nil
}
