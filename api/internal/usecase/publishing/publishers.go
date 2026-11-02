package publishing

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type CreatePublisherInput struct {
	Slug string
	Name string
}

// CreatePublisher creates a publisher owned by the actor. Requires a verified email; a plain
// "user" is promoted to the "publisher" platform role (visible in the next access token).
func (s *Service) CreatePublisher(ctx context.Context, actor Actor, in CreatePublisherInput) (*domain.Publisher, error) {
	if actor == nil {
		return nil, domain.Unauthorized("unauthenticated", "sign in required")
	}
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	fe := validSlug("slug", slug, 2, 40)
	if fe == nil && reservedSlugs[slug] {
		fe = &domain.FieldError{Field: "slug", Message: "this name is reserved"}
	}
	if err := collect(fe, validName("name", in.Name)); err != nil {
		return nil, err
	}
	u, err := s.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, internal(err)
	}
	if !u.EmailVerified() && actor.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("email_unverified", "confirm your email address before creating a publisher")
	}
	p := &domain.Publisher{ID: uuid.New(), Slug: slug, Name: strings.TrimSpace(in.Name), OwnerUserID: &u.ID, CreatedAt: s.Now()}
	if err := s.Publishers.Create(ctx, p, u.ID); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("publisher_slug_taken", "a publisher with this name already exists")
		}
		return nil, internal(err)
	}
	if u.Role == domain.RoleUser {
		if err := s.Users.UpdateRole(ctx, u.ID, domain.RolePublisher); err != nil {
			return nil, internal(err)
		}
	}
	return p, nil
}

func (s *Service) MyPublishers(ctx context.Context, actor Actor) ([]domain.PublisherMembership, error) {
	if actor == nil {
		return nil, domain.Unauthorized("unauthenticated", "sign in required")
	}
	m, err := s.Publishers.ListForUser(ctx, actor.UserID)
	return m, internalOrNil(err)
}

func internalOrNil(err error) error {
	if err == nil {
		return nil
	}
	return internal(err)
}

func (s *Service) GetPublisher(ctx context.Context, slug string) (*domain.Publisher, error) {
	return s.publisher(ctx, slug)
}

// Members is visible to members of the publisher.
func (s *Service) Members(ctx context.Context, actor Actor, slug string) ([]domain.Member, error) {
	p, err := s.publisher(ctx, slug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, p.ID, domain.PermDraft); err != nil {
		return nil, err
	}
	m, err := s.Publishers.Members(ctx, p.ID)
	return m, internalOrNil(err)
}

func (s *Service) AddMember(ctx context.Context, actor Actor, slug, email string, role domain.MemberRole) (*domain.Member, error) {
	p, err := s.publisher(ctx, slug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, p.ID, domain.PermManageMembers); err != nil {
		return nil, err
	}
	if role != domain.MemberMaintainer && role != domain.MemberUploader {
		return nil, domain.Validation(domain.FieldError{Field: "role", Message: "role must be maintainer or uploader"})
	}
	u, err := s.Users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.NotFound("user_not_found", "no OpenAudio account uses this email; ask them to sign up first")
	}
	if err != nil {
		return nil, internal(err)
	}
	if err := s.Publishers.AddMember(ctx, p.ID, u.ID, role); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("member_exists", "this user is already a member")
		}
		return nil, internal(err)
	}
	return &domain.Member{UserID: u.ID, DisplayName: u.DisplayName, Email: u.Email, Role: role}, nil
}

// RemoveMember lets owners remove members and lets any member leave. The owner cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, actor Actor, slug string, userID uuid.UUID) error {
	p, err := s.publisher(ctx, slug)
	if err != nil {
		return err
	}
	if actor == nil {
		return domain.Unauthorized("unauthenticated", "sign in required")
	}
	if actor.UserID != userID {
		if err := s.authorize(ctx, actor, p.ID, domain.PermManageMembers); err != nil {
			return err
		}
	}
	r, err := s.Publishers.MemberRole(ctx, p.ID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NotFound("member_not_found", "this user is not a member")
	}
	if err != nil {
		return internal(err)
	}
	if r == domain.MemberOwner {
		return domain.Validation(domain.FieldError{Field: "userId", Message: "the owner cannot be removed"})
	}
	return internalOrNil(s.Publishers.RemoveMember(ctx, p.ID, userID))
}
