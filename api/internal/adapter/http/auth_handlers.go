package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
)

// The refresh token is delivered both in the JSON body (CLI / native clients) and as an
// HttpOnly cookie scoped to /api/v1/auth (browsers: the SPA never stores it in JS).
const refreshCookie = "oa_refresh"

func (a *API) setRefreshCookie(w http.ResponseWriter, s *auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: s.RefreshToken, Path: APIPrefix + "/auth",
		Expires: s.RefreshExpiresAt, MaxAge: int(time.Until(s.RefreshExpiresAt).Seconds()),
		HttpOnly: true, Secure: a.opts.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookie, Value: "", Path: APIPrefix + "/auth", MaxAge: -1,
		HttpOnly: true, Secure: a.opts.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func meta(r *http.Request) auth.ClientMeta {
	return auth.ClientMeta{UserAgent: r.UserAgent(), IP: clientIP(r)}
}

func (a *API) respondSession(w http.ResponseWriter, status int, s *auth.Session) {
	a.setRefreshCookie(w, s)
	writeJSON(w, status, toTokenPair(s, a.now()))
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	s, err := a.auth.Register(r.Context(), auth.RegisterInput{Email: in.Email, Password: in.Password, DisplayName: in.DisplayName}, meta(r))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.respondSession(w, http.StatusCreated, s)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	s, err := a.auth.Login(r.Context(), in.Email, in.Password, meta(r))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.respondSession(w, http.StatusOK, s)
}

// refreshTokenFrom prefers an explicit body token, then the HttpOnly cookie.
func refreshTokenFrom(r *http.Request) (string, error) {
	var in struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := decodeJSON(r, &in, true); err != nil {
		return "", err
	}
	if in.RefreshToken != "" {
		return in.RefreshToken, nil
	}
	if c, err := r.Cookie(refreshCookie); err == nil {
		return c.Value, nil
	}
	return "", nil
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	tok, err := refreshTokenFrom(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	s, err := a.auth.RefreshSession(r.Context(), tok, meta(r))
	if err != nil {
		if domain.KindOf(err) == domain.KindUnauthorized {
			a.clearRefreshCookie(w)
		}
		a.writeError(w, r, err)
		return
	}
	a.respondSession(w, http.StatusOK, s)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	tok, err := refreshTokenFrom(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if err := a.auth.Logout(r.Context(), tok); err != nil {
		a.writeError(w, r, err)
		return
	}
	a.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	c, _ := ClaimsFrom(r.Context())
	p, err := a.auth.Me(r.Context(), c.UserID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(p.User, p.Identities))
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	if err := a.auth.VerifyEmail(r.Context(), in.Token); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) resendVerification(w http.ResponseWriter, r *http.Request) {
	c, _ := ClaimsFrom(r.Context())
	if err := a.auth.ResendVerification(r.Context(), c.UserID); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	if err := a.auth.RequestPasswordReset(r.Context(), in.Email); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	if err := a.auth.ResetPassword(r.Context(), in.Token, in.Password); err != nil {
		a.writeError(w, r, err)
		return
	}
	a.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) setRole(w http.ResponseWriter, r *http.Request) {
	c, _ := ClaimsFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.writeError(w, r, domain.NotFound("user_not_found", "user not found"))
		return
	}
	var in struct {
		Role domain.Role `json:"role"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	u, err := a.auth.SetRole(r.Context(), *c, id, in.Role)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u, nil))
}
