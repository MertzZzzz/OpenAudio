package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type RefreshTokenRepo struct{ pool *pgxpool.Pool }

func NewRefreshTokenRepo(pool *pgxpool.Pool) *RefreshTokenRepo { return &RefreshTokenRepo{pool: pool} }

func insertRefresh(ctx context.Context, q querier, t *domain.RefreshToken) error {
	_, err := q.Exec(ctx, `INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at, user_agent, ip, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, t.ID, t.UserID, t.FamilyID, t.TokenHash, t.ExpiresAt, t.UserAgent, t.IP, t.CreatedAt)
	return err
}

func (r *RefreshTokenRepo) Create(ctx context.Context, t *domain.RefreshToken) error {
	return insertRefresh(ctx, r.pool, t)
}

func (r *RefreshTokenRepo) GetByHash(ctx context.Context, hash []byte) (*domain.RefreshToken, error) {
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, family_id, token_hash, expires_at, revoked_at, replaced_by, user_agent, ip, created_at
		FROM refresh_tokens WHERE token_hash = $1`, hash).
		Scan(&t.ID, &t.UserID, &t.FamilyID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedBy, &t.UserAgent, &t.IP, &t.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, next *domain.RefreshToken, at time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertRefresh(ctx, tx, next); err != nil {
			return err
		}
		// The revoked_at IS NULL guard makes concurrent rotations of one token race-safe:
		// exactly one wins, the other sees 0 rows and is treated as reuse.
		tag, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2, replaced_by = $3 WHERE id = $1 AND revoked_at IS NULL`, oldID, at, next.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.Conflict("token_reused", "refresh token already rotated")
		}
		return nil
	})
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, at)
	return err
}

func (r *RefreshTokenRepo) RevokeFamily(ctx context.Context, family uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`, family, at)
	return err
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, at)
	return err
}

/* -------------------------------------------------------------- one-time */

type OneTimeTokenRepo struct{ pool *pgxpool.Pool }

func NewOneTimeTokenRepo(pool *pgxpool.Pool) *OneTimeTokenRepo { return &OneTimeTokenRepo{pool: pool} }

func (r *OneTimeTokenRepo) Create(ctx context.Context, t *domain.OneTimeToken) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, string(t.Purpose), t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return err
}

func (r *OneTimeTokenRepo) Consume(ctx context.Context, hash []byte, purpose domain.OneTimePurpose, at time.Time) (*domain.OneTimeToken, error) {
	var t domain.OneTimeToken
	var p string
	err := r.pool.QueryRow(ctx, `UPDATE one_time_tokens SET used_at = $3
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
		RETURNING id, user_id, purpose, token_hash, expires_at, used_at, created_at`, hash, string(purpose), at).
		Scan(&t.ID, &t.UserID, &p, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.Unauthorized("token_invalid", "token invalid")
		}
		return nil, err
	}
	t.Purpose = domain.OneTimePurpose(p)
	return &t, nil
}

func (r *OneTimeTokenRepo) InvalidateForUser(ctx context.Context, userID uuid.UUID, purpose domain.OneTimePurpose, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE one_time_tokens SET used_at = $3 WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, userID, string(purpose), at)
	return err
}
