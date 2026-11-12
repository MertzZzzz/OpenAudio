package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// AdminAccount is the default administrator configured through the environment.
type AdminAccount struct {
	Email       string
	Password    string
	DisplayName string
}

// EnsureAdmin makes the configured account exist with the admin role, a verified email and the
// configured password. The environment is the source of truth: if the password stored in the
// database differs (changed in the UI or in .env), it is replaced and the account's sessions are
// revoked. Safe to run on every start.
func (s *Service) EnsureAdmin(ctx context.Context, a AdminAccount) (created bool, err error) {
	email := normalizeEmail(a.Email)
	name := strings.TrimSpace(a.DisplayName)
	if name == "" {
		name = "Admin"
	}
	if err := collect(validateEmail(email), validatePassword(a.Password)); err != nil {
		return false, fmt.Errorf("default admin (ADMIN_EMAIL/ADMIN_PASSWORD): %w", err)
	}
	now := s.Now()

	u, err := s.Users.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		hash, err := s.Hasher.Hash(a.Password)
		if err != nil {
			return false, err
		}
		u = &domain.User{
			ID: uuid.New(), Email: email, PasswordHash: &hash, DisplayName: truncate(name, maxNameLen),
			Role: domain.RoleAdmin, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.Users.Create(ctx, u); err != nil {
			return false, fmt.Errorf("create default admin: %w", err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("look up default admin: %w", err)
	}

	if u.Role != domain.RoleAdmin {
		if err := s.Users.UpdateRole(ctx, u.ID, domain.RoleAdmin); err != nil {
			return false, fmt.Errorf("promote default admin: %w", err)
		}
	}
	if !u.EmailVerified() {
		if err := s.Users.MarkEmailVerified(ctx, u.ID, now); err != nil {
			return false, fmt.Errorf("verify default admin email: %w", err)
		}
	}
	same := false
	if u.HasPassword() {
		if same, err = s.Hasher.Verify(a.Password, *u.PasswordHash); err != nil {
			same = false
		}
	}
	if !same {
		hash, err := s.Hasher.Hash(a.Password)
		if err != nil {
			return false, err
		}
		if err := s.Users.UpdatePassword(ctx, u.ID, hash); err != nil {
			return false, fmt.Errorf("set default admin password: %w", err)
		}
		if err := s.Refresh.RevokeAllForUser(ctx, u.ID, now); err != nil {
			return false, fmt.Errorf("revoke default admin sessions: %w", err)
		}
		s.Logger.InfoContext(ctx, "default admin password updated from environment", "email", email)
	}
	return false, nil
}
