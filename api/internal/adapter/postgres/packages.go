package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type PackageRepo struct{ pool *pgxpool.Pool }

func NewPackageRepo(pool *pgxpool.Pool) *PackageRepo { return &PackageRepo{pool: pool} }

func (r *PackageRepo) Create(ctx context.Context, p *domain.Package) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO packages (id, publisher_id, slug, name, kind, description, visibility, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, p.PublisherID, p.Slug, p.Name, p.Kind, p.Description, p.Visibility, p.CreatedAt, p.UpdatedAt)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

// packageColumns/packageFrom join the latest published release (manifest) and its artifact stats.
const packageColumns = `
SELECT p.id, p.publisher_id, pub.slug, pub.name, p.slug, p.name, p.kind, COALESCE(p.description, ''), p.latest_version,
       p.visibility, p.created_at, p.updated_at,
       COALESCE(r.manifest_canonical, convert_to(r.manifest::text, 'UTF8')), r.published_at,
       COALESCE(st.cnt, 0), COALESCE(st.samples, 0), COALESCE(st.size, 0)::bigint, p.download_count`

const packageFrom = `
FROM packages p
JOIN publishers pub ON pub.id = p.publisher_id
LEFT JOIN releases r ON r.package_id = p.id AND r.version = p.latest_version
LEFT JOIN LATERAL (
  SELECT count(*) AS cnt, count(*) FILTER (WHERE a.kind = 'sample') AS samples, sum(a.size_bytes) AS size
  FROM artifacts a WHERE a.release_id = r.id
) st ON true`

func scanPackage(row pgx.Row, extra ...any) (*domain.PackageView, error) {
	var v domain.PackageView
	dest := []any{&v.ID, &v.PublisherID, &v.PublisherSlug, &v.PublisherName, &v.Slug, &v.Name, &v.Kind, &v.Description, &v.LatestVersion,
		&v.Visibility, &v.CreatedAt, &v.UpdatedAt, &v.LatestManifest, &v.LatestPublishedAt, &v.ArtifactCount, &v.SampleCount, &v.TotalSize, &v.Downloads}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, notFound(err)
	}
	return &v, nil
}

func (r *PackageRepo) Get(ctx context.Context, publisherSlug, slug string) (*domain.PackageView, error) {
	return scanPackage(r.pool.QueryRow(ctx, packageColumns+packageFrom+` WHERE pub.slug = $1 AND p.slug = $2`, publisherSlug, slug))
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *PackageRepo) List(ctx context.Context, f domain.PackageFilter) ([]domain.PackageView, int, error) {
	order := `COALESCE(r.published_at, p.updated_at) DESC, p.id`
	switch f.Sort {
	case domain.SortName:
		order = `lower(p.name), p.id`
	case domain.SortCreated:
		order = `p.created_at DESC, p.id`
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 24
	}
	if f.Kinds == nil {
		f.Kinds = []string{}
	}
	q := packageColumns + `, count(*) OVER () AS total` + packageFrom + `
WHERE ($1 = '' OR p.name ILIKE $1 OR p.slug ILIKE $1 OR p.description ILIKE $1 OR pub.name ILIKE $1)
  AND (cardinality($2::text[]) = 0 OR p.kind = ANY($2))
  AND ($3 = '' OR pub.slug = $3)
  AND ($4 OR p.latest_version IS NOT NULL)
  AND ($4 OR p.visibility = 'public')
ORDER BY ` + order + `
LIMIT $5 OFFSET $6`
	like := ""
	if s := strings.TrimSpace(f.Query); s != "" {
		like = "%" + likeEscape(s) + "%"
	}
	rows, err := r.pool.Query(ctx, q, like, f.Kinds, f.PublisherSlug, f.IncludeUnpublished, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.PackageView{}
	total := 0
	for rows.Next() {
		v, err := scanPackage(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *v)
	}
	return out, total, rows.Err()
}

func (r *PackageRepo) Update(ctx context.Context, id uuid.UUID, name, description, visibility string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE packages SET name = $2, description = $3, visibility = $4, updated_at = $5 WHERE id = $1`,
		id, name, description, visibility, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PackageRepo) CountDownload(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE packages SET download_count = download_count + 1 WHERE id = $1`, id)
	return err
}
