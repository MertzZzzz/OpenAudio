package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// fakeProvider emulates a token endpoint and a user-info endpoint.
func fakeProvider(t *testing.T, wantVerifier string, userinfo any) (*httptest.Server, Endpoints) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "the-code", r.Form.Get("code"))
		assert.Equal(t, wantVerifier, r.Form.Get("code_verifier"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-123", "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(userinfo)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, Endpoints{AuthURL: srv.URL + "/authorize", TokenURL: srv.URL + "/token", UserInfoURL: srv.URL + "/userinfo"}
}

var creds = Credentials{ClientID: "cid", ClientSecret: "secret", RedirectURL: "http://localhost/cb"}

func TestGoogle(t *testing.T) {
	_, ep := fakeProvider(t, "verifier-xyz", map[string]any{"sub": "1077", "email": "a@gmail.com", "email_verified": true, "name": "Ann", "picture": "https://p/x.png"})
	g := NewGoogle(creds, ep)

	u, err := url.Parse(g.AuthCodeURL("st4te", "verifier-xyz"))
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "st4te", q.Get("state"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	sum := sha256.Sum256([]byte("verifier-xyz"))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), q.Get("code_challenge"))

	p, err := g.Exchange(context.Background(), "the-code", "verifier-xyz")
	require.NoError(t, err)
	assert.Equal(t, domain.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "1077", Email: "a@gmail.com", EmailVerified: true, Name: "Ann", AvatarURL: "https://p/x.png"}, *p)
}

func TestDiscord(t *testing.T) {
	_, ep := fakeProvider(t, "", map[string]any{"id": "80351110224678912", "username": "nelly", "global_name": "Nelly", "email": "nelly@example.com", "verified": true, "avatar": "8342729096ea3675442027381ff50dfe"})
	d := NewDiscord(creds, ep)

	u, _ := url.Parse(d.AuthCodeURL("s", "ignored"))
	assert.Equal(t, "identify email", u.Query().Get("scope"))
	assert.Empty(t, u.Query().Get("code_challenge"), "no PKCE for Discord")

	p, err := d.Exchange(context.Background(), "the-code", "")
	require.NoError(t, err)
	assert.Equal(t, "80351110224678912", p.Subject)
	assert.Equal(t, "Nelly", p.Name)
	assert.True(t, p.EmailVerified)
	assert.Equal(t, "https://cdn.discordapp.com/avatars/80351110224678912/8342729096ea3675442027381ff50dfe.png", p.AvatarURL)
}

func TestDiscordWithoutEmailScope(t *testing.T) {
	_, ep := fakeProvider(t, "", map[string]any{"id": "1", "username": "x", "verified": false})
	p, err := NewDiscord(creds, ep).Exchange(context.Background(), "the-code", "")
	require.NoError(t, err)
	assert.Empty(t, p.Email)
	assert.Empty(t, p.AvatarURL)
}

func TestUserInfoFailure(t *testing.T) {
	srv, ep := fakeProvider(t, "v", map[string]any{})
	ep.UserInfoURL = srv.URL + "/missing"
	_, err := NewGoogle(creds, ep).Exchange(context.Background(), "the-code", "v")
	assert.ErrorContains(t, err, "userinfo: status 404")
}
