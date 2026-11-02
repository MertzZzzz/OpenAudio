package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type PublisherRepo struct{ pool *pgxpool.Pool }

func NewPublisherRepo(pool *pgxpool.Pool) *PublisherRepo { return &PublisherRepo{pool: pool} }

func (r *PublisherRepo) Create(ctx context.Context, p *domain.Publisher, owner uuid.UUID) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO publishers (id, slug, name, owner_user_id, created_at) VALUES ($1,$2,$3,$4,$5)`,
			p.ID, p.Slug, p.Name, owner, p.CreatedAt)
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO publisher_members (publisher_id, user_id, role) VALUES ($1,$2,'owner')`, p.ID, owner)
		return err
	})
}

func scanPublisher(row pgx.Row) (*domain.Publisher, error) {
	var p domain.Publisher
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.OwnerUserID, &p.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *PublisherRepo) GetBySlug(ctx context.Context, slug string) (*domain.Publisher, error) {
	return scanPublisher(r.pool.QueryRow(ctx, `SELECT id, slug, name, owner_user_id, created_at FROM publishers WHERE slug = $1`, slug))
}

func (r *PublisherRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.PublisherMembership, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.slug, p.name, p.owner_user_id, p.created_at, m.role
		FROM publisher_members m JOIN publishers p ON p.id = m.publisher_id
		WHERE m.user_id = $1 ORDER BY p.slug`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PublisherMembership{}
	for rows.Next() {
		var m domain.PublisherMembership
		var role string
		if err := rows.Scan(&m.Publisher.ID, &m.Publisher.Slug, &m.Publisher.Name, &m.Publisher.OwnerUserID, &m.Publisher.CreatedAt, &role); err != nil {
			return nil, err
		}
		m.Role = domain.MemberRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PublisherRepo) MemberRole(ctx context.Context, publisherID, userID uuid.UUID) (domain.MemberRole, error) {
	var role string
	err := r.pool.QueryRow(ctx, `SELECT role FROM publisher_members WHERE publisher_id = $1 AND user_id = $2`, publisherID, userID).Scan(&role)
	if err != nil {
		return "", notFound(err)
	}
	return domain.MemberRole(role), nil
}

func (r *PublisherRepo) Members(ctx context.Context, publisherID uuid.UUID) ([]domain.Member, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id, u.display_name, u.email, m.role
		FROM publisher_members m JOIN users u ON u.id = m.user_id
		WHERE m.publisher_id = $1
		ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'maintainer' THEN 1 ELSE 2 END, u.display_name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Member{}
	for rows.Next() {
		var m domain.Member
		var role string
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Email, &role); err != nil {
			return nil, err
		}
		m.Role = domain.MemberRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PublisherRepo) AddMember(ctx context.Context, publisherID, userID uuid.UUID, role domain.MemberRole) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO publisher_members (publisher_id, user_id, role) VALUES ($1,$2,$3)`, publisherID, userID, string(role))
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (r *PublisherRepo) RemoveMember(ctx context.Context, publisherID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM publisher_members WHERE publisher_id = $1 AND user_id = $2`, publisherID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
