package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/MertzZzzz/OpenAudio/api/internal/adapter/http"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/mailer"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/memory"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/oauth"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/security"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/uploads"
)

/* ------------------------------------------------------------ test rig */

type fakeProvider struct {
	name    domain.AuthProvider
	profile domain.ExternalProfile
	err     error
	gotCode string
	gotVer  string
}

func (f *fakeProvider) Name() domain.AuthProvider { return f.name }
func (f *fakeProvider) AuthCodeURL(state, verifier string) string {
	return "https://idp.example/authorize?state=" + url.QueryEscape(state) + "&v=" + url.QueryEscape(verifier)
}
func (f *fakeProvider) Exchange(_ context.Context, code, verifier string) (*domain.ExternalProfile, error) {
	f.gotCode, f.gotVer = code, verifier
	if f.err != nil {
		return nil, f.err
	}
	p := f.profile
	return &p, nil
}

type rig struct {
	blobs   *memory.Blobs
	h       http.Handler
	mail    *mailer.Recorder
	google  *fakeProvider
	checkOK *bool
}

const webURL = "http://localhost:5173"

func newRig(t *testing.T, limiter *httpapi.RateLimiter) *rig {
	t.Helper()
	store := memory.NewStore()
	rec := &mailer.Recorder{}
	jwtIssuer, err := security.NewJWTIssuer([]byte(strings.Repeat("k", 32)), "openaudio", 15*time.Minute, nil)
	require.NoError(t, err)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := auth.NewService(auth.Deps{
		Users: store.Users(), Identities: store.Identities(), Refresh: store.Refresh(), OneTime: store.OneTime(),
		Hasher: security.NewArgon2Hasher(security.Argon2Params{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}),
		Tokens: jwtIssuer, Mailer: rec, Logger: log,
	}, auth.Config{WebURL: webURL, AdminEmails: []string{"admin@openaudio.dev"}})
	require.NoError(t, err)

	google := &fakeProvider{name: domain.ProviderGoogle, profile: domain.ExternalProfile{
		Provider: domain.ProviderGoogle, Subject: "g-42", Email: "gina@gmail.com", EmailVerified: true, Name: "Gina",
	}}
	ok := true
	cat := memory.NewCatalog(store)
	blobs := memory.NewBlobs()
	validator, err := manifest.NewValidator()
	require.NoError(t, err)
	pubSvc := publishing.NewService(publishing.Deps{Users: store.Users(), Publishers: cat.Publishers(), Packages: cat.Packages(), Releases: cat.Releases(), Blobs: blobs, Validator: validator, Logger: log})
	upSvc := uploads.NewService(uploads.Deps{Uploads: cat.Uploads(), Publishers: cat.Publishers(), Blobs: blobs, Logger: log}, uploads.Config{PartSize: 4})
	h := httpapi.NewHandler(httpapi.Options{
		Auth: svc, Publishing: pubSvc, Uploads: upSvc, Tokens: jwtIssuer, Providers: []oauth.Provider{google}, Logger: log,
		WebURL: webURL, CORSOrigins: []string{webURL}, StateKey: []byte("state-key"), Limiter: limiter,
		Checks: []httpapi.HealthCheck{{Name: "database", Check: func(context.Context) error {
			if ok {
				return nil
			}
			return errors.New("connection refused")
		}}},
	})
	return &rig{h: h, mail: rec, google: google, checkOK: &ok, blobs: blobs}
}

type req struct {
	method, path string
	body         any
	bearer       string
	cookies      []*http.Cookie
	headers      map[string]string
}

