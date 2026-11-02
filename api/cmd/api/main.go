// Command api runs the OpenAudio HTTP API.
// It only wires configuration, connections and dependencies; logic lives in internal/.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/MertzZzzz/OpenAudio/api/internal/adapter/http"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/mailer"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/oauth"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/postgres"
	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/s3"
	"github.com/MertzZzzz/OpenAudio/api/internal/config"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/logger"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/security"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/library"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/search"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/uploads"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logger.New(os.Stdout, cfg.LogLevel, cfg.Env)
	slog.SetDefault(log)
	for _, w := range cfg.Warnings {
		log.Warn(w)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// PostgreSQL
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.AutoMigrate {
		if err := postgres.Migrate(pool); err != nil {
			return err
		}
		log.Info("database migrations applied")
	}

	// Object storage: not required for auth, so a failure only degrades readiness.
	storage, err := s3.New(s3.Config{Endpoint: cfg.S3Endpoint, PublicEndpoint: cfg.S3PublicEndpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Bucket: cfg.S3Bucket, Region: cfg.S3Region})
	if err != nil {
		return err
	}
	if err := storage.EnsureBucket(ctx); err != nil {
		log.Warn("object storage unavailable", "err", err)
	}

	// Auth
	jwtIssuer, err := security.NewJWTIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTTL, nil)
	if err != nil {
		return err
	}
	var mail domain.Mailer = mailer.LogMailer{Logger: log}
	if cfg.SMTP.Host != "" {
		mail = mailer.SMTPMailer{Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password, From: cfg.SMTP.From}
	}
	authSvc, err := auth.NewService(auth.Deps{
		Users:      postgres.NewUserRepo(pool),
		Identities: postgres.NewIdentityRepo(pool),
		Refresh:    postgres.NewRefreshTokenRepo(pool),
		OneTime:    postgres.NewOneTimeTokenRepo(pool),
		Hasher:     security.NewArgon2Hasher(security.DefaultArgon2Params),
		Tokens:     jwtIssuer,
		Mailer:     mail,
		Logger:     log,
	}, auth.Config{RefreshTTL: cfg.RefreshTTL, WebURL: cfg.WebURL, AdminEmails: cfg.AdminEmails})
	if err != nil {
		return err
	}

	// Publishing & uploads
	validator, err := manifest.NewValidator()
	if err != nil {
		return err
	}
	publisherRepo := postgres.NewPublisherRepo(pool)
	packageRepo := postgres.NewPackageRepo(pool)
	pubSvc := publishing.NewService(publishing.Deps{
		Users: postgres.NewUserRepo(pool), Publishers: publisherRepo, Packages: packageRepo,
		Releases: postgres.NewReleaseRepo(pool), Blobs: storage, Validator: validator, Logger: log, DownloadTTL: cfg.DownloadTTL,
	})
	upSvc := uploads.NewService(uploads.Deps{Uploads: postgres.NewUploadRepo(pool), Publishers: publisherRepo, Blobs: storage, Logger: log},
		uploads.Config{PartSize: cfg.UploadPartSize, MaxSize: cfg.MaxBlobSize})

	libSvc := library.NewService(library.Deps{Libraries: postgres.NewLibraryRepo(pool), Lockfiles: postgres.NewLockfileRepo(pool),
		Packages: packageRepo, Releases: postgres.NewReleaseRepo(pool), Logger: log})
	searchSvc := search.NewService(search.Deps{Repo: postgres.NewSearchRepo(pool), Packages: packageRepo, Logger: log})

	var providers []oauth.Provider
	callback := func(p string) string { return cfg.PublicURL + httpapi.APIPrefix + "/auth/oauth/" + p + "/callback" }
	if cfg.Google.Enabled() {
		providers = append(providers, oauth.NewGoogle(oauth.Credentials{ClientID: cfg.Google.ClientID, ClientSecret: cfg.Google.ClientSecret, RedirectURL: callback("google")}, oauth.GoogleEndpoints))
	}
	if cfg.Discord.Enabled() {
		providers = append(providers, oauth.NewDiscord(oauth.Credentials{ClientID: cfg.Discord.ClientID, ClientSecret: cfg.Discord.ClientSecret, RedirectURL: callback("discord")}, oauth.DiscordEndpoints))
	}
	for _, p := range providers {
		log.Info("oauth provider enabled", "provider", p.Name(), "redirect_uri", callback(string(p.Name())))
	}

	// Derive a separate key for OAuth state cookies from the JWT secret.
	stateMAC := hmac.New(sha256.New, cfg.JWTSecret)
	stateMAC.Write([]byte("openaudio/oauth-state"))

	handler := httpapi.NewHandler(httpapi.Options{
		Auth: authSvc, Publishing: pubSvc, Uploads: upSvc, Search: searchSvc, Library: libSvc, Tokens: jwtIssuer, Providers: providers, Logger: log,
		WebURL: cfg.WebURL, CORSOrigins: cfg.CORSOrigins, CookieSecure: cfg.CookieSecure, TrustProxy: cfg.TrustProxy,
		StateKey: stateMAC.Sum(nil), Limiter: httpapi.NewRateLimiter(10, 5),
		Checks: []httpapi.HealthCheck{
			{Name: "database", Check: pool.Ping},
			{Name: "storage", Check: storage.Ping},
		},
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// Generous body timeouts: upload parts are up to UPLOAD_PART_SIZE and
		// completing an upload re-hashes the whole blob.
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
