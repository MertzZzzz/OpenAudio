package auth_test

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/mailer"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/memory"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/security"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
)

type env struct {
	svc   *auth.Service
	store *memory.Store
	mail  *mailer.Recorder
	jwt   *security.JWTIssuer
	now   *time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := memory.NewStore()
	rec := &mailer.Recorder{}
	jwtIssuer, err := security.NewJWTIssuer([]byte(strings.Repeat("k", 32)), "openaudio", 15*time.Minute, clock)
	require.NoError(t, err)
	svc, err := auth.NewService(auth.Deps{
		Users: store.Users(), Identities: store.Identities(), Refresh: store.Refresh(), OneTime: store.OneTime(),
		Hasher: security.NewArgon2Hasher(security.Argon2Params{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}),
		Tokens: jwtIssuer, Mailer: rec, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: clock,
	}, auth.Config{RefreshTTL: 24 * time.Hour, WebURL: "http://web", AdminEmails: []string{"Boss@OpenAudio.dev"}})
	require.NoError(t, err)
	return &env{svc: svc, store: store, mail: rec, jwt: jwtIssuer, now: &now}
}

var ctx = context.Background()
var meta = auth.ClientMeta{UserAgent: "test", IP: "127.0.0.1"}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	return de.Code
}

func (e *env) register(t *testing.T, email string) *auth.Session {
	t.Helper()
	s, err := e.svc.Register(ctx, auth.RegisterInput{Email: email, Password: "long-enough-pw", DisplayName: "Producer"}, meta)
	require.NoError(t, err)
	return s
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-%]+)`)

func tokenFromMail(t *testing.T, body string) string {
	t.Helper()
	m := tokenRe.FindStringSubmatch(body)
	require.Len(t, m, 2, body)
	return m[1]
}

func TestRegister(t *testing.T) {
	e := newEnv(t)
	s := e.register(t, "  Producer@Example.COM ")
	assert.Equal(t, "producer@example.com", s.User.Email, "email is normalised")
	assert.Equal(t, domain.RoleUser, s.User.Role)
	assert.False(t, s.User.EmailVerified())
	assert.NotEmpty(t, s.RefreshToken)

	claims, err := e.jwt.Parse(s.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.User.ID, claims.UserID)

	assert.Equal(t, "producer@example.com", e.mail.Last().To)
	assert.Contains(t, e.mail.Last().Body, "http://web/verify-email?token=")
}

func TestRegisterValidationAndConflicts(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Register(ctx, auth.RegisterInput{Email: "nope", Password: "short", DisplayName: ""}, meta)
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	assert.Equal(t, domain.KindValidation, de.Kind)
	fields := map[string]bool{}
	for _, f := range de.Fields {
		fields[f.Field] = true
	}
	assert.Equal(t, map[string]bool{"email": true, "password": true, "displayName": true}, fields)

	e.register(t, "dup@example.com")
	_, err = e.svc.Register(ctx, auth.RegisterInput{Email: "DUP@example.com", Password: "long-enough-pw", DisplayName: "x"}, meta)
	assert.Equal(t, "email_taken", codeOf(t, err))
}

func TestAdminBootstrap(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, domain.RoleAdmin, e.register(t, "boss@openaudio.dev").User.Role)
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	e.register(t, "a@example.com")

	s, err := e.svc.Login(ctx, "A@example.com", "long-enough-pw", meta)
	require.NoError(t, err)
	assert.Equal(t, "a@example.com", s.User.Email)

	_, err = e.svc.Login(ctx, "a@example.com", "wrong-password", meta)
	assert.Equal(t, "invalid_credentials", codeOf(t, err))
	_, err = e.svc.Login(ctx, "ghost@example.com", "whatever-pw", meta)
	assert.Equal(t, "invalid_credentials", codeOf(t, err), "unknown email looks the same as a wrong password")
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	e := newEnv(t)
	s1 := e.register(t, "r@example.com")

	s2, err := e.svc.RefreshSession(ctx, s1.RefreshToken, meta)
	require.NoError(t, err)
	assert.NotEqual(t, s1.RefreshToken, s2.RefreshToken)

	// Replaying the rotated token revokes the whole family, including s2.
	_, err = e.svc.RefreshSession(ctx, s1.RefreshToken, meta)
	assert.Equal(t, "invalid_refresh_token", codeOf(t, err))
	_, err = e.svc.RefreshSession(ctx, s2.RefreshToken, meta)
	assert.Equal(t, "invalid_refresh_token", codeOf(t, err))

	// A separate login (another device) is unaffected.
	s3, err := e.svc.Login(ctx, "r@example.com", "long-enough-pw", meta)
	require.NoError(t, err)
	_, err = e.svc.RefreshSession(ctx, s3.RefreshToken, meta)
	assert.NoError(t, err)
}

func TestRefreshExpiryAndGarbage(t *testing.T) {
	e := newEnv(t)
	s := e.register(t, "x@example.com")
	_, err := e.svc.RefreshSession(ctx, "garbage", meta)
	assert.Equal(t, "invalid_refresh_token", codeOf(t, err))
	*e.now = e.now.Add(25 * time.Hour)
	_, err = e.svc.RefreshSession(ctx, s.RefreshToken, meta)
	assert.Equal(t, "invalid_refresh_token", codeOf(t, err))
}

func TestLogoutRevokesFamily(t *testing.T) {
	e := newEnv(t)
	s1 := e.register(t, "l@example.com")
	s2, _ := e.svc.RefreshSession(ctx, s1.RefreshToken, meta)
	require.NoError(t, e.svc.Logout(ctx, s2.RefreshToken))
	_, err := e.svc.RefreshSession(ctx, s2.RefreshToken, meta)
	assert.Error(t, err)
	assert.NoError(t, e.svc.Logout(ctx, "unknown"), "logout is idempotent")
}

func google(sub, email string, verified bool) domain.ExternalProfile {
	return domain.ExternalProfile{Provider: domain.ProviderGoogle, Subject: sub, Email: email, EmailVerified: verified, Name: "G User", AvatarURL: "https://img/a.png"}
}

func TestOAuthCreatesUserAndReusesIdentity(t *testing.T) {
	e := newEnv(t)
	s, err := e.svc.OAuthLogin(ctx, google("g-1", "New@Gmail.com", true), meta)
	require.NoError(t, err)
	assert.Equal(t, "new@gmail.com", s.User.Email)
	assert.True(t, s.User.EmailVerified())
	assert.False(t, s.User.HasPassword())
	require.NotNil(t, s.User.AvatarURL)

	again, err := e.svc.OAuthLogin(ctx, google("g-1", "changed@gmail.com", true), meta)
	require.NoError(t, err)
	assert.Equal(t, s.User.ID, again.User.ID, "identity wins over email")

	// OAuth-only accounts cannot sign in with a password until they set one.
	_, err = e.svc.Login(ctx, "new@gmail.com", "anything-long", meta)
	assert.Equal(t, "invalid_credentials", codeOf(t, err))
}

func TestOAuthLinksVerifiedEmailToExistingAccount(t *testing.T) {
	e := newEnv(t)
	reg := e.register(t, "same@example.com")
	discord := domain.ExternalProfile{Provider: domain.ProviderDiscord, Subject: "123", Email: "same@example.com", EmailVerified: true, Name: "disc"}
	s, err := e.svc.OAuthLogin(ctx, discord, meta)
	require.NoError(t, err)
	assert.Equal(t, reg.User.ID, s.User.ID)
	assert.True(t, s.User.EmailVerified(), "verified provider email verifies the account")

	p, err := e.svc.Me(ctx, reg.User.ID)
	require.NoError(t, err)
	require.Len(t, p.Identities, 1)
	assert.Equal(t, domain.ProviderDiscord, p.Identities[0].Provider)
}

func TestOAuthRefusesUnverifiedTakeover(t *testing.T) {
	e := newEnv(t)
	e.register(t, "victim@example.com")
	_, err := e.svc.OAuthLogin(ctx, google("attacker", "victim@example.com", false), meta)
	assert.Equal(t, "email_unverified_at_provider", codeOf(t, err))
}

func TestOAuthRequiresEmail(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.OAuthLogin(ctx, google("g-2", "", true), meta)
	assert.Equal(t, "oauth_email_missing", codeOf(t, err))
}

func TestEmailVerification(t *testing.T) {
	e := newEnv(t)
	s := e.register(t, "v@example.com")
	first := tokenFromMail(t, e.mail.Last().Body)

	require.NoError(t, e.svc.ResendVerification(ctx, s.User.ID))
	second := tokenFromMail(t, e.mail.Last().Body)

	assert.Equal(t, "token_invalid", codeOf(t, e.svc.VerifyEmail(ctx, first)), "resend invalidates older links")
	require.NoError(t, e.svc.VerifyEmail(ctx, second))
	assert.Equal(t, "token_invalid", codeOf(t, e.svc.VerifyEmail(ctx, second)), "links are single-use")

	p, _ := e.svc.Me(ctx, s.User.ID)
	assert.True(t, p.User.EmailVerified())
	assert.Equal(t, "email_already_verified", codeOf(t, e.svc.ResendVerification(ctx, s.User.ID)))
}

func TestPasswordReset(t *testing.T) {
	e := newEnv(t)
	s := e.register(t, "p@example.com")

	sent := len(e.mail.Sent)
	require.NoError(t, e.svc.RequestPasswordReset(ctx, "ghost@example.com"), "unknown emails succeed silently")
	assert.Len(t, e.mail.Sent, sent)

	require.NoError(t, e.svc.RequestPasswordReset(ctx, "P@example.com"))
	tok := tokenFromMail(t, e.mail.Last().Body)

	assert.Equal(t, "validation_failed", codeOf(t, e.svc.ResetPassword(ctx, tok, "short")))
	require.NoError(t, e.svc.ResetPassword(ctx, tok, "brand-new-password"))

	_, err := e.svc.RefreshSession(ctx, s.RefreshToken, meta)
	assert.Error(t, err, "reset signs out existing sessions")
	_, err = e.svc.Login(ctx, "p@example.com", "brand-new-password", meta)
	assert.NoError(t, err)

	*e.now = e.now.Add(2 * time.Hour)
	require.NoError(t, e.svc.RequestPasswordReset(ctx, "p@example.com"))
	expired := tokenFromMail(t, e.mail.Last().Body)
	*e.now = e.now.Add(2 * time.Hour)
	assert.Equal(t, "token_invalid", codeOf(t, e.svc.ResetPassword(ctx, expired, "another-password")))
}

func TestSetRole(t *testing.T) {
	e := newEnv(t)
	admin := e.register(t, "boss@openaudio.dev")
	user := e.register(t, "u@example.com")
	adminClaims := domain.AccessClaims{UserID: admin.User.ID, Role: domain.RoleAdmin}

	u, err := e.svc.SetRole(ctx, adminClaims, user.User.ID, domain.RolePublisher)
	require.NoError(t, err)
	assert.Equal(t, domain.RolePublisher, u.Role)

	_, err = e.svc.SetRole(ctx, domain.AccessClaims{UserID: user.User.ID, Role: domain.RolePublisher}, user.User.ID, domain.RoleAdmin)
	assert.Equal(t, "forbidden", codeOf(t, err))
	_, err = e.svc.SetRole(ctx, adminClaims, admin.User.ID, domain.RoleUser)
	assert.Equal(t, "validation_failed", codeOf(t, err))
	_, err = e.svc.SetRole(ctx, adminClaims, uuid.New(), domain.RoleUser)
	assert.Equal(t, "user_not_found", codeOf(t, err))
	_, err = e.svc.SetRole(ctx, adminClaims, user.User.ID, "root")
	assert.Equal(t, "validation_failed", codeOf(t, err))
}
