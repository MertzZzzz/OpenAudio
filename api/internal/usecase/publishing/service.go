// Package publishing implements publishers, packages and the release lifecycle:
// draft → (validate, upload blobs) → publish (immutable) → yank.
package publishing

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
)

type Deps struct {
	Users      domain.UserRepository
	Publishers domain.PublisherRepository
	Packages   domain.PackageRepository
	Releases   domain.ReleaseRepository
	Blobs      domain.BlobStore
	Validator  *manifest.Validator
	Logger     *slog.Logger
	Now        func() time.Time
	// DownloadTTL is the lifetime of presigned artifact URLs.
	DownloadTTL time.Duration
}

type Service struct{ Deps }

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.DownloadTTL == 0 {
		d.DownloadTTL = 10 * time.Minute
	}
	return &Service{d}
}

// Actor is the authenticated caller; nil means anonymous.
type Actor = *domain.AccessClaims

var (
	slugRe        = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	reservedSlugs = map[string]bool{"admin": true, "api": true, "openaudio": true, "new": true, "settings": true, "me": true, "official": true}

	errForbidden        = domain.Forbidden("forbidden", "you do not have permission to do this")
	errPublisherMissing = domain.NotFound("publisher_not_found", "publisher not found")
	errPackageMissing   = domain.NotFound("package_not_found", "package not found")
	errReleaseMissing   = domain.NotFound("release_not_found", "release not found")
)

func validSlug(field, s string, min, max int) *domain.FieldError {
	if len(s) < min || len(s) > max || !slugRe.MatchString(s) {
		return &domain.FieldError{Field: field, Message: "use lowercase letters, digits and single dashes (" + strconv.Itoa(min) + "–" + strconv.Itoa(max) + " chars)"}
	}
	return nil
}

func validName(field, s string) *domain.FieldError {
	if n := utf8.RuneCountInString(strings.TrimSpace(s)); n < 2 || n > 80 {
		return &domain.FieldError{Field: field, Message: "must be 2–80 characters"}
	}
	return nil
}

func collect(errs ...*domain.FieldError) error {
	var out []domain.FieldError
	for _, e := range errs {
		if e != nil {
			out = append(out, *e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return domain.Validation(out...)
}

func internal(err error) error {
	var de *domain.Error
	if errors.As(err, &de) {
		return err
	}
	return domain.Internal(err)
}

/* ---------------------------------------------------------- authorization */

func (s *Service) publisher(ctx context.Context, slug string) (*domain.Publisher, error) {
	p, err := s.Publishers.GetBySlug(ctx, slug)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errPublisherMissing
	}
	if err != nil {
		return nil, internal(err)
	}
	return p, nil
}

// role returns the actor's member role ("" when not a member). Admins act as owners.
func (s *Service) role(ctx context.Context, actor Actor, publisherID uuid.UUID) (domain.MemberRole, error) {
	if actor == nil {
		return "", nil
	}
	if actor.Role == domain.RoleAdmin {
		return domain.MemberOwner, nil
	}
	r, err := s.Publishers.MemberRole(ctx, publisherID, actor.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", internal(err)
	}
	return r, nil
}

func (s *Service) authorize(ctx context.Context, actor Actor, publisherID uuid.UUID, perm domain.Permission) error {
	if actor == nil {
		return domain.Unauthorized("unauthenticated", "sign in required")
	}
	r, err := s.role(ctx, actor, publisherID)
	if err != nil {
		return err
	}
	if !r.Allows(perm) {
		return errForbidden
	}
	return nil
}
