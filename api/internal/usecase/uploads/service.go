// Package uploads implements resumable, content-addressed blob uploads:
//
//	POST /uploads {digest,size}  → session (or "done" right away when the blob already exists)
//	PUT  /uploads/{id}/parts/{n} → each part, exact size, in any order, retryable
//	POST /uploads/{id}/complete  → assemble, re-hash on the server, move to blobs/{sha256}
//
// The server never trusts the client's digest: the assembled object is hashed and
// only stored under blobs/ when the SHA-256 and size match.
package uploads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type Config struct {
	// PartSize must be ≥ 5 MiB with real S3 (minimum for non-final parts); default 8 MiB.
	PartSize int64
	MaxSize  int64 // default 2 GiB
}

type Deps struct {
	Uploads    domain.UploadRepository
	Publishers domain.PublisherRepository
	Blobs      domain.BlobStore
	Logger     *slog.Logger
	Now        func() time.Time
}

type Service struct {
	Deps
	cfg Config
}

func NewService(d Deps, cfg Config) *Service {
	if cfg.PartSize <= 0 {
		cfg.PartSize = 8 << 20
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 2 << 30
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{Deps: d, cfg: cfg}
}

type Actor = *domain.AccessClaims

var errUploadMissing = domain.NotFound("upload_not_found", "upload not found")

// canUpload: admins, platform publishers and members of any publisher.
func (s *Service) canUpload(ctx context.Context, actor Actor) error {
	if actor == nil {
		return domain.Unauthorized("unauthenticated", "sign in required")
	}
	if actor.Role.AtLeast(domain.RolePublisher) {
		return nil
	}
	ms, err := s.Publishers.ListForUser(ctx, actor.UserID)
	if err != nil {
		return domain.Internal(err)
	}
	if len(ms) == 0 {
		return domain.Forbidden("not_a_publisher", "join or create a publisher to upload files")
	}
	return nil
}

type CreateInput struct {
	Digest      string
	Size        int64
	ContentType string
}

func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (*domain.Upload, error) {
	if err := s.canUpload(ctx, actor); err != nil {
		return nil, err
	}
	var fields []domain.FieldError
	if !domain.ValidDigest(in.Digest) {
		fields = append(fields, domain.FieldError{Field: "digest", Message: "digest must be sha256:<64 lowercase hex>"})
	}
	if in.Size <= 0 || in.Size > s.cfg.MaxSize {
		fields = append(fields, domain.FieldError{Field: "size", Message: fmt.Sprintf("size must be between 1 and %d bytes", s.cfg.MaxSize)})
	}
	if len(in.ContentType) > 128 {
		fields = append(fields, domain.FieldError{Field: "contentType", Message: "too long"})
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	now := s.Now()
	u := &domain.Upload{ID: uuid.New(), UserID: actor.UserID, Digest: in.Digest, Size: in.Size, ContentType: in.ContentType,
		Status: domain.UploadPending, PartSize: s.cfg.PartSize, CreatedAt: now}

	// Content addressing makes re-uploads free: an identical blob is already stored.
	key, _ := domain.BlobKey(in.Digest)
	size, exists, err := s.Blobs.Stat(ctx, key)
	if err != nil {
		return nil, domain.Internal(err)
	}
	if exists {
		if size != in.Size {
			return nil, domain.Conflict("digest_size_mismatch", fmt.Sprintf("a blob with this digest exists with %d bytes", size))
		}
		u.Status, u.CompletedAt = domain.UploadDone, &now
	} else {
		mp, err := s.Blobs.CreateMultipart(ctx, domain.UploadKey(u.ID.String()), in.ContentType)
		if err != nil {
			return nil, domain.Internal(err)
		}
		u.MultipartID = mp
	}
	if err := s.Uploads.Create(ctx, u); err != nil {
		return nil, domain.Internal(err)
	}
	return u, nil
}

func (s *Service) own(ctx context.Context, actor Actor, id uuid.UUID) (*domain.Upload, error) {
	if actor == nil {
		return nil, domain.Unauthorized("unauthenticated", "sign in required")
	}
	u, err := s.Uploads.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errUploadMissing
	}
	if err != nil {
		return nil, domain.Internal(err)
	}
	if u.UserID != actor.UserID && actor.Role != domain.RoleAdmin {
		return nil, errUploadMissing
	}
	return u, nil
}

type Status struct {
	Upload *domain.Upload
	Parts  []domain.UploadPart
}

func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Status, error) {
	u, err := s.own(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	parts, err := s.Uploads.Parts(ctx, id)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return &Status{Upload: u, Parts: parts}, nil
}

func notPending(u *domain.Upload) error {
	return domain.Conflict("upload_finished", fmt.Sprintf("upload is already %s", u.Status))
}

// PutPart stores part n. contentLength must equal the exact expected part size.
func (s *Service) PutPart(ctx context.Context, actor Actor, id uuid.UUID, n int, body io.Reader, contentLength int64) (*domain.UploadPart, error) {
	u, err := s.own(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if u.Status != domain.UploadPending {
		return nil, notPending(u)
	}
	if n < 1 || n > u.PartCount() {
		return nil, domain.Validation(domain.FieldError{Field: "partNumber", Message: fmt.Sprintf("part number must be between 1 and %d", u.PartCount())})
	}
	want := u.ExpectedPartSize(n)
	if contentLength != want {
		return nil, domain.Validation(domain.FieldError{Field: "Content-Length", Message: fmt.Sprintf("part %d must be exactly %d bytes, got %d", n, want, contentLength)})
	}
	etag, err := s.Blobs.UploadPart(ctx, domain.UploadKey(u.ID.String()), u.MultipartID, n, io.LimitReader(body, want), want)
	if err != nil {
		return nil, domain.Internal(fmt.Errorf("upload part %d: %w", n, err))
	}
	p := domain.UploadPart{Number: n, ETag: etag, Size: want}
	if err := s.Uploads.PutPart(ctx, u.ID, p); err != nil {
		return nil, domain.Internal(err)
	}
	return &p, nil
}

// Complete assembles the parts, verifies SHA-256 and size, and moves the object to blobs/{sha256}.
func (s *Service) Complete(ctx context.Context, actor Actor, id uuid.UUID) (*domain.Upload, error) {
	u, err := s.own(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if u.Status == domain.UploadDone {
		return u, nil // idempotent
	}
	if u.Status != domain.UploadPending {
		return nil, notPending(u)
	}
	parts, err := s.Uploads.Parts(ctx, u.ID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	have := map[int]domain.UploadPart{}
	for _, p := range parts {
		have[p.Number] = p
	}
	var missing []int
	completed := make([]domain.CompletedPart, 0, u.PartCount())
	for n := 1; n <= u.PartCount(); n++ {
		p, ok := have[n]
		if !ok {
			missing = append(missing, n)
			continue
		}
		completed = append(completed, domain.CompletedPart{Number: n, ETag: p.ETag})
	}
	if len(missing) > 0 {
		return nil, domain.Validation(domain.FieldError{Field: "parts", Message: fmt.Sprintf("missing parts: %v", missing)})
	}

	tmp := domain.UploadKey(u.ID.String())
	if err := s.Blobs.CompleteMultipart(ctx, tmp, u.MultipartID, completed); err != nil {
		return nil, domain.Internal(fmt.Errorf("complete multipart: %w", err))
	}
	digest, size, err := s.hash(ctx, tmp)
	if err != nil {
		return nil, domain.Internal(err)
	}
	now := s.Now()
	if digest != u.Digest || size != u.Size {
		_ = s.Blobs.Delete(ctx, tmp)
		msg := fmt.Sprintf("uploaded content has %s (%d bytes), expected %s (%d bytes)", digest, size, u.Digest, u.Size)
		if err := s.Uploads.Finish(ctx, u.ID, domain.UploadFailed, msg, now); err != nil {
			s.Logger.WarnContext(ctx, "mark upload failed", "upload_id", u.ID, "err", err)
		}
		return nil, &domain.Error{Kind: domain.KindValidation, Code: "digest_mismatch", Message: msg}
	}
	key, _ := domain.BlobKey(u.Digest)
	if _, exists, err := s.Blobs.Stat(ctx, key); err != nil {
		return nil, domain.Internal(err)
	} else if !exists {
		if err := s.Blobs.Copy(ctx, tmp, key); err != nil {
			return nil, domain.Internal(fmt.Errorf("store blob: %w", err))
		}
	}
	if err := s.Blobs.Delete(ctx, tmp); err != nil {
		s.Logger.WarnContext(ctx, "delete temporary upload object", "key", tmp, "err", err)
	}
	if err := s.Uploads.Finish(ctx, u.ID, domain.UploadDone, "", now); err != nil {
		return nil, domain.Internal(err)
	}
	u.Status, u.CompletedAt = domain.UploadDone, &now
	return u, nil
}

func (s *Service) hash(ctx context.Context, key string) (string, int64, error) {
	r, err := s.Blobs.Open(ctx, key)
	if err != nil {
		return "", 0, fmt.Errorf("open assembled upload: %w", err)
	}
	defer r.Close()
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, fmt.Errorf("hash assembled upload: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), n, nil
}

// Abort cancels a pending upload and releases the multipart storage.
func (s *Service) Abort(ctx context.Context, actor Actor, id uuid.UUID) error {
	u, err := s.own(ctx, actor, id)
	if err != nil {
		return err
	}
	if u.Status != domain.UploadPending {
		return notPending(u)
	}
	if err := s.Blobs.AbortMultipart(ctx, domain.UploadKey(u.ID.String()), u.MultipartID); err != nil {
		s.Logger.WarnContext(ctx, "abort multipart", "upload_id", u.ID, "err", err)
	}
	if err := s.Uploads.Finish(ctx, u.ID, domain.UploadFailed, "aborted", s.Now()); err != nil {
		return domain.Internal(err)
	}
	return nil
}
