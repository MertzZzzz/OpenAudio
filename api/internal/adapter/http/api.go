package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/oauth"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/library"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/search"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/uploads"
)

// TokenParser verifies access tokens.
type TokenParser interface {
	Parse(token string) (*domain.AccessClaims, error)
}

// HealthCheck is a named readiness dependency (database, object storage).
type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

type Options struct {
	Auth       *auth.Service
	Publishing *publishing.Service
	Uploads    *uploads.Service
	Search     *search.Service
	Library    *library.Service
	Tokens     TokenParser
	Providers  []oauth.Provider
	Checks     []HealthCheck
	Logger     *slog.Logger

	WebURL       string // where the SPA lives; OAuth finishes at {WebURL}/auth/callback
	CORSOrigins  []string
	CookieSecure bool
	TrustProxy   bool
	// StateKey signs the short-lived OAuth state cookie.
	StateKey []byte
	// Limiter guards credential endpoints; nil disables rate limiting.
	Limiter *RateLimiter
	Now     func() time.Time
}

type API struct {
	auth      *auth.Service
	pub       *publishing.Service
	up        *uploads.Service
	srch      *search.Service
	lib       *library.Service
	tokens    TokenParser
	providers map[domain.AuthProvider]oauth.Provider
	order     []domain.AuthProvider
	checks    []HealthCheck
	log       *slog.Logger
	limiter   *RateLimiter
	opts      Options
	now       func() time.Time
}

const APIPrefix = "/api/v1"

// NewHandler builds the HTTP handler with every route under /api/v1.
func NewHandler(o Options) http.Handler {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	a := &API{
		auth: o.Auth, pub: o.Publishing, up: o.Uploads, srch: o.Search, lib: o.Library, tokens: o.Tokens, checks: o.Checks, log: o.Logger, limiter: o.Limiter, opts: o, now: o.Now,
		providers: map[domain.AuthProvider]oauth.Provider{},
	}
	for _, p := range o.Providers {
		a.providers[p.Name()] = p
		a.order = append(a.order, p.Name())
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	if o.TrustProxy {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.Recoverer, requestLogger(o.Logger), securityHeaders, cors(o.CORSOrigins))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		a.writeError(w, r, domain.NotFound("route_not_found", "no such endpoint"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed", "message": "method not allowed"})
	})

	r.Route(APIPrefix, func(r chi.Router) {
		r.Use(a.authenticate)
		r.Get("/health", a.health)
		r.Get("/health/ready", a.ready)

		r.Route("/auth", func(r chi.Router) {
			r.With(a.rateLimit).Post("/register", a.register)
			r.With(a.rateLimit).Post("/login", a.login)
			r.Post("/refresh", a.refresh)
			r.Post("/logout", a.logout)
			r.With(a.rateLimit).Post("/email/verify", a.verifyEmail)
			r.With(a.requireAuth, a.rateLimit).Post("/email/resend", a.resendVerification)
			r.With(a.rateLimit).Post("/password/forgot", a.forgotPassword)
			r.With(a.rateLimit).Post("/password/reset", a.resetPassword)
			r.Get("/providers", a.listProviders)
			r.Get("/oauth/{provider}/start", a.oauthStart)
			r.Get("/oauth/{provider}/callback", a.oauthCallback)
		})

		r.With(a.requireAuth).Get("/me", a.me)

		// Manifests
		r.Get("/schemas/openaudio.package/v1", a.manifestSchema)
		r.Post("/manifests/validate", a.validateManifest)

		// Publishers
		r.With(a.requireAuth).Post("/publishers", a.createPublisher)
		r.With(a.requireAuth).Get("/me/publishers", a.myPublishers)
		r.Route("/publishers/{publisher}", func(r chi.Router) {
			r.Get("/", a.getPublisher)
			r.With(a.requireAuth).Get("/packages", a.publisherPackages)
			r.With(a.requireAuth).Get("/members", a.listMembers)
			r.With(a.requireAuth).Post("/members", a.addMember)
			r.With(a.requireAuth).Delete("/members/{userId}", a.removeMember)
		})

		// Packages & releases
		r.Get("/packages", a.listPackages)
		r.With(a.requireAuth).Post("/packages", a.createPackage)
		r.Route("/packages/{publisher}/{slug}", func(r chi.Router) {
			r.Get("/", a.getPackage)
			r.With(a.requireAuth).Patch("/", a.updatePackage)
			r.Get("/versions", a.listVersions)
			r.Get("/versions/{version}/manifest", a.getManifest)
			r.With(a.requireAuth).Post("/releases", a.createRelease)
			r.Get("/releases/{version}", a.getRelease)
			r.With(a.requireAuth).Put("/releases/{version}", a.updateRelease)
			r.With(a.requireAuth).Delete("/releases/{version}", a.deleteRelease)
			r.With(a.requireAuth).Post("/releases/{version}/publish", a.publishRelease)
			r.With(a.requireAuth).Post("/releases/{version}/yank", a.yankRelease)
			r.Get("/releases/{version}/files/*", a.downloadFile)
		})

		// Search
		r.Get("/search", a.search)
		r.Get("/search/facets", a.searchFacets)
		r.Get("/search/samples", a.searchSamples)
		r.Get("/taxonomy", a.taxonomy)

		// Libraries & lockfiles (personal)
		r.Route("/libraries", func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Get("/", a.listLibraries)
			r.Post("/", a.createLibrary)
			r.Patch("/{libraryId}", a.renameLibrary)
			r.Delete("/{libraryId}", a.deleteLibrary)
			r.Get("/{libraryId}/items", a.listItems)
			r.Post("/{libraryId}/items", a.addItem)
			r.Patch("/{libraryId}/items/{packageId}", a.updateItem)
			r.Delete("/{libraryId}/items/{packageId}", a.removeItem)
			r.Post("/{libraryId}/resolve", a.resolveLibrary)
			r.Post("/{libraryId}/lockfiles", a.saveLockfile)
			r.Post("/{libraryId}/import", a.importLockfile)
		})
		r.Route("/lockfiles", func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Get("/", a.listLockfiles)
			r.Get("/{lockfileId}", a.getLockfile)
			r.Delete("/{lockfileId}", a.deleteLockfile)
		})

		// Uploads
		r.Route("/uploads", func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Post("/", a.createUpload)
			r.Get("/{uploadId}", a.getUpload)
			r.Delete("/{uploadId}", a.abortUpload)
			r.Put("/{uploadId}/parts/{partNumber}", a.putPart)
			r.Post("/{uploadId}/complete", a.completeUpload)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(a.requireRole(domain.RoleAdmin))
			r.Patch("/users/{id}/role", a.setRole)
		})
	})
	return r
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready reports each dependency; 503 if any is down (used by orchestrators / compose).
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	checks := map[string]string{}
	status, code := "ok", http.StatusOK
	for _, c := range a.checks {
		if err := c.Check(ctx); err != nil {
			checks[c.Name] = "down: " + err.Error()
			status, code = "degraded", http.StatusServiceUnavailable
		} else {
			checks[c.Name] = "ok"
		}
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": checks})
}
