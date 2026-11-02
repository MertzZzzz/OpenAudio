package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/postgres"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

func TestLibraryRepos(t *testing.T) {
	pool := setup(t)
	users := postgres.NewUserRepo(pool)
	libs := postgres.NewLibraryRepo(pool)
	locks := postgres.NewLockfileRepo(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	u := newUser("lib@example.com")
	require.NoError(t, users.Create(ctx, u))
	s := &seeder{t: t, pubs: postgres.NewPublisherRepo(pool), pkgs: postgres.NewPackageRepo(pool), rels: postgres.NewReleaseRepo(pool), user: u.ID, ids: map[string]uuid.UUID{}, at: now}
	s.publisher("acme", "Acme Audio")
	kit := s.publish("acme", "kit", "Kit", "sample-pack", "1.0.0", map[string]any{
		"license":  map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy": tax("lo-fi", nil, nil, nil), "artifacts": []any{art("a.wav", 'a')},
	})

	lib := &domain.Library{ID: uuid.New(), UserID: u.ID, Name: "My Library", CreatedAt: now}
	require.NoError(t, libs.Create(ctx, lib))
	assert.ErrorIs(t, libs.Create(ctx, &domain.Library{ID: uuid.New(), UserID: u.ID, Name: "my library", CreatedAt: now}), domain.ErrConflict, "names are unique per user, case-insensitively")
	second := &domain.Library{ID: uuid.New(), UserID: u.ID, Name: "Live set", CreatedAt: now.Add(time.Second)}
	require.NoError(t, libs.Create(ctx, second))
	list, err := libs.ListForUser(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"My Library", "Live set"}, []string{list[0].Name, list[1].Name})
	assert.ErrorIs(t, libs.Rename(ctx, second.ID, "MY LIBRARY"), domain.ErrConflict)
	require.NoError(t, libs.Rename(ctx, second.ID, "Gig"))

	v := "1.0.0"
	require.NoError(t, libs.PutItem(ctx, lib.ID, kit, nil, &v, now))
	require.NoError(t, libs.PutItem(ctx, lib.ID, kit, &v, &v, now.Add(time.Hour)))
	items, err := libs.Items(ctx, lib.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "acme.kit", items[0].Package.CanonicalID())
	assert.Equal(t, "1.0.0", *items[0].PinnedVersion)
	assert.True(t, items[0].AddedAt.Equal(now), "upsert keeps added_at")
	bad := "1.0"
	assert.Error(t, libs.PutItem(ctx, lib.ID, kit, &bad, nil, now), "version check constraint")
	require.NoError(t, libs.RemoveItem(ctx, lib.ID, kit))
	assert.ErrorIs(t, libs.RemoveItem(ctx, lib.ID, kit), domain.ErrNotFound)

	content := []byte(`{"lockfileVersion":1,"name":"x","packages":{"b":1,"a":2}}`)
	rec := &domain.LockfileRecord{ID: uuid.New(), OwnerUserID: u.ID, LibraryID: &lib.ID, Name: "My Library", Content: content, ContentDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000", CreatedAt: now}
	require.NoError(t, locks.Create(ctx, rec))
	got, err := locks.Get(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(got.Content), "exact bytes round-trip")
	ls, err := locks.ListForUser(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, ls, 1)
	assert.Nil(t, ls[0].Content)

	require.NoError(t, libs.Delete(ctx, lib.ID))
	got, _ = locks.Get(ctx, rec.ID)
	assert.Nil(t, got.LibraryID, "lockfiles outlive their library")
	require.NoError(t, locks.Delete(ctx, rec.ID))
	assert.ErrorIs(t, locks.Delete(ctx, rec.ID), domain.ErrNotFound)
}