func (r *rig) do(t *testing.T, q req) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if q.body != nil {
		if s, ok := q.body.(string); ok {
			body = strings.NewReader(s)
		} else if b, ok := q.body.([]byte); ok {
			body = bytes.NewReader(b)
		} else {
			b, _ := json.Marshal(q.body)
			body = bytes.NewReader(b)
		}
	}
	hr := httptest.NewRequest(q.method, q.path, body)
	if _, raw := q.body.([]byte); q.body != nil && !raw {
		hr.Header.Set("Content-Type", "application/json")
	}
	if q.bearer != "" {
		hr.Header.Set("Authorization", "Bearer "+q.bearer)
	}
	for k, v := range q.headers {
		hr.Header.Set(k, v)
	}
	for _, c := range q.cookies {
		hr.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, hr)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (r *rig) register(t *testing.T, email string) (access string, refresh *http.Cookie) {
	t.Helper()
	w := r.do(t, req{method: "POST", path: "/api/v1/auth/register", body: map[string]string{"email": email, "password": "a-good-password", "displayName": "Tester"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return decode(t, w)["accessToken"].(string), cookie(w, "oa_refresh")
}

/* ----------------------------------------------------------------- tests */

func TestRegisterLoginMe(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "POST", path: "/api/v1/auth/register", body: map[string]string{"email": "Prod@Example.com", "password": "a-good-password", "displayName": "Prod"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	body := decode(t, w)
	assert.Equal(t, "Bearer", body["tokenType"])
	assert.InDelta(t, 900, body["expiresIn"], 2)
	assert.NotEmpty(t, body["refreshToken"])
	user := body["user"].(map[string]any)
	assert.Equal(t, "prod@example.com", user["email"])
	assert.Equal(t, "user", user["role"])
	assert.Equal(t, false, user["emailVerified"])

	c := cookie(w, "oa_refresh")
	require.NotNil(t, c)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, "/api/v1/auth", c.Path)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, body["refreshToken"], c.Value)

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "prod@example.com", "password": "a-good-password"}})
	require.Equal(t, http.StatusOK, w.Code)
	access := decode(t, w)["accessToken"].(string)

	w = r.do(t, req{method: "GET", path: "/api/v1/me", bearer: access})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "prod@example.com", decode(t, w)["email"])
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestAuthErrorsUseUnifiedFormat(t *testing.T) {
	r := newRig(t, nil)

	w := r.do(t, req{method: "GET", path: "/api/v1/me"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	b := decode(t, w)
	assert.Equal(t, "unauthenticated", b["code"])
	assert.NotEmpty(t, b["requestId"])

	w = r.do(t, req{method: "GET", path: "/api/v1/me", bearer: "garbage"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "invalid_token", decode(t, w)["code"])

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/register", body: map[string]string{"email": "bad", "password": "short", "displayName": ""}})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	b = decode(t, w)
	assert.Equal(t, "validation_failed", b["code"])
	issues := b["issues"].([]any)
	assert.Len(t, issues, 3)
	assert.Equal(t, map[string]any{"severity": "error", "path": "email", "message": "enter a valid email address"}, issues[0])

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: `{"email":"a@b.co","password":"x","admin":true}`})
	assert.Equal(t, "invalid_json", decode(t, w)["code"], "unknown fields are rejected")

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: `email=a`, headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}})
	assert.Equal(t, "unsupported_media_type", decode(t, w)["code"])

	r.register(t, "taken@example.com")
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/register", body: map[string]string{"email": "taken@example.com", "password": "a-good-password", "displayName": "x"}})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "email_taken", decode(t, w)["code"])

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "taken@example.com", "password": "nope-nope-nope"}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "invalid_credentials", decode(t, w)["code"])

	w = r.do(t, req{method: "GET", path: "/api/v1/nope"})
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "route_not_found", decode(t, w)["code"])
}

func TestRefreshWithCookieAndBody(t *testing.T) {
	r := newRig(t, nil)
	_, c1 := r.register(t, "r@example.com")

	w := r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", cookies: []*http.Cookie{c1}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	c2 := cookie(w, "oa_refresh")
	require.NotNil(t, c2)
	assert.NotEqual(t, c1.Value, c2.Value)

	// Native clients may send the token in the body.
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", body: map[string]string{"refreshToken": c2.Value}})
	require.Equal(t, http.StatusOK, w.Code)
	c3 := cookie(w, "oa_refresh")

	// Replaying a rotated token kills the family and clears the cookie.
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", cookies: []*http.Cookie{c1}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, -1, cookie(w, "oa_refresh").MaxAge)
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", cookies: []*http.Cookie{c3}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh"})
	assert.Equal(t, "invalid_refresh_token", decode(t, w)["code"])
}

