package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mservice/gitscanner/libs/entity"
)

const shareColumns = `id, repository_id, detail, include_resolved, created_at, expires_at, revoked_at, views, last_viewed_at`

func scanShare(row pgx.Row) (entity.ShareLink, error) {
	var l entity.ShareLink
	var detail string
	err := row.Scan(&l.ID, &l.RepositoryID, &detail, &l.IncludeResolved,
		&l.CreatedAt, &l.ExpiresAt, &l.RevokedAt, &l.Views, &l.LastViewedAt)
	l.Detail = entity.ShareDetail(detail)
	return l, err
}

// CreateShareLink stores a link under the hash of its token. The token itself
// is never passed here.
func (db *DB) CreateShareLink(ctx context.Context, l entity.ShareLink, tokenHash []byte) error {
	if err := l.Validate(); err != nil {
		return err
	}
	_, err := db.pool.Exec(ctx, `
		INSERT INTO share_links (id, repository_id, token_hash, detail, include_resolved, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		l.ID, l.RepositoryID, tokenHash, string(l.Detail), l.IncludeResolved, l.CreatedAt, l.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store: create share link: %w", err)
	}
	return nil
}

// ListShareLinks returns a repository's links, newest first, whatever their state.
func (db *DB) ListShareLinks(ctx context.Context, repoID entity.ID) ([]entity.ShareLink, error) {
	rows, err := db.pool.Query(ctx,
		"SELECT "+shareColumns+" FROM share_links WHERE repository_id = $1 ORDER BY created_at DESC", repoID)
	if err != nil {
		return nil, fmt.Errorf("store: list share links: %w", err)
	}
	defer rows.Close()
	var out []entity.ShareLink
	for rows.Next() {
		l, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list share links: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// RevokeShareLink ends a link now. Revoking twice keeps the first time.
func (db *DB) RevokeShareLink(ctx context.Context, id entity.ID) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE share_links SET revoked_at = COALESCE(revoked_at, now()) WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: revoke share link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// OpenShareLink finds the active link with this token hash and counts the view.
// A revoked, expired or unknown token is ErrNotFound alike, so a caller cannot
// tell a link that never existed from one that ended.
func (db *DB) OpenShareLink(ctx context.Context, tokenHash []byte, now time.Time) (entity.ShareLink, error) {
	l, err := scanShare(db.pool.QueryRow(ctx, `
		UPDATE share_links SET views = views + 1, last_viewed_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING `+shareColumns, tokenHash, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ShareLink{}, ErrNotFound
	}
	if err != nil {
		return entity.ShareLink{}, fmt.Errorf("store: open share link: %w", err)
	}
	return l, nil
}

// ListFindingsByRepository returns every finding for a repository, whatever its
// status, most severe first.
func (db *DB) ListFindingsByRepository(ctx context.Context, repoID entity.ID) ([]entity.Finding, error) {
	rows, err := db.pool.Query(ctx,
		"SELECT "+findingColumns+" FROM findings WHERE repository_id = $1 ORDER BY detected_at DESC", repoID)
	if err != nil {
		return nil, fmt.Errorf("store: list findings: %w", err)
	}
	defer rows.Close()
	return collectFindings(rows)
}

// RepositoryAuthorized reports whether a repository belongs to a target the
// operator chose — a repository, user, org or allowlist — rather than only to a
// random sample. Sharing results is a form of disclosure, and plan.md §0.1
// keeps random samples away from disclosure.
//
// The repository's own target is not enough to decide: a row is created by the
// first target that finds it and keeps that target, so a repository first met
// in a random sample and later scanned on purpose still points at the sample.
// Any non-random target naming it — the owner for a user or org, the URL for a
// single repository — counts.
func (db *DB) RepositoryAuthorized(ctx context.Context, repoID entity.ID) (bool, error) {
	var ok bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM repositories r JOIN targets t ON t.kind <> 'random' AND (
				t.id = r.target_id
				OR (t.kind IN ('user', 'org') AND lower(t.value) = lower(r.owner))
				OR (t.kind = 'repo' AND regexp_replace(lower(t.value), '(\.git)?/*$', '')
				    = 'https://github.com/' || lower(r.owner) || '/' || lower(r.name))
			)
			WHERE r.id = $1
		)`, repoID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("store: repository authorized: %w", err)
	}
	return ok, nil
}
