package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type LibraryRepo struct{ pool *pgxpool.Pool }

func NewLibraryRepo(pool *pgxpool.Pool) *LibraryRepo { return &LibraryRepo{pool: pool} }

func (r *LibraryRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Library, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, name, created_at FROM libraries WHERE user_id = $1 ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Library{}
	for rows.Next() {
		var l domain.Library
		if err := rows.Scan(&l.ID, &l.UserID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *LibraryRepo) Create(ctx context.Context, l *domain.Library) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO libraries (id, user_id, name, created_at) VALUES ($1,$2,$3,$4)`, l.ID, l.UserID, l.Name, l.CreatedAt)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (r *LibraryRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Library, error) {
	var l domain.Library
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, name, created_at FROM libraries WHERE id = $1`, id).Scan(&l.ID, &l.UserID, &l.Name, &l.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *LibraryRepo) Rename(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE libraries SET name = $2 WHERE id = $1`, id, name)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *LibraryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM libraries WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *LibraryRepo) Items(ctx context.Context, libraryID uuid.UUID) ([]domain.LibraryItem, error) {
	rows, err := r.pool.Query(ctx, packageColumns+`, li.library_id, li.pinned_version, li.installed_version, li.added_at`+packageFrom+`
		JOIN library_items li ON li.package_id = p.id
		WHERE li.library_id = $1
		ORDER BY li.added_at, p.id`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.LibraryItem{}
	for rows.Next() {
		var it domain.LibraryItem
		v, err := scanPackage(rows, &it.LibraryID, &it.PinnedVersion, &it.InstalledVersion, &it.AddedAt)
		if err != nil {
			return nil, err
		}
		it.Package = *v
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *LibraryRepo) PutItem(ctx context.Context, libraryID, packageID uuid.UUID, pinned, installed *string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO library_items (library_id, package_id, pinned_version, installed_version, added_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (library_id, package_id) DO UPDATE SET pinned_version = EXCLUDED.pinned_version, installed_version = EXCLUDED.installed_version`,
		libraryID, packageID, pinned, installed, at)
	return err
}

func (r *LibraryRepo) RemoveItem(ctx context.Context, libraryID, packageID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM library_items WHERE library_id = $1 AND package_id = $2`, libraryID, packageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

/* ------------------------------------------------------------- lockfiles */

type LockfileRepo struct{ pool *pgxpool.Pool }

func NewLockfileRepo(pool *pgxpool.Pool) *LockfileRepo { return &LockfileRepo{pool: pool} }

func (r *LockfileRepo) Create(ctx context.Context, l *domain.LockfileRecord) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO project_lockfiles (id, owner_user_id, library_id, name, content, content_canonical, content_digest, created_at)
		VALUES ($1,$2,$3,$4,convert_from($5, 'UTF8')::jsonb,$5,$6,$7)`, l.ID, l.OwnerUserID, l.LibraryID, l.Name, l.Content, l.ContentDigest, l.CreatedAt)
	return err
}

func (r *LockfileRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.LockfileRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, owner_user_id, library_id, name, content_digest, created_at
		FROM project_lockfiles WHERE owner_user_id = $1 ORDER BY created_at DESC, id LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.LockfileRecord{}
	for rows.Next() {
		var l domain.LockfileRecord
		if err := rows.Scan(&l.ID, &l.OwnerUserID, &l.LibraryID, &l.Name, &l.ContentDigest, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *LockfileRepo) Get(ctx context.Context, id uuid.UUID) (*domain.LockfileRecord, error) {
	var l domain.LockfileRecord
	err := r.pool.QueryRow(ctx, `SELECT id, owner_user_id, library_id, name,
		COALESCE(content_canonical, convert_to(content::text, 'UTF8')), content_digest, created_at FROM project_lockfiles WHERE id = $1`, id).
		Scan(&l.ID, &l.OwnerUserID, &l.LibraryID, &l.Name, &l.Content, &l.ContentDigest, &l.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *LockfileRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_lockfiles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