func TestLogout(t *testing.T) {
	r := newRig(t, nil)
	_, c := r.register(t, "bye@example.com")
	w := r.do(t, req{method: "POST", path: "/api/v1/auth/logout", cookies: []*http.Cookie{c}})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, -1, cookie(w, "oa_refresh").MaxAge)
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", cookies: []*http.Cookie{c}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRateLimitOnCredentialEndpoints(t *testing.T) {
	r := newRig(t, httpapi.NewRateLimiter(1, 2))
	login := req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "x@example.com", "password": "whatever-123"}}
	assert.Equal(t, http.StatusUnauthorized, r.do(t, login).Code)
	assert.Equal(t, http.StatusUnauthorized, r.do(t, login).Code)
	w := r.do(t, login)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "rate_limited", decode(t, w)["code"])
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	// Limits are per endpoint: refresh is not limited by failed logins.
	assert.Equal(t, http.StatusUnauthorized, r.do(t, req{method: "POST", path: "/api/v1/auth/refresh"}).Code)
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-%]+)`)

func TestEmailVerificationAndPasswordReset(t *testing.T) {
	r := newRig(t, nil)
	access, _ := r.register(t, "mail@example.com")
	tok := tokenRe.FindStringSubmatch(r.mail.Last().Body)[1]

	w := r.do(t, req{method: "POST", path: "/api/v1/auth/email/verify", body: map[string]string{"token": tok}})
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = r.do(t, req{method: "GET", path: "/api/v1/me", bearer: access})
	assert.Equal(t, true, decode(t, w)["emailVerified"])
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/email/verify", body: map[string]string{"token": tok}})
	assert.Equal(t, "token_invalid", decode(t, w)["code"])

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]string{"email": "nobody@example.com"}})
	assert.Equal(t, http.StatusAccepted, w.Code, "no account enumeration")
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]string{"email": "mail@example.com"}})
	assert.Equal(t, http.StatusAccepted, w.Code)
	reset := tokenRe.FindStringSubmatch(r.mail.Last().Body)[1]
	assert.Contains(t, r.mail.Last().Body, webURL+"/reset-password?token=")

	w = r.do(t, req{method: "POST", path: "/api/v1/auth/password/reset", body: map[string]string{"token": reset, "password": "the-new-password"}})
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "mail@example.com", "password": "the-new-password"}})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOAuthFlow(t *testing.T) {
	r := newRig(t, nil)

	w := r.do(t, req{method: "GET", path: "/api/v1/auth/providers"})
	assert.Equal(t, map[string]any{"email": true, "oauth": []any{map[string]any{"id": "google", "startUrl": "/api/v1/auth/oauth/google/start"}}}, decode(t, w))

	w = r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/discord/start"})
	assert.Equal(t, http.StatusNotFound, w.Code, "disabled provider")

	w = r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/start?returnTo=/library"})
	require.Equal(t, http.StatusFound, w.Code)
	loc, _ := url.Parse(w.Header().Get("Location"))
	assert.Equal(t, "idp.example", loc.Host)
	state, verifier := loc.Query().Get("state"), loc.Query().Get("v")
	require.NotEmpty(t, state)
	require.NotEmpty(t, verifier)
	stateCookie := cookie(w, "oa_oauth")
	require.NotNil(t, stateCookie)
	assert.True(t, stateCookie.HttpOnly)
	assert.Equal(t, "/api/v1/auth/oauth", stateCookie.Path)

	w = r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=abc&state=" + url.QueryEscape(state), cookies: []*http.Cookie{stateCookie}})
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, webURL+"/auth/callback?returnTo=%2Flibrary", w.Header().Get("Location"))
	assert.Equal(t, "abc", r.google.gotCode)
	assert.Equal(t, verifier, r.google.gotVer, "PKCE verifier survives the round trip")
	assert.Equal(t, -1, cookie(w, "oa_oauth").MaxAge, "state cookie is consumed")
	refresh := cookie(w, "oa_refresh")
	require.NotNil(t, refresh)

	// The SPA then exchanges the cookie for an access token.
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/refresh", cookies: []*http.Cookie{refresh}})
	require.Equal(t, http.StatusOK, w.Code)
	access := decode(t, w)["accessToken"].(string)
	w = r.do(t, req{method: "GET", path: "/api/v1/me", bearer: access})
	me := decode(t, w)
	assert.Equal(t, "gina@gmail.com", me["email"])
	assert.Equal(t, true, me["emailVerified"])
	assert.Equal(t, false, me["hasPassword"])
	assert.Equal(t, "google", me["identities"].([]any)[0].(map[string]any)["provider"])
}

func TestOAuthCallbackRejections(t *testing.T) {
	r := newRig(t, nil)
	start := func(returnTo string) (string, *http.Cookie) {
		w := r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/start?returnTo=" + url.QueryEscape(returnTo)})
		loc, _ := url.Parse(w.Header().Get("Location"))
		return loc.Query().Get("state"), cookie(w, "oa_oauth")
	}
	errorOf := func(w *httptest.ResponseRecorder) (string, string) {
		loc, _ := url.Parse(w.Header().Get("Location"))
		return loc.Query().Get("error"), loc.Query().Get("returnTo")
	}

	t.Run("state mismatch", func(t *testing.T) {
		_, c := start("/")
		code, _ := errorOf(r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=forged", cookies: []*http.Cookie{c}}))
		assert.Equal(t, "oauth_state_mismatch", code)
	})
	t.Run("missing cookie (login CSRF)", func(t *testing.T) {
		state, _ := start("/")
		code, _ := errorOf(r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=" + state}))
		assert.Equal(t, "oauth_state_mismatch", code)
	})
	t.Run("tampered cookie", func(t *testing.T) {
		state, c := start("/")
		c.Value = "e30." + strings.SplitN(c.Value, ".", 2)[1]
		code, _ := errorOf(r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=" + state, cookies: []*http.Cookie{c}}))
		assert.Equal(t, "oauth_state_mismatch", code)
	})
	t.Run("user denied consent", func(t *testing.T) {
		state, c := start("/samples")
		code, ret := errorOf(r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?error=access_denied&state=" + state, cookies: []*http.Cookie{c}}))
		assert.Equal(t, "oauth_denied", code)
		assert.Equal(t, "/samples", ret)
	})
	t.Run("open redirect is neutralised", func(t *testing.T) {
		state, c := start("//evil.example/phish")
		w := r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=" + state, cookies: []*http.Cookie{c}})
		assert.Equal(t, webURL+"/auth/callback?returnTo=%2F", w.Header().Get("Location"))
	})
	t.Run("exchange failure", func(t *testing.T) {
		r.google.err = errors.New("boom")
		defer func() { r.google.err = nil }()
		state, c := start("/")
		code, _ := errorOf(r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=" + state, cookies: []*http.Cookie{c}}))
		assert.Equal(t, "oauth_exchange_failed", code)
	})
	t.Run("unverified email cannot take over an account", func(t *testing.T) {
		r.register(t, "victim@example.com")
		r.google.profile = domain.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "evil", Email: "victim@example.com", EmailVerified: false}
		state, c := start("/")
		w := r.do(t, req{method: "GET", path: "/api/v1/auth/oauth/google/callback?code=x&state=" + state, cookies: []*http.Cookie{c}})
		code, _ := errorOf(w)
		assert.Equal(t, "email_unverified_at_provider", code)
		assert.Nil(t, cookie(w, "oa_refresh"))
	})
}

func TestAdminRoleManagement(t *testing.T) {
	r := newRig(t, nil)
	adminToken, _ := r.register(t, "admin@openaudio.dev")
	userToken, _ := r.register(t, "user@example.com")
	w := r.do(t, req{method: "GET", path: "/api/v1/me", bearer: userToken})
	userID := decode(t, w)["id"].(string)

	path := "/api/v1/admin/users/" + userID + "/role"
	w = r.do(t, req{method: "PATCH", path: path, bearer: userToken, body: map[string]string{"role": "admin"}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = r.do(t, req{method: "PATCH", path: path, body: map[string]string{"role": "admin"}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = r.do(t, req{method: "PATCH", path: path, bearer: adminToken, body: map[string]string{"role": "publisher"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "publisher", decode(t, w)["role"])

	// The role is embedded in the next access token.
	w = r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "user@example.com", "password": "a-good-password"}})
	assert.Equal(t, "publisher", decode(t, w)["user"].(map[string]any)["role"])

	w = r.do(t, req{method: "PATCH", path: "/api/v1/admin/users/not-a-uuid/role", bearer: adminToken, body: map[string]string{"role": "user"}})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCORS(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "OPTIONS", path: "/api/v1/auth/login", headers: map[string]string{"Origin": webURL, "Access-Control-Request-Method": "POST"}})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, webURL, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))

	w = r.do(t, req{method: "GET", path: "/api/v1/health", headers: map[string]string{"Origin": "https://evil.example"}})
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestHealth(t *testing.T) {
	r := newRig(t, nil)
	assert.Equal(t, http.StatusOK, r.do(t, req{method: "GET", path: "/api/v1/health"}).Code)
	w := r.do(t, req{method: "GET", path: "/api/v1/health/ready"})
	assert.Equal(t, map[string]any{"status": "ok", "checks": map[string]any{"database": "ok"}}, decode(t, w))

	*r.checkOK = false
	w = r.do(t, req{method: "GET", path: "/api/v1/health/ready"})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "degraded", decode(t, w)["status"])
}
