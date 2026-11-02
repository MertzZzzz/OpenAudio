// Package auth implements account and session use cases: email/password sign-up and
// sign-in, Google/Discord sign-in, refresh-token rotation, email verification,
// password reset and role management.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/security"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
	NeedsRehash(encoded string) bool
}

type AccessTokenIssuer interface {
	Issue(userID uuid.UUID, role domain.Role) (string, time.Time, error)
}

type Config struct {
	RefreshTTL       time.Duration
	VerifyEmailTTL   time.Duration
	PasswordResetTTL time.Duration
	// WebURL is used to build links in emails, e.g. {WebURL}/verify-email?token=...
	WebURL string
	// AdminEmails get the admin role on account creation (bootstrap).
	AdminEmails []string
}

type Deps struct {
	Users      domain.UserRepository
	Identities domain.IdentityRepository
	Refresh    domain.RefreshTokenRepository
	OneTime    domain.OneTimeTokenRepository
	Hasher     PasswordHasher
	Tokens     AccessTokenIssuer
	Mailer     domain.Mailer
	Logger     *slog.Logger
	Now        func() time.Time
}

type Service struct {
	Deps
	cfg         Config
	adminEmails map[string]struct{}
	dummyHash   string
}

func NewService(d Deps, cfg Config) (*Service, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	if cfg.VerifyEmailTTL == 0 {
		cfg.VerifyEmailTTL = 48 * time.Hour
	}
	if cfg.PasswordResetTTL == 0 {
		cfg.PasswordResetTTL = time.Hour
	}
	admins := make(map[string]struct{}, len(cfg.AdminEmails))
	for _, e := range cfg.AdminEmails {
		if e = normalizeEmail(e); e != "" {
			admins[e] = struct{}{}
		}
	}
	// Used to spend the same time on unknown emails as on wrong passwords (no user enumeration by timing).
	dummy, err := d.Hasher.Hash("openaudio-timing-equalizer")
	if err != nil {
		return nil, fmt.Errorf("init dummy hash: %w", err)
	}
	return &Service{Deps: d, cfg: cfg, adminEmails: admins, dummyHash: dummy}, nil
}

// ClientMeta is recorded with refresh tokens for session auditing.
type ClientMeta struct {
	UserAgent string
	IP        string
}

// Session is the result of every successful sign-in or refresh.
type Session struct {
	User             *domain.User
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

/* ------------------------------------------------------------ validation */

const (
	minPasswordLen = 10
	maxPasswordLen = 128
	maxNameLen     = 64
)

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validateEmail(email string) *domain.FieldError {
	if email == "" {
		return &domain.FieldError{Field: "email", Message: "email is required"}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") || len(email) > 254 {
		return &domain.FieldError{Field: "email", Message: "enter a valid email address"}
	}
	return nil
}

func validatePassword(pw string) *domain.FieldError {
	n := utf8.RuneCountInString(pw)
	if n < minPasswordLen {
		return &domain.FieldError{Field: "password", Message: fmt.Sprintf("password must be at least %d characters", minPasswordLen)}
	}
	if n > maxPasswordLen {
		return &domain.FieldError{Field: "password", Message: fmt.Sprintf("password must be at most %d characters", maxPasswordLen)}
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

var errInvalidCredentials = domain.Unauthorized("invalid_credentials", "email or password is incorrect")

/* ---------------------------------------------------- email + password */

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

func (s *Service) Register(ctx context.Context, in RegisterInput, meta ClientMeta) (*Session, error) {
	email := normalizeEmail(in.Email)
	name := strings.TrimSpace(in.DisplayName)
	var nameErr *domain.FieldError
	if name == "" {
		nameErr = &domain.FieldError{Field: "displayName", Message: "display name is required"}
	} else if utf8.RuneCountInString(name) > maxNameLen {
		nameErr = &domain.FieldError{Field: "displayName", Message: fmt.Sprintf("display name must be at most %d characters", maxNameLen)}
	}
	if err := collect(validateEmail(email), validatePassword(in.Password), nameErr); err != nil {
		return nil, err
	}

	hash, err := s.Hasher.Hash(in.Password)
	if err != nil {
		return nil, domain.Internal(err)
	}
	now := s.Now()
	u := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: &hash,
		DisplayName:  name,
		Role:         s.initialRole(email),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.Users.Create(ctx, u); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("email_taken", "an account with this email already exists")
		}
		return nil, domain.Internal(err)
	}
	if err := s.sendVerification(ctx, u); err != nil {
		// Sign-up must not fail because mail is down; the user can resend later.
		s.Logger.WarnContext(ctx, "send verification email failed", "user_id", u.ID, "err", err)
	}
	return s.startSession(ctx, u, meta)
}

func (s *Service) Login(ctx context.Context, email, password string, meta ClientMeta) (*Session, error) {
	email = normalizeEmail(email)
	u, err := s.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, domain.Internal(err)
	}
	if u == nil || !u.HasPassword() {
		_, _ = s.Hasher.Verify(password, s.dummyHash)
		return nil, errInvalidCredentials
	}
	ok, err := s.Hasher.Verify(password, *u.PasswordHash)
	if err != nil {
		return nil, domain.Internal(err)
	}
	if !ok {
		return nil, errInvalidCredentials
	}
	if s.Hasher.NeedsRehash(*u.PasswordHash) {
		if h, err := s.Hasher.Hash(password); err == nil {
			if err := s.Users.UpdatePassword(ctx, u.ID, h); err != nil {
				s.Logger.WarnContext(ctx, "password rehash failed", "user_id", u.ID, "err", err)
			}
		}
	}
	return s.startSession(ctx, u, meta)
}

