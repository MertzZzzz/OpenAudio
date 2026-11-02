package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// The OAuth "state" round-trip is stateless: the state, PKCE verifier and return path are
// kept in a short-lived, HMAC-signed, HttpOnly cookie scoped to the OAuth routes.
const (
	oauthCookie   = "oa_oauth"
	oauthStateTTL = 10 * time.Minute
)

type oauthState struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Verifier string `json:"v"`
	ReturnTo string `json:"r"`
	Expires  int64  `json:"e"`
}

func (a *API) sign(payload []byte) string {
	mac := hmac.New(sha256.New, a.opts.StateKey)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *API) verify(value string) ([]byte, bool) {
	payloadB64, sigB64, ok := strings.Cut(value, ".")
	if !ok {
		return nil, false
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(payloadB64)
	sig, err2 := base64.RawURLEncoding.DecodeString(sigB64)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	mac := hmac.New(sha256.New, a.opts.StateKey)
	mac.Write(payload)
	return payload, hmac.Equal(sig, mac.Sum(nil))
}

// safeReturnTo only allows same-site relative paths (prevents open redirects).
func safeReturnTo(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.ContainsAny(p, "\r\n") {
		return "/"
	}
	return p
}

func (a *API) finishURL(returnTo, errCode string) string {
	q := url.Values{}
	q.Set("returnTo", safeReturnTo(returnTo))
	if errCode != "" {
		q.Set("error", errCode)
	}
	return a.opts.WebURL + "/auth/callback?" + q.Encode()
}

func (a *API) listProviders(w http.ResponseWriter, _ *http.Request) {
	type providerDTO struct {
		ID       string `json:"id"`
		StartURL string `json:"startUrl"`
	}
	out := make([]providerDTO, 0, len(a.order))
	for _, p := range a.order {
		out = append(out, providerDTO{ID: string(p), StartURL: APIPrefix + "/auth/oauth/" + string(p) + "/start"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"email": true, "oauth": out})
}

func (a *API) oauthStart(w http.ResponseWriter, r *http.Request) {
	name := domain.AuthProvider(chi.URLParam(r, "provider"))
	p, ok := a.providers[name]
	if !ok {
		a.writeError(w, r, domain.NotFound("provider_not_enabled", "sign-in with this provider is not enabled"))
		return
	}
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		a.writeError(w, r, domain.Internal(err))
		return
	}
	st := oauthState{
		Provider: string(name),
		State:    base64.RawURLEncoding.EncodeToString(stateBytes),
		Verifier: oauth2.GenerateVerifier(),
		ReturnTo: safeReturnTo(r.URL.Query().Get("returnTo")),
		Expires:  a.now().Add(oauthStateTTL).Unix(),
	}
	payload, _ := json.Marshal(st)
	http.SetCookie(w, &http.Cookie{
		Name: oauthCookie, Value: a.sign(payload), Path: APIPrefix + "/auth/oauth",
		MaxAge: int(oauthStateTTL.Seconds()), HttpOnly: true, Secure: a.opts.CookieSecure,
		// Lax is required: the provider redirects back with a top-level GET.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, p.AuthCodeURL(st.State, st.Verifier), http.StatusFound)
}

func (a *API) oauthCallback(w http.ResponseWriter, r *http.Request) {
	name := domain.AuthProvider(chi.URLParam(r, "provider"))
	// Always consume the state cookie.
	http.SetCookie(w, &http.Cookie{Name: oauthCookie, Value: "", Path: APIPrefix + "/auth/oauth", MaxAge: -1,
		HttpOnly: true, Secure: a.opts.CookieSecure, SameSite: http.SameSiteLaxMode})

	fail := func(returnTo, code string) { http.Redirect(w, r, a.finishURL(returnTo, code), http.StatusFound) }

	p, ok := a.providers[name]
	if !ok {
		fail("/", "provider_not_enabled")
		return
	}
	st, err := a.readState(r, name)
	if err != nil {
		a.log.WarnContext(r.Context(), "oauth state rejected", "provider", name, "err", err)
		fail("/", "oauth_state_mismatch")
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		code := "oauth_failed"
		if e == "access_denied" {
			code = "oauth_denied"
		}
		fail(st.ReturnTo, code)
		return
	}
	code := q.Get("code")
	if code == "" {
		fail(st.ReturnTo, "oauth_failed")
		return
	}
	profile, err := p.Exchange(r.Context(), code, st.Verifier)
	if err != nil {
		a.log.WarnContext(r.Context(), "oauth exchange failed", "provider", name, "err", err)
		fail(st.ReturnTo, "oauth_exchange_failed")
		return
	}
	s, err := a.auth.OAuthLogin(r.Context(), *profile, meta(r))
	if err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Kind != domain.KindInternal {
			fail(st.ReturnTo, de.Code)
			return
		}
		a.log.ErrorContext(r.Context(), "oauth login failed", "provider", name, "err", err)
		fail(st.ReturnTo, "internal")
		return
	}
	// The SPA picks the session up by calling POST /auth/refresh with this cookie.
	a.setRefreshCookie(w, s)
	http.Redirect(w, r, a.finishURL(st.ReturnTo, ""), http.StatusFound)
}

func (a *API) readState(r *http.Request, provider domain.AuthProvider) (*oauthState, error) {
	c, err := r.Cookie(oauthCookie)
	if err != nil {
		return nil, errors.New("missing state cookie")
	}
	payload, ok := a.verify(c.Value)
	if !ok {
		return nil, errors.New("bad signature")
	}
	var st oauthState
	if err := json.Unmarshal(payload, &st); err != nil {
		return nil, err
	}
	if st.Provider != string(provider) {
		return nil, errors.New("provider mismatch")
	}
	if a.now().Unix() > st.Expires {
		return nil, errors.New("state expired")
	}
	if subtle.ConstantTimeCompare([]byte(st.State), []byte(r.URL.Query().Get("state"))) != 1 {
		return nil, errors.New("state mismatch")
	}
	return &st, nil
}
