package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
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

// Settings is every operator switch, for the settings page.
type Settings struct {
	NotificationsEnabled bool
	MaskingEnabled       bool
	UnmaskOwners         []string
}

// GetSettings loads every operator switch in one round trip.
func (db *DB) GetSettings(ctx context.Context) (Settings, error) {
	var s Settings
	err := db.pool.QueryRow(ctx, `
		SELECT notifications_enabled, masking_enabled,
		       coalesce((SELECT array_agg(owner ORDER BY owner) FROM unmask_owners), '{}')
		FROM app_settings`).Scan(&s.NotificationsEnabled, &s.MaskingEnabled, &s.UnmaskOwners)
	if err != nil {
		return Settings{}, fmt.Errorf("store: get settings: %w", err)
	}
	return s, nil
}

// SetMaskingEnabled flips whether the api may show sealed credentials.
func (db *DB) SetMaskingEnabled(ctx context.Context, on bool) error {
	if _, err := db.pool.Exec(ctx,
		`UPDATE app_settings SET masking_enabled = $1, updated_at = now()`, on); err != nil {
		return fmt.Errorf("store: set masking enabled: %w", err)
	}
	return nil
}

// AddUnmaskOwner puts a GitHub owner on the unmask list. Owners compare
// case-insensitively, as GitHub logins do.
func (db *DB) AddUnmaskOwner(ctx context.Context, owner string) error {
	if _, err := db.pool.Exec(ctx,
		`INSERT INTO unmask_owners (owner) VALUES (lower($1)) ON CONFLICT DO NOTHING`, owner); err != nil {
		return fmt.Errorf("store: add unmask owner: %w", err)
	}
	return nil
}

// RemoveUnmaskOwner takes an owner off the list and destroys every value
// sealed for their repositories: removing someone means their credentials are
// no longer kept, not merely no longer shown.
func (db *DB) RemoveUnmaskOwner(ctx context.Context, owner string) error {
	return db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM unmask_owners WHERE owner = lower($1)`, owner); err != nil {
			return fmt.Errorf("store: remove unmask owner: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM sealed_secrets s USING repositories r
			WHERE r.id = s.repository_id AND lower(r.owner) = lower($1)`, owner); err != nil {
			return fmt.Errorf("store: drop sealed secrets: %w", err)
		}
		return nil
	})
}

// IsUnmaskOwner reports whether scan-worker should keep the credentials found
// in this owner's repositories.
func (db *DB) IsUnmaskOwner(ctx context.Context, owner string) (bool, error) {
	var ok bool
	if err := db.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM unmask_owners WHERE owner = lower($1))`, owner).Scan(&ok); err != nil {
		return false, fmt.Errorf("store: is unmask owner: %w", err)
	}
	return ok, nil
}
