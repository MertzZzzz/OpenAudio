package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	require.NoError(t, err)
	assert.Equal(t, "development", c.Env)
	assert.Equal(t, "http://localhost:5173", c.PublicURL, "dev API is reached through the Vite proxy")
	assert.Equal(t, []string{"http://localhost:5173"}, c.CORSOrigins)
	assert.Len(t, c.JWTSecret, 64, "random dev secret")
	assert.False(t, c.CookieSecure)
	assert.Equal(t, 15*time.Minute, c.AccessTTL)
	assert.NotEmpty(t, c.Warnings)
}

func TestOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{
		"APP_ENV": "production", "PUBLIC_URL": "https://api.openaudio.dev", "WEB_URL": "https://openaudio.dev/",
		"JWT_SECRET": strings.Repeat("x", 40), "ACCESS_TOKEN_TTL": "5m", "ADMIN_EMAILS": "a@x.io, b@x.io",
		"GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "s", "CORS_ORIGINS": "https://openaudio.dev,https://staging.openaudio.dev",
	}))
	require.NoError(t, err)
	assert.True(t, c.CookieSecure, "https public URL → secure cookies")
	assert.Equal(t, "https://openaudio.dev", c.WebURL)
	assert.Equal(t, 5*time.Minute, c.AccessTTL)
	assert.Equal(t, []string{"a@x.io", "b@x.io"}, c.AdminEmails)
	assert.True(t, c.Google.Enabled())
	assert.False(t, c.Discord.Enabled())
	assert.Len(t, c.CORSOrigins, 2)
}

func TestValidation(t *testing.T) {
	_, err := Load(env(map[string]string{"APP_ENV": "production"}))
	assert.ErrorContains(t, err, "JWT_SECRET is required in production")
	assert.ErrorContains(t, err, "COOKIE_SECURE must be true")

	_, err = Load(env(map[string]string{"JWT_SECRET": "short", "ACCESS_TOKEN_TTL": "soon", "WEB_URL": "nope", "APP_ENV": "staging"}))
	assert.ErrorContains(t, err, "at least 32 bytes")
	assert.ErrorContains(t, err, "ACCESS_TOKEN_TTL")
	assert.ErrorContains(t, err, "WEB_URL must be an absolute URL")
	assert.ErrorContains(t, err, "APP_ENV")
}

func TestBaseDomain(t *testing.T) {
	c, err := Load(env(map[string]string{"BASE_DOMAIN": "OpenAudio.Example.com"}))
	require.NoError(t, err)
	assert.Equal(t, "openaudio.example.com", c.BaseDomain)
	assert.Equal(t, "https://openaudio.example.com", c.WebURL)
	assert.Equal(t, "https://openaudio.example.com", c.PublicURL)
	assert.Equal(t, []string{"https://openaudio.example.com"}, c.CORSOrigins)
	assert.True(t, c.CookieSecure)
	assert.Equal(t, "https://openaudio.example.com/auth/callback", c.AuthCallbackURL)
	assert.Equal(t, "OpenAudio <no-reply@openaudio.example.com>", c.SMTP.From)

	c, err = Load(env(map[string]string{"BASE_DOMAIN": "localhost:5173"}))
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5173", c.WebURL, "local hosts default to http")

	c, err = Load(env(map[string]string{"BASE_DOMAIN": "audio.lan:8443", "BASE_SCHEME": "https", "PUBLIC_URL": "https://api.audio.lan"}))
	require.NoError(t, err)
	assert.Equal(t, "https://audio.lan:8443", c.WebURL)
	assert.Equal(t, "https://api.audio.lan", c.PublicURL, "explicit URLs win over BASE_DOMAIN")

	_, err = Load(env(map[string]string{"BASE_DOMAIN": "https://openaudio.example.com"}))
	assert.ErrorContains(t, err, "BASE_DOMAIN must be a host name")
}

func TestDefaultAdmin(t *testing.T) {
	c, err := Load(env(map[string]string{"ADMIN_EMAIL": "Root@Example.com", "ADMIN_PASSWORD": "admin-password-1", "ADMIN_EMAILS": "a@x.io"}))
	require.NoError(t, err)
	assert.Equal(t, DefaultAdmin{Email: "root@example.com", Password: "admin-password-1", DisplayName: "Admin"}, c.DefaultAdmin)
	assert.Equal(t, []string{"a@x.io", "root@example.com"}, c.AdminEmails)

	_, err = Load(env(map[string]string{"ADMIN_EMAIL": "root@example.com"}))
	assert.ErrorContains(t, err, "ADMIN_EMAIL and ADMIN_PASSWORD must be set together")

	_, err = Load(env(map[string]string{"APP_ENV": "production", "JWT_SECRET": strings.Repeat("x", 40), "BASE_DOMAIN": "openaudio.dev",
		"ADMIN_EMAIL": "root@example.com", "ADMIN_PASSWORD": ExampleAdminPassword}))
	assert.ErrorContains(t, err, "example value")
}

func TestAuthRedirect(t *testing.T) {
	c, err := Load(env(nil))
	require.NoError(t, err)
	assert.Equal(t, "/", c.AuthRedirectPath)
	assert.Equal(t, "http://localhost:5173/auth/callback", c.AuthCallbackURL)

	c, err = Load(env(map[string]string{"AUTH_REDIRECT_PATH": "/library", "AUTH_CALLBACK_URL": "https://app.example.com/oauth/done"}))
	require.NoError(t, err)
	assert.Equal(t, "/library", c.AuthRedirectPath)
	assert.Equal(t, "https://app.example.com/oauth/done", c.AuthCallbackURL)

	_, err = Load(env(map[string]string{"AUTH_REDIRECT_PATH": "https://evil.example", "AUTH_CALLBACK_URL": "javascript:alert(1)"}))
	assert.ErrorContains(t, err, "AUTH_REDIRECT_PATH must be a path")
	assert.ErrorContains(t, err, "AUTH_CALLBACK_URL must be an absolute URL")
	_, err = Load(env(map[string]string{"AUTH_REDIRECT_PATH": "//evil.example"}))
	assert.ErrorContains(t, err, "AUTH_REDIRECT_PATH")
}
