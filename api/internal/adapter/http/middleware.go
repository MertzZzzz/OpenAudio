package http

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type ctxKey int

const claimsKey ctxKey = iota

// ClaimsFrom returns the authenticated caller, if any.
func ClaimsFrom(ctx context.Context) (*domain.AccessClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*domain.AccessClaims)
	return c, ok
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			log.LogAttrs(r.Context(), slog.LevelInfo, "http",
				slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)), slog.String("request_id", middleware.GetReqID(r.Context())))
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// cors allows credentialed requests from the configured web origins only.
func cors(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok && origin != "" {
				h := w.Header()
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					h.Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authenticate attaches claims for a valid "Authorization: Bearer <jwt>" header.
// Invalid tokens are rejected; missing tokens pass through (requireAuth decides).
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			a.writeError(w, r, domain.Unauthorized("invalid_token", "Authorization header must be 'Bearer <token>'"))
			return
		}
		claims, err := a.tokens.Parse(token)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			a.writeError(w, r, domain.Unauthorized("invalid_token", "access token is invalid or expired"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFrom(r.Context()); !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			a.writeError(w, r, domain.Unauthorized("unauthenticated", "sign in required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) requireRole(min domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := ClaimsFrom(r.Context())
			if !ok {
				a.writeError(w, r, domain.Unauthorized("unauthenticated", "sign in required"))
				return
			}
			if !c.Role.AtLeast(min) {
				a.writeError(w, r, domain.Forbidden("forbidden", string(min)+" role required"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter is a per-client-IP token bucket used on credential endpoints.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*limiterEntry
	limit   rate.Limit
	burst   int
	lastGC  time.Time
}

type limiterEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{clients: map[string]*limiterEntry{}, limit: rate.Limit(float64(perMinute) / 60), burst: burst}
}

func (l *RateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastGC) > time.Minute {
		for k, e := range l.clients {
			if now.Sub(e.seen) > 10*time.Minute {
				delete(l.clients, k)
			}
		}
		l.lastGC = now
	}
	e, ok := l.clients[key]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(l.limit, l.burst)}
		l.clients[key] = e
	}
	e.seen = now
	return e.lim.Allow()
}

func (a *API) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.limiter != nil && !a.limiter.allow(clientIP(r)+" "+r.URL.Path) {
			w.Header().Set("Retry-After", "60")
			a.writeError(w, r, domain.RateLimited())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP uses RemoteAddr, which chi's RealIP middleware rewrites when TRUST_PROXY is on.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
