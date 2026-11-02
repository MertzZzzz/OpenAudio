package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/postgres"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// Integration tests run against a real database:
//
//	TEST_DATABASE_URL=postgres://openaudio:openaudio@localhost:5432/openaudio_test?sslmode=disable go test ./...
//
// The database is wiped (migrate down + up) at the start of the run.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration tests")
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, postgres.MigrateDown(pool))
	require.NoError(t, postgres.Migrate(pool))
	return pool
}

var ctx = context.Background()

func newUser(email string) *domain.User {
	now := time.Now().UTC().Truncate(time.Microsecond)
	h := "$argon2id$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA"
	return &domain.User{ID: uuid.New(), Email: email, PasswordHash: &h, DisplayName: "Tester", Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now}
}

func TestPostgres(t *testing.T) {
	pool := setup(t)
	users := postgres.NewUserRepo(pool)
	idents := postgres.NewIdentityRepo(pool)
	refresh := postgres.NewRefreshTokenRepo(pool)
	oneTime := postgres.NewOneTimeTokenRepo(pool)

	t.Run("migrations are reversible", func(t *testing.T) {
		require.NoError(t, postgres.MigrateDown(pool))
		require.NoError(t, postgres.Migrate(pool))
		require.NoError(t, postgres.Migrate(pool), "idempotent")
	})

	t.Run("users: create, case-insensitive email, updates", func(t *testing.T) {
		u := newUser("Case@Example.com")
		require.NoError(t, users.Create(ctx, u))
		assert.ErrorIs(t, users.Create(ctx, newUser("case@example.COM")), domain.ErrConflict)

		got, err := users.GetByEmail(ctx, "CASE@example.com")
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
		assert.Equal(t, *u.PasswordHash, *got.PasswordHash)

		require.NoError(t, users.UpdateRole(ctx, u.ID, domain.RolePublisher))
		at := time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, users.MarkEmailVerified(ctx, u.ID, at))
		require.NoError(t, users.MarkEmailVerified(ctx, u.ID, at.Add(time.Hour)), "first verification time is kept")
		got, _ = users.GetByID(ctx, u.ID)
		assert.Equal(t, domain.RolePublisher, got.Role)
		assert.True(t, got.EmailVerifiedAt.Equal(at))

		_, err = users.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, users.UpdateRole(ctx, uuid.New(), domain.RoleUser), domain.ErrNotFound)

		_, err = pool.Exec(ctx, `UPDATE users SET role = 'root' WHERE id = $1`, u.ID)
		assert.Error(t, err, "role check constraint")
	})

	t.Run("CreateWithIdentity is atomic", func(t *testing.T) {
		u := newUser("oauth@example.com")
		u.PasswordHash = nil
		id := &domain.Identity{ID: uuid.New(), UserID: u.ID, Provider: domain.ProviderGoogle, Subject: "g-1", Email: u.Email, CreatedAt: u.CreatedAt}
		require.NoError(t, users.CreateWithIdentity(ctx, u, id))

		got, err := idents.Get(ctx, domain.ProviderGoogle, "g-1")
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.UserID)

		// Same provider subject → the whole transaction rolls back, no orphan user.
		u2 := newUser("other@example.com")
		dup := &domain.Identity{ID: uuid.New(), UserID: u2.ID, Provider: domain.ProviderGoogle, Subject: "g-1", CreatedAt: u2.CreatedAt}
		assert.ErrorIs(t, users.CreateWithIdentity(ctx, u2, dup), domain.ErrConflict)
		_, err = users.GetByEmail(ctx, "other@example.com")
		assert.ErrorIs(t, err, domain.ErrNotFound)

		list, err := idents.ListByUser(ctx, u.ID)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("refresh tokens rotate once", func(t *testing.T) {
		u := newUser("rt@example.com")
		require.NoError(t, users.Create(ctx, u))
		now := time.Now().UTC()
		fam := uuid.New()
		first := &domain.RefreshToken{ID: uuid.New(), UserID: u.ID, FamilyID: fam, TokenHash: []byte("h1"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
		require.NoError(t, refresh.Create(ctx, first))

		second := &domain.RefreshToken{ID: uuid.New(), UserID: u.ID, FamilyID: fam, TokenHash: []byte("h2"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
		require.NoError(t, refresh.Rotate(ctx, first.ID, second, now))

		third := &domain.RefreshToken{ID: uuid.New(), UserID: u.ID, FamilyID: fam, TokenHash: []byte("h3"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
		err := refresh.Rotate(ctx, first.ID, third, now)
		var de *domain.Error
		require.ErrorAs(t, err, &de)
		assert.Equal(t, "token_reused", de.Code)
		_, err = refresh.GetByHash(ctx, []byte("h3"))
		assert.ErrorIs(t, err, domain.ErrNotFound, "failed rotation inserts nothing")

		got, err := refresh.GetByHash(ctx, []byte("h1"))
		require.NoError(t, err)
		require.NotNil(t, got.ReplacedBy)
		assert.Equal(t, second.ID, *got.ReplacedBy)

		require.NoError(t, refresh.RevokeFamily(ctx, fam, now))
		got, _ = refresh.GetByHash(ctx, []byte("h2"))
		assert.NotNil(t, got.RevokedAt)
	})

	t.Run("one-time tokens are single use and expire", func(t *testing.T) {
		u := newUser("ott@example.com")
		require.NoError(t, users.Create(ctx, u))
		now := time.Now().UTC()
		tok := &domain.OneTimeToken{ID: uuid.New(), UserID: u.ID, Purpose: domain.PurposeVerifyEmail, TokenHash: []byte("v1"), ExpiresAt: now.Add(time.Hour), CreatedAt: now}
		require.NoError(t, oneTime.Create(ctx, tok))

		_, err := oneTime.Consume(ctx, []byte("v1"), domain.PurposePasswordReset, now)
		assert.Error(t, err, "wrong purpose")
		got, err := oneTime.Consume(ctx, []byte("v1"), domain.PurposeVerifyEmail, now)
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.UserID)
		_, err = oneTime.Consume(ctx, []byte("v1"), domain.PurposeVerifyEmail, now)
		assert.Error(t, err, "second use")

		old := &domain.OneTimeToken{ID: uuid.New(), UserID: u.ID, Purpose: domain.PurposeVerifyEmail, TokenHash: []byte("v2"), ExpiresAt: now.Add(-time.Minute), CreatedAt: now}
		require.NoError(t, oneTime.Create(ctx, old))
		_, err = oneTime.Consume(ctx, []byte("v2"), domain.PurposeVerifyEmail, now)
		assert.Error(t, err, "expired")
	})

	t.Run("published releases are immutable at the database level", func(t *testing.T) {
		digest := "sha256:" + strings.Repeat("a", 64)
		var pubID, pkgID, relID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO publishers (slug, name) VALUES ('acme', 'Acme') RETURNING id`).Scan(&pubID))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO packages (publisher_id, slug, name, kind) VALUES ($1, 'kit', 'Kit', 'sample-pack') RETURNING id`, pubID).Scan(&pkgID))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO releases (package_id, version, manifest, manifest_digest) VALUES ($1, '1.0.0', '{"a":1}', $2) RETURNING id`, pkgID, digest).Scan(&relID))

		_, err := pool.Exec(ctx, `UPDATE releases SET manifest = '{"a":2}' WHERE id = $1`, relID)
		require.NoError(t, err, "drafts are editable")
		_, err = pool.Exec(ctx, `UPDATE releases SET status = 'published', published_at = now() WHERE id = $1`, relID)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `UPDATE releases SET manifest = '{"a":3}' WHERE id = $1`, relID)
		assert.ErrorContains(t, err, "immutable")
		_, err = pool.Exec(ctx, `UPDATE releases SET status = 'draft' WHERE id = $1`, relID)
		assert.ErrorContains(t, err, "invalid release status transition")
		_, err = pool.Exec(ctx, `DELETE FROM releases WHERE id = $1`, relID)
		assert.ErrorContains(t, err, "cannot be deleted")
		_, err = pool.Exec(ctx, `UPDATE releases SET status = 'yanked' WHERE id = $1`, relID)
		assert.NoError(t, err, "published → yanked is allowed")

		_, err = pool.Exec(ctx, `INSERT INTO releases (package_id, version, manifest, manifest_digest) VALUES ($1, '1.0', '{}', $2)`, pkgID, digest)
		assert.Error(t, err, "semver check")

		_, err = pool.Exec(ctx, `DELETE FROM packages WHERE id = $1`, pkgID)
		assert.NoError(t, err, "cascading package delete is still possible for admins")
	})
}