/* ---------------------------------------------------------------- OAuth */

// OAuthLogin signs in (or up) with a profile returned by Google or Discord.
//
// Linking policy:
//   - a known (provider, subject) pair signs into its user;
//   - otherwise a *verified* provider email is linked to an existing account with that email;
//   - an unverified provider email never takes over an existing account;
//   - otherwise a new password-less account is created.
func (s *Service) OAuthLogin(ctx context.Context, p domain.ExternalProfile, meta ClientMeta) (*Session, error) {
	if p.Subject == "" {
		return nil, domain.Unauthorized("oauth_profile_invalid", "identity provider returned no user id")
	}
	email := normalizeEmail(p.Email)
	if validateEmail(email) != nil {
		return nil, domain.Unauthorized("oauth_email_missing", "identity provider did not share a valid email address")
	}

	ident, err := s.Identities.Get(ctx, p.Provider, p.Subject)
	switch {
	case err == nil:
		u, err := s.Users.GetByID(ctx, ident.UserID)
		if err != nil {
			return nil, domain.Internal(err)
		}
		return s.startSession(ctx, u, meta)
	case !errors.Is(err, domain.ErrNotFound):
		return nil, domain.Internal(err)
	}

	now := s.Now()
	newIdentity := &domain.Identity{ID: uuid.New(), Provider: p.Provider, Subject: p.Subject, Email: email, CreatedAt: now}

	existing, err := s.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, domain.Internal(err)
	}
	if existing != nil {
		if !p.EmailVerified {
			return nil, domain.Conflict("email_unverified_at_provider",
				"an account with this email exists; verify the email at the provider or sign in with your password")
		}
		newIdentity.UserID = existing.ID
		if err := s.Identities.Create(ctx, newIdentity); err != nil {
			return nil, domain.Internal(err)
		}
		if !existing.EmailVerified() {
			if err := s.Users.MarkEmailVerified(ctx, existing.ID, now); err != nil {
				return nil, domain.Internal(err)
			}
			existing.EmailVerifiedAt = &now
		}
		return s.startSession(ctx, existing, meta)
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = email[:strings.Index(email, "@")]
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		name = string([]rune(name)[:maxNameLen])
	}
	u := &domain.User{ID: uuid.New(), Email: email, DisplayName: name, Role: s.initialRole(email), CreatedAt: now, UpdatedAt: now}
	if p.AvatarURL != "" {
		avatar := p.AvatarURL
		u.AvatarURL = &avatar
	}
	if p.EmailVerified {
		u.EmailVerifiedAt = &now
	}
	newIdentity.UserID = u.ID
	if err := s.Users.CreateWithIdentity(ctx, u, newIdentity); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent sign-up; let the client retry.
			return nil, domain.Conflict("email_taken", "an account with this email already exists")
		}
		return nil, domain.Internal(err)
	}
	return s.startSession(ctx, u, meta)
}

/* ------------------------------------------------------------- sessions */

func (s *Service) startSession(ctx context.Context, u *domain.User, meta ClientMeta) (*Session, error) {
	plain, hash, err := security.NewOpaqueToken()
	if err != nil {
		return nil, domain.Internal(err)
	}
	now := s.Now()
	rt := &domain.RefreshToken{
		ID: uuid.New(), UserID: u.ID, FamilyID: uuid.New(), TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.RefreshTTL), UserAgent: truncate(meta.UserAgent, 512), IP: meta.IP, CreatedAt: now,
	}
	if err := s.Refresh.Create(ctx, rt); err != nil {
		return nil, domain.Internal(err)
	}
	return s.session(u, plain, rt.ExpiresAt)
}

