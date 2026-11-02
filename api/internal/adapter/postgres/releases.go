package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type ReleaseRepo struct{ pool *pgxpool.Pool }

func NewReleaseRepo(pool *pgxpool.Pool) *ReleaseRepo { return &ReleaseRepo{pool: pool} }

var errImmutable = domain.Conflict("release_immutable", "only draft releases can be changed; publish a new version instead")

const releaseColumns = `id, package_id, version, manifest, manifest_digest, changelog, status, published_at, created_by, published_by, created_at`

func scanRelease(row pgx.Row) (*domain.Release, error) {
	var r domain.Release
	var status string
	if err := row.Scan(&r.ID, &r.PackageID, &r.Version, &r.Manifest, &r.ManifestDigest, &r.Changelog, &status, &r.PublishedAt, &r.CreatedBy, &r.PublishedBy, &r.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	r.Status = domain.ReleaseStatus(status)
	return &r, nil
}

func (s *ReleaseRepo) CreateDraft(ctx context.Context, r *domain.Release) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO releases (id, package_id, version, manifest, manifest_digest, changelog, status, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,'draft',$7,$8)`, r.ID, r.PackageID, r.Version, r.Manifest, r.ManifestDigest, r.Changelog, r.CreatedBy, r.CreatedAt)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (s *ReleaseRepo) Get(ctx context.Context, packageID uuid.UUID, version string) (*domain.Release, error) {
	return scanRelease(s.pool.QueryRow(ctx, `SELECT `+releaseColumns+` FROM releases WHERE package_id = $1 AND version = $2`, packageID, version))
}

// semverOrder sorts "MAJOR.MINOR.PATCH" numerically (the version CHECK constraint guarantees the format).
const semverOrder = `string_to_array(version, '.')::int[] DESC`

func (s *ReleaseRepo) List(ctx context.Context, packageID uuid.UUID) ([]domain.Release, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+releaseColumns+` FROM releases WHERE package_id = $1 ORDER BY `+semverOrder, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Release{}
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *ReleaseRepo) UpdateDraft(ctx context.Context, id uuid.UUID, manifest []byte, digest, changelog string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE releases SET manifest = $2, manifest_digest = $3, changelog = $4 WHERE id = $1 AND status = 'draft'`,
		id, manifest, digest, changelog)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errImmutable
	}
	return nil
}

func (s *ReleaseRepo) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM releases WHERE id = $1 AND status = 'draft'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errImmutable
	}
	return nil
}

const recomputeLatest = `
UPDATE packages SET updated_at = $2, latest_version = (
  SELECT version FROM releases WHERE package_id = $1 AND status = 'published' ORDER BY ` + semverOrder + ` LIMIT 1
) WHERE id = $1`

func (s *ReleaseRepo) Publish(ctx context.Context, in domain.PublishInput) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Lock the draft row so two concurrent publishes cannot both succeed.
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM releases WHERE id = $1 FOR UPDATE`, in.ReleaseID).Scan(&status); err != nil {
			return notFound(err)
		}
		if status != string(domain.ReleaseDraft) {
			return errImmutable
		}
		rows := make([][]any, len(in.Artifacts))
		for i, a := range in.Artifacts {
			rows[i] = []any{a.ID, in.ReleaseID, a.Kind, a.Path, a.Digest, a.SizeBytes, a.ContentType, a.StorageKey, in.At}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"artifacts"},
			[]string{"id", "release_id", "kind", "path", "digest", "size_bytes", "content_type", "storage_key", "created_at"},
			pgx.CopyFromRows(rows)); err != nil {
			return err
		}
		for _, t := range in.Terms {
			if _, err := tx.Exec(ctx, `INSERT INTO package_taxonomy (package_id, release_id, domain, slug, weight) VALUES ($1,$2,$3,$4,$5)
				ON CONFLICT DO NOTHING`, in.PackageID, in.ReleaseID, t.Domain, t.Slug, t.Weight); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE releases SET status = 'published', published_at = $2, published_by = $3 WHERE id = $1`,
			in.ReleaseID, in.At, in.PublishedBy); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, recomputeLatest, in.PackageID, in.At)
		return err
	})
}

func (s *ReleaseRepo) Yank(ctx context.Context, releaseID, packageID uuid.UUID, at time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE releases SET status = 'yanked' WHERE id = $1 AND status = 'published'`, releaseID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.Conflict("not_published", "only published releases can be yanked")
		}
		_, err = tx.Exec(ctx, recomputeLatest, packageID, at)
		return err
	})
}

func (s *ReleaseRepo) Artifacts(ctx context.Context, releaseID uuid.UUID) ([]domain.Artifact, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, release_id, kind, path, digest, size_bytes, COALESCE(content_type, ''), storage_key, created_at
		FROM artifacts WHERE release_id = $1 ORDER BY path`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Artifact{}
	for rows.Next() {
		var a domain.Artifact
		if err := rows.Scan(&a.ID, &a.ReleaseID, &a.Kind, &a.Path, &a.Digest, &a.SizeBytes, &a.ContentType, &a.StorageKey, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
