// Package oauth implements sign-in with Google and Discord (OAuth 2.0 authorization code flow).
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// Provider is one external identity provider.
type Provider interface {
	Name() domain.AuthProvider
	// AuthCodeURL returns the provider consent URL. verifier is the PKCE code verifier
	// (ignored by providers that do not use PKCE).
	AuthCodeURL(state, verifier string) string
	// Exchange trades the authorization code for a verified profile.
	Exchange(ctx context.Context, code, verifier string) (*domain.ExternalProfile, error)
}

// Endpoints can be overridden in tests.
type Endpoints struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
}

type Credentials struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type base struct {
	cfg      oauth2.Config
	userInfo string
	pkce     bool
	client   *http.Client
}

func newBase(c Credentials, e Endpoints, scopes []string, pkce bool) base {
	return base{
		cfg: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Scopes:       scopes,
			Endpoint:     oauth2.Endpoint{AuthURL: e.AuthURL, TokenURL: e.TokenURL},
		},
		userInfo: e.UserInfoURL,
		pkce:     pkce,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (b base) authCodeURL(state, verifier string, extra ...oauth2.AuthCodeOption) string {
	if b.pkce {
		extra = append(extra, oauth2.S256ChallengeOption(verifier))
	}
	return b.cfg.AuthCodeURL(state, extra...)
}

// fetchJSON exchanges the code and GETs the user-info endpoint with the access token.
func (b base) fetchJSON(ctx context.Context, code, verifier string, out any) error {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, b.client)
	var opts []oauth2.AuthCodeOption
	if b.pkce {
		opts = append(opts, oauth2.VerifierOption(verifier))
	}
	tok, err := b.cfg.Exchange(ctx, code, opts...)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.userInfo, nil)
	if err != nil {
		return err
	}
	tok.SetAuthHeader(req)
	res, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("userinfo request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("userinfo: status %d: %s", res.StatusCode, body)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
}

/* ----------------------------------------------------------------- Google */

var GoogleEndpoints = Endpoints{
	AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL:    "https://oauth2.googleapis.com/token",
	UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
}

type Google struct{ base }

func NewGoogle(c Credentials, e Endpoints) *Google {
	return &Google{newBase(c, e, []string{"openid", "email", "profile"}, true)}
}

func (g *Google) Name() domain.AuthProvider { return domain.ProviderGoogle }

func (g *Google) AuthCodeURL(state, verifier string) string {
	return g.authCodeURL(state, verifier, oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (g *Google) Exchange(ctx context.Context, code, verifier string) (*domain.ExternalProfile, error) {
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := g.fetchJSON(ctx, code, verifier, &info); err != nil {
		return nil, err
	}
	return &domain.ExternalProfile{
		Provider: domain.ProviderGoogle, Subject: info.Sub, Email: info.Email,
		EmailVerified: info.EmailVerified, Name: info.Name, AvatarURL: info.Picture,
	}, nil
}

/* ---------------------------------------------------------------- Discord */

var DiscordEndpoints = Endpoints{
	AuthURL:     "https://discord.com/oauth2/authorize",
	TokenURL:    "https://discord.com/api/oauth2/token",
	UserInfoURL: "https://discord.com/api/users/@me",
}

type Discord struct{ base }

// NewDiscord requests the identify + email scopes. PKCE is not used; CSRF protection
// relies on the signed state cookie, as for every provider.
func NewDiscord(c Credentials, e Endpoints) *Discord {
	return &Discord{newBase(c, e, []string{"identify", "email"}, false)}
}

func (d *Discord) Name() domain.AuthProvider { return domain.ProviderDiscord }

func (d *Discord) AuthCodeURL(state, verifier string) string {
	return d.authCodeURL(state, verifier, oauth2.SetAuthURLParam("prompt", "none"))
}

func (d *Discord) Exchange(ctx context.Context, code, verifier string) (*domain.ExternalProfile, error) {
	var info struct {
		ID         string  `json:"id"`
		Username   string  `json:"username"`
		GlobalName *string `json:"global_name"`
		Email      *string `json:"email"`
		Verified   bool    `json:"verified"`
		Avatar     *string `json:"avatar"`
	}
	if err := d.fetchJSON(ctx, code, verifier, &info); err != nil {
		return nil, err
	}
	p := &domain.ExternalProfile{Provider: domain.ProviderDiscord, Subject: info.ID, EmailVerified: info.Verified, Name: info.Username}
	if info.GlobalName != nil && *info.GlobalName != "" {
		p.Name = *info.GlobalName
	}
	if info.Email != nil {
		p.Email = *info.Email
	}
	if info.Avatar != nil && *info.Avatar != "" {
		p.AvatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", info.ID, *info.Avatar)
	}
	return p, nil
}