func (s *Service) session(u *domain.User, refresh string, refreshExp time.Time) (*Session, error) {
	access, exp, err := s.Tokens.Issue(u.ID, u.Role)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return &Session{User: u, AccessToken: access, AccessExpiresAt: exp, RefreshToken: refresh, RefreshExpiresAt: refreshExp}, nil
}

var errInvalidRefresh = domain.Unauthorized("invalid_refresh_token", "session expired, sign in again")

// RefreshSession rotates a refresh token. Re-using an already rotated token is treated
// as theft: the whole token family is revoked and the caller must sign in again.
func (s *Service) RefreshSession(ctx context.Context, refreshToken string, meta ClientMeta) (*Session, error) {
	if refreshToken == "" {
		return nil, errInvalidRefresh
	}
	now := s.Now()
	cur, err := s.Refresh.GetByHash(ctx, security.HashToken(refreshToken))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errInvalidRefresh
	}
	if err != nil {
		return nil, domain.Internal(err)
	}
	if cur.RevokedAt != nil {
		if cur.ReplacedBy != nil {
			s.Logger.WarnContext(ctx, "refresh token reuse detected; revoking family", "user_id", cur.UserID, "family_id", cur.FamilyID)
			if err := s.Refresh.RevokeFamily(ctx, cur.FamilyID, now); err != nil {
				return nil, domain.Internal(err)
			}
		}
		return nil, errInvalidRefresh
	}
	if !now.Before(cur.ExpiresAt) {
		return nil, errInvalidRefresh
	}

	u, err := s.Users.GetByID(ctx, cur.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errInvalidRefresh
	}
	if err != nil {
		return nil, domain.Internal(err)
	}

	plain, hash, err := security.NewOpaqueToken()
	if err != nil {
		return nil, domain.Internal(err)
	}
	next := &domain.RefreshToken{
		ID: uuid.New(), UserID: u.ID, FamilyID: cur.FamilyID, TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.RefreshTTL), UserAgent: truncate(meta.UserAgent, 512), IP: meta.IP, CreatedAt: now,
	}
	if err := s.Refresh.Rotate(ctx, cur.ID, next, now); err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Code == "token_reused" {
			_ = s.Refresh.RevokeFamily(ctx, cur.FamilyID, now)
			return nil, errInvalidRefresh
		}
		return nil, domain.Internal(err)
	}
	return s.session(u, plain, next.ExpiresAt)
}

// Logout revokes the presented refresh token. Unknown tokens are ignored (idempotent).
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	cur, err := s.Refresh.GetByHash(ctx, security.HashToken(refreshToken))
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return domain.Internal(err)
	}
	if err := s.Refresh.RevokeFamily(ctx, cur.FamilyID, s.Now()); err != nil {
		return domain.Internal(err)
	}
	return nil
}

/* ------------------------------------------------------------- profile */

type Profile struct {
	User       *domain.User
	Identities []domain.Identity
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	u, err := s.Users.GetByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.Unauthorized("user_not_found", "account no longer exists")
	}
	if err != nil {
		return nil, domain.Internal(err)
	}
	ids, err := s.Identities.ListByUser(ctx, userID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return &Profile{User: u, Identities: ids}, nil
}

// SetRole changes a user's platform role. Only admins may call it; admins cannot demote themselves
// (prevents locking the platform out of its last admin by accident).
func (s *Service) SetRole(ctx context.Context, actor domain.AccessClaims, target uuid.UUID, role domain.Role) (*domain.User, error) {
	if actor.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("forbidden", "admin role required")
	}
	if !role.Valid() {
		return nil, domain.Validation(domain.FieldError{Field: "role", Message: "role must be one of user, publisher, admin"})
	}
	if actor.UserID == target && role != domain.RoleAdmin {
		return nil, domain.Validation(domain.FieldError{Field: "role", Message: "admins cannot demote themselves"})
	}
	if err := s.Users.UpdateRole(ctx, target, role); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("user_not_found", "user not found")
		}
		return nil, domain.Internal(err)
	}
	u, err := s.Users.GetByID(ctx, target)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return u, nil
}

func (s *Service) initialRole(email string) domain.Role {
	if _, ok := s.adminEmails[email]; ok {
		return domain.RoleAdmin
	}
	return domain.RoleUser
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
