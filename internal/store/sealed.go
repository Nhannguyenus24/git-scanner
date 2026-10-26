package store

import (
	"context"
	"fmt"
	"time"

	"github.com/mservice/gitscanner/libs/entity"
)

// SealedTTL bounds how long a recoverable credential is kept. A test of the
// detector needs the value for days, not indefinitely.
const SealedTTL = 30 * 24 * time.Hour

// SealedAAD is the additional data a value is sealed under: its repository and
// fingerprint, so a ciphertext moved to another row fails to open.
func SealedAAD(repoID entity.ID, fingerprint string) []byte {
	return []byte(repoID.String() + "/" + fingerprint)
}

// KeepSealedSecret stores one sealed credential, replacing an older seal of the
// same value, and sweeps this repository's expired ones while it is there.
func (db *DB) KeepSealedSecret(ctx context.Context, repoID entity.ID, fingerprint string, sealed []byte) error {
	_, err := db.pool.Exec(ctx, `
		WITH swept AS (
			DELETE FROM sealed_secrets WHERE repository_id = $1 AND expires_at < now()
		)
		INSERT INTO sealed_secrets (repository_id, fingerprint, sealed, expires_at)
		VALUES ($1, $2, $3, now() + $4 * interval '1 second')
		ON CONFLICT (repository_id, fingerprint) DO UPDATE SET
			sealed = EXCLUDED.sealed, created_at = now(), expires_at = EXCLUDED.expires_at`,
		repoID, fingerprint, sealed, int64(SealedTTL.Seconds()))
	if err != nil {
		return fmt.Errorf("store: keep sealed secret: %w", err)
	}
	return nil
}
