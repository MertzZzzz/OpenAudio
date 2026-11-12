package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
)

func TestEnsureAdminCreatesVerifiedAdmin(t *testing.T) {
	e := newEnv(t)
	created, err := e.svc.EnsureAdmin(ctx, auth.AdminAccount{Email: " Root@OpenAudio.dev ", Password: "admin-password-1", DisplayName: "Root"})
	require.NoError(t, err)
	assert.True(t, created)

	s, err := e.svc.Login(ctx, "root@openaudio.dev", "admin-password-1", meta)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, s.User.Role)
	assert.True(t, s.User.EmailVerified())
	assert.Equal(t, "Root", s.User.DisplayName)

	// Idempotent: a second start changes nothing and keeps sessions.
	created, err = e.svc.EnsureAdmin(ctx, auth.AdminAccount{Email: "root@openaudio.dev", Password: "admin-password-1"})
	require.NoError(t, err)
	assert.False(t, created)
	_, err = e.svc.RefreshSession(ctx, s.RefreshToken, meta)
	require.NoError(t, err)
}

func TestEnsureAdminPromotesExistingUserAndSyncsPassword(t *testing.T) {
	e := newEnv(t)
	s := e.register(t, "owner@example.com")
	require.Equal(t, domain.RoleUser, s.User.Role)

	_, err := e.svc.EnsureAdmin(ctx, auth.AdminAccount{Email: "owner@example.com", Password: "new-admin-password"})
	require.NoError(t, err)

	_, err = e.svc.Login(ctx, "owner@example.com", "long-enough-pw", meta)
	assert.Equal(t, "invalid_credentials", codeOf(t, err), "old password no longer works")
	_, err = e.svc.RefreshSession(ctx, s.RefreshToken, meta)
	require.Error(t, err, "sessions are revoked when the password changes")

	s2, err := e.svc.Login(ctx, "owner@example.com", "new-admin-password", meta)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, s2.User.Role)
	assert.True(t, s2.User.EmailVerified())
}

func TestEnsureAdminRejectsWeakPassword(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.EnsureAdmin(ctx, auth.AdminAccount{Email: "root@openaudio.dev", Password: "short"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADMIN_PASSWORD")
}
