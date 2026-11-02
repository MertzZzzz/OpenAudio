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
