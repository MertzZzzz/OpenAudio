package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

const userColumns = `id, email, password_hash, display_name, role, avatar_url, email_verified_at, created_at, updated_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var role string
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &role, &u.AvatarURL, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, notFound(err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

func insertUser(ctx context.Context, q querier, u *domain.User) error {
	_, err := q.Exec(ctx, `INSERT INTO users (`+userColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, string(u.Role), u.AvatarURL, u.EmailVerifiedAt, u.CreatedAt, u.UpdatedAt)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	return insertUser(ctx, r.pool, u)
}

func (r *UserRepo) CreateWithIdentity(ctx context.Context, u *domain.User, id *domain.Identity) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertUser(ctx, tx, u); err != nil {
			return err
		}
		return insertIdentity(ctx, tx, id)
	})
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

func (r *UserRepo) exec1(ctx context.Context, sql string, args ...any) error {
	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role domain.Role) error {
	return r.exec1(ctx, `UPDATE users SET role = $2, updated_at = now() WHERE id = $1`, id, string(role))
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	return r.exec1(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
}

func (r *UserRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.exec1(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, $2), updated_at = now() WHERE id = $1`, id, at)
}

/* ------------------------------------------------------------ identities */

type IdentityRepo struct{ pool *pgxpool.Pool }

func NewIdentityRepo(pool *pgxpool.Pool) *IdentityRepo { return &IdentityRepo{pool: pool} }

func insertIdentity(ctx context.Context, q querier, i *domain.Identity) error {
	_, err := q.Exec(ctx, `INSERT INTO user_identities (id, user_id, provider, subject, email, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		i.ID, i.UserID, string(i.Provider), i.Subject, i.Email, i.CreatedAt)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (r *IdentityRepo) Create(ctx context.Context, i *domain.Identity) error {
	return insertIdentity(ctx, r.pool, i)
}

func scanIdentity(row pgx.Row) (*domain.Identity, error) {
	var i domain.Identity
	var provider string
	var email *string
	if err := row.Scan(&i.ID, &i.UserID, &provider, &i.Subject, &email, &i.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	i.Provider = domain.AuthProvider(provider)
	if email != nil {
		i.Email = *email
	}
	return &i, nil
}

func (r *IdentityRepo) Get(ctx context.Context, p domain.AuthProvider, subject string) (*domain.Identity, error) {
	return scanIdentity(r.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, subject, email, created_at FROM user_identities WHERE provider = $1 AND subject = $2`, string(p), subject))
}

func (r *IdentityRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Identity, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, provider, subject, email, created_at FROM user_identities WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Identity{}
	for rows.Next() {
		i, err := scanIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}
