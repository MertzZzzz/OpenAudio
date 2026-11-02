// Package config loads settings from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type OAuthClient struct {
	ClientID     string
	ClientSecret string
}

func (c OAuthClient) Enabled() bool { return c.ClientID != "" && c.ClientSecret != "" }

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Config struct {
	Env      string // development | production
	LogLevel string
	HTTPAddr string

	// PublicURL is where browsers reach the API (used for OAuth redirect URIs).
	// In development the Vite dev server proxies /api, so it equals WebURL and cookies stay first-party.
	PublicURL   string
	WebURL      string
	CORSOrigins []string
	TrustProxy  bool

	DatabaseURL string
	AutoMigrate bool

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	JWTSecret    []byte
	JWTIssuer    string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool
	AdminEmails  []string

	Google  OAuthClient
	Discord OAuthClient
	SMTP    SMTP

	// Warnings are surfaced in the log on start-up (e.g. generated dev secrets).
	Warnings []string
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads the environment. getenv is injectable for tests.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	var errs []error
	str := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	boolean := func(key string, def bool) bool {
		v := str(key, "")
		if v == "" {
			return def
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		return b
	}
	duration := func(key string, def time.Duration) time.Duration {
		v := str(key, "")
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s: invalid duration %q", key, v))
		}
		return d
	}
	list := func(key string) []string {
		var out []string
		for _, p := range strings.Split(str(key, ""), ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}

	c := Config{
		Env:         str("APP_ENV", "development"),
		LogLevel:    str("LOG_LEVEL", "info"),
		HTTPAddr:    str("HTTP_ADDR", ":8080"),
		WebURL:      strings.TrimRight(str("WEB_URL", "http://localhost:5173"), "/"),
		TrustProxy:  boolean("TRUST_PROXY", false),
		DatabaseURL: str("DATABASE_URL", "postgres://openaudio:openaudio@localhost:5432/openaudio?sslmode=disable"),
		AutoMigrate: boolean("AUTO_MIGRATE", true),
		S3Endpoint:  str("S3_ENDPOINT", "http://localhost:9000"),
		S3AccessKey: str("S3_ACCESS_KEY", "openaudio"),
		S3SecretKey: str("S3_SECRET_KEY", "openaudio-secret"),
		S3Bucket:    str("S3_BUCKET", "openaudio-dev"),
		S3Region:    str("S3_REGION", "us-east-1"),
		JWTIssuer:   str("JWT_ISSUER", "openaudio"),
		AccessTTL:   duration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:  duration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		AdminEmails: list("ADMIN_EMAILS"),
		Google:      OAuthClient{ClientID: str("GOOGLE_CLIENT_ID", ""), ClientSecret: str("GOOGLE_CLIENT_SECRET", "")},
		Discord:     OAuthClient{ClientID: str("DISCORD_CLIENT_ID", ""), ClientSecret: str("DISCORD_CLIENT_SECRET", "")},
		SMTP: SMTP{
			Host: str("SMTP_HOST", ""), Username: str("SMTP_USERNAME", ""), Password: str("SMTP_PASSWORD", ""),
			From: str("SMTP_FROM", "OpenAudio <no-reply@openaudio.local>"),
		},
	}
	c.PublicURL = strings.TrimRight(str("PUBLIC_URL", c.WebURL), "/")
	c.CORSOrigins = list("CORS_ORIGINS")
	if len(c.CORSOrigins) == 0 {
		c.CORSOrigins = []string{c.WebURL}
	}
	c.CookieSecure = boolean("COOKIE_SECURE", strings.HasPrefix(c.PublicURL, "https://"))
	if p := str("SMTP_PORT", "587"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("SMTP_PORT: %w", err))
		}
		c.SMTP.Port = n
	}

	if c.Env != "development" && c.Env != "production" {
		errs = append(errs, fmt.Errorf("APP_ENV must be development or production, got %q", c.Env))
	}
	for key, raw := range map[string]string{"WEB_URL": c.WebURL, "PUBLIC_URL": c.PublicURL} {
		if u, err := url.Parse(raw); err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Errorf("%s must be an absolute URL, got %q", key, raw))
		}
	}

	switch secret := getenv("JWT_SECRET"); {
	case len(secret) >= 32:
		c.JWTSecret = []byte(secret)
	case secret != "":
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 bytes"))
	case c.IsProduction():
		errs = append(errs, errors.New("JWT_SECRET is required in production"))
	default:
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		c.JWTSecret = []byte(hex.EncodeToString(b))
		c.Warnings = append(c.Warnings, "JWT_SECRET not set: generated a random one; sessions will not survive restarts")
	}
	if c.IsProduction() && !c.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE must be true in production (serve the API over https)"))
	}
	if !c.Google.Enabled() && !c.Discord.Enabled() {
		c.Warnings = append(c.Warnings, "no OAuth providers configured (set GOOGLE_CLIENT_ID/SECRET, DISCORD_CLIENT_ID/SECRET)")
	}
	if c.SMTP.Host == "" {
		c.Warnings = append(c.Warnings, "SMTP_HOST not set: emails are written to the log")
	}
	return c, errors.Join(errs...)
}
