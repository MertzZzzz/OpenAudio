package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/security"
)

func (s *Service) issueOneTime(ctx context.Context, userID uuid.UUID, purpose domain.OneTimePurpose, ttl time.Duration) (string, error) {
	now := s.Now()
	// Only the newest link of each kind stays valid.
	if err := s.OneTime.InvalidateForUser(ctx, userID, purpose, now); err != nil {
		return "", err
	}
	plain, hash, err := security.NewOpaqueToken()
	if err != nil {
		return "", err
	}
	t := &domain.OneTimeToken{ID: uuid.New(), UserID: userID, Purpose: purpose, TokenHash: hash, ExpiresAt: now.Add(ttl), CreatedAt: now}
	if err := s.OneTime.Create(ctx, t); err != nil {
		return "", err
	}
	return plain, nil
}

func (s *Service) link(path, token string) string {
	return fmt.Sprintf("%s%s?token=%s", s.cfg.WebURL, path, url.QueryEscape(token))
}

func (s *Service) sendVerification(ctx context.Context, u *domain.User) error {
	token, err := s.issueOneTime(ctx, u.ID, domain.PurposeVerifyEmail, s.cfg.VerifyEmailTTL)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Hi %s,\n\nconfirm your email address for OpenAudio:\n\n%s\n\nThe link expires in %s.\n",
		u.DisplayName, s.link("/verify-email", token), s.cfg.VerifyEmailTTL)
	return s.Mailer.Send(ctx, u.Email, "Confirm your OpenAudio email", body)
}

// ResendVerification sends a fresh verification link to a signed-in user.
func (s *Service) ResendVerification(ctx context.Context, userID uuid.UUID) error {
	u, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		return domain.Internal(err)
	}
	if u.EmailVerified() {
		return domain.Conflict("email_already_verified", "email is already verified")
	}
	if err := s.sendVerification(ctx, u); err != nil {
		return domain.Internal(err)
	}
	return nil
}

var errTokenInvalid = domain.Unauthorized("token_invalid", "this link is invalid or has expired")

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return errTokenInvalid
	}
	now := s.Now()
	t, err := s.OneTime.Consume(ctx, security.HashToken(token), domain.PurposeVerifyEmail, now)
	if err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Code == "token_invalid" {
			return errTokenInvalid
		}
		return domain.Internal(err)
	}
	if err := s.Users.MarkEmailVerified(ctx, t.UserID, now); err != nil {
		return domain.Internal(err)
	}
	return nil
}

// RequestPasswordReset always succeeds from the caller's point of view so the
// endpoint cannot be used to discover which emails have accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if fe := validateEmail(email); fe != nil {
		return domain.Validation(*fe)
	}
	u, err := s.Users.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return domain.Internal(err)
	}
	token, err := s.issueOneTime(ctx, u.ID, domain.PurposePasswordReset, s.cfg.PasswordResetTTL)
	if err != nil {
		return domain.Internal(err)
	}
	body := fmt.Sprintf("Hi %s,\n\nreset your OpenAudio password here:\n\n%s\n\nThe link expires in %s. If you did not ask for this, ignore this email.\n",
		u.DisplayName, s.link("/reset-password", token), s.cfg.PasswordResetTTL)
	if err := s.Mailer.Send(ctx, u.Email, "Reset your OpenAudio password", body); err != nil {
		s.Logger.WarnContext(ctx, "send password reset email failed", "user_id", u.ID, "err", err)
	}
	return nil
}

// ResetPassword sets a new password and signs out every session of the user.
// It also works for OAuth-only accounts, which is how they add a password.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if fe := validatePassword(newPassword); fe != nil {
		return domain.Validation(*fe)
	}
	now := s.Now()
	t, err := s.OneTime.Consume(ctx, security.HashToken(token), domain.PurposePasswordReset, now)
	if err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Code == "token_invalid" {
			return errTokenInvalid
		}
		return domain.Internal(err)
	}
	hash, err := s.Hasher.Hash(newPassword)
	if err != nil {
		return domain.Internal(err)
	}
	if err := s.Users.UpdatePassword(ctx, t.UserID, hash); err != nil {
		return domain.Internal(err)
	}
	// Receiving the reset email proves control of the address.
	if err := s.Users.MarkEmailVerified(ctx, t.UserID, now); err != nil {
		return domain.Internal(err)
	}
	if err := s.Refresh.RevokeAllForUser(ctx, t.UserID, now); err != nil {
		return domain.Internal(err)
	}
	return nil
}
