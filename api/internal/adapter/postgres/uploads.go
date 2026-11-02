package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type UploadRepo struct{ pool *pgxpool.Pool }

func NewUploadRepo(pool *pgxpool.Pool) *UploadRepo { return &UploadRepo{pool: pool} }

func (r *UploadRepo) Create(ctx context.Context, u *domain.Upload) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO uploads (id, user_id, digest, size_bytes, content_type, status, multipart_id, part_size, created_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.ID, u.UserID, u.Digest, u.Size, u.ContentType, string(u.Status), u.MultipartID, u.PartSize, u.CreatedAt, u.CompletedAt)
	return err
}

func (r *UploadRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Upload, error) {
	var u domain.Upload
	var status string
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, digest, size_bytes, content_type, status, multipart_id, part_size, error, created_at, completed_at
		FROM uploads WHERE id = $1`, id).
		Scan(&u.ID, &u.UserID, &u.Digest, &u.Size, &u.ContentType, &status, &u.MultipartID, &u.PartSize, &u.Error, &u.CreatedAt, &u.CompletedAt)
	if err != nil {
		return nil, notFound(err)
	}
	u.Status = domain.UploadStatus(status)
	return &u, nil
}

func (r *UploadRepo) PutPart(ctx context.Context, uploadID uuid.UUID, p domain.UploadPart) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO upload_parts (upload_id, part_number, etag, size_bytes) VALUES ($1,$2,$3,$4)
		ON CONFLICT (upload_id, part_number) DO UPDATE SET etag = EXCLUDED.etag, size_bytes = EXCLUDED.size_bytes`,
		uploadID, p.Number, p.ETag, p.Size)
	return err
}

func (r *UploadRepo) Parts(ctx context.Context, uploadID uuid.UUID) ([]domain.UploadPart, error) {
	rows, err := r.pool.Query(ctx, `SELECT part_number, etag, size_bytes FROM upload_parts WHERE upload_id = $1 ORDER BY part_number`, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.UploadPart{}
	for rows.Next() {
		var p domain.UploadPart
		if err := rows.Scan(&p.Number, &p.ETag, &p.Size); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *UploadRepo) Finish(ctx context.Context, id uuid.UUID, status domain.UploadStatus, errMsg string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE uploads SET status = $2, error = $3, completed_at = $4 WHERE id = $1 AND status = 'pending'`,
		id, string(status), errMsg, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.Conflict("upload_finished", "upload is already completed or failed")
	}
	return nil
}
