package postgres_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/postgres"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

func TestCatalogRepos(t *testing.T) {
	pool := setup(t)
	users := postgres.NewUserRepo(pool)
	pubs := postgres.NewPublisherRepo(pool)
	pkgs := postgres.NewPackageRepo(pool)
	rels := postgres.NewReleaseRepo(pool)
	ups := postgres.NewUploadRepo(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	owner, other := newUser("owner@example.com"), newUser("other@example.com")
	require.NoError(t, users.Create(ctx, owner))
	require.NoError(t, users.Create(ctx, other))

	pub := &domain.Publisher{ID: uuid.New(), Slug: "acme", Name: "Acme Audio", CreatedAt: now}
	var pkg *domain.Package

	t.Run("publishers and members", func(t *testing.T) {
		require.NoError(t, pubs.Create(ctx, pub, owner.ID))
		assert.ErrorIs(t, pubs.Create(ctx, &domain.Publisher{ID: uuid.New(), Slug: "ACME", Name: "x", CreatedAt: now}, other.ID), domain.ErrConflict)

		role, err := pubs.MemberRole(ctx, pub.ID, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.MemberOwner, role)
		_, err = pubs.MemberRole(ctx, pub.ID, other.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		require.NoError(t, pubs.AddMember(ctx, pub.ID, other.ID, domain.MemberUploader))
		assert.ErrorIs(t, pubs.AddMember(ctx, pub.ID, other.ID, domain.MemberMaintainer), domain.ErrConflict)
		ms, err := pubs.Members(ctx, pub.ID)
		require.NoError(t, err)
		require.Len(t, ms, 2)
		assert.Equal(t, domain.MemberOwner, ms[0].Role)
		assert.Equal(t, "other@example.com", ms[1].Email)

		list, err := pubs.ListForUser(ctx, other.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "acme", list[0].Publisher.Slug)
		require.NoError(t, pubs.RemoveMember(ctx, pub.ID, other.ID))
		assert.ErrorIs(t, pubs.RemoveMember(ctx, pub.ID, other.ID), domain.ErrNotFound)
	})

	t.Run("packages", func(t *testing.T) {
		pkg = &domain.Package{ID: uuid.New(), PublisherID: pub.ID, Slug: "kit", Name: "Warm Kit", Kind: "sample-pack", Description: "dusty 100% drums", Visibility: "public", CreatedAt: now, UpdatedAt: now}
		require.NoError(t, pkgs.Create(ctx, pkg))
		assert.ErrorIs(t, pkgs.Create(ctx, &domain.Package{ID: uuid.New(), PublisherID: pub.ID, Slug: "kit", Name: "x", Kind: "effect", Visibility: "public", CreatedAt: now, UpdatedAt: now}), domain.ErrConflict)

		v, err := pkgs.Get(ctx, "acme", "kit")
		require.NoError(t, err)
		assert.Equal(t, "acme.kit", v.CanonicalID())
		assert.Nil(t, v.LatestVersion)
		assert.Nil(t, v.LatestManifest)

		list, total, err := pkgs.List(ctx, domain.PackageFilter{})
		require.NoError(t, err)
		assert.Equal(t, 0, total, "unpublished packages are hidden from the public list")
		assert.Empty(t, list)
		_, total, _ = pkgs.List(ctx, domain.PackageFilter{PublisherSlug: "acme", IncludeUnpublished: true})
		assert.Equal(t, 1, total)
		_, total, _ = pkgs.List(ctx, domain.PackageFilter{IncludeUnpublished: true, Query: "100%"})
		assert.Equal(t, 1, total, "LIKE wildcards in queries are escaped, literal % still matches")
		_, total, _ = pkgs.List(ctx, domain.PackageFilter{IncludeUnpublished: true, Query: "_"})
		assert.Equal(t, 0, total)
	})

	digest := func(c string) string { return "sha256:" + strings.Repeat(c, 64) }
	manifest := []byte(`{"id":"acme.kit","version":"1.0.0"}`)

	t.Run("release lifecycle", func(t *testing.T) {
		for _, v := range []string{"1.0.0", "1.10.0", "1.9.0"} {
			require.NoError(t, rels.CreateDraft(ctx, &domain.Release{ID: uuid.New(), PackageID: pkg.ID, Version: v, Manifest: manifest, ManifestDigest: digest("a"), CreatedBy: &owner.ID, CreatedAt: now}))
		}
		assert.ErrorIs(t, rels.CreateDraft(ctx, &domain.Release{ID: uuid.New(), PackageID: pkg.ID, Version: "1.0.0", Manifest: manifest, ManifestDigest: digest("a"), CreatedAt: now}), domain.ErrConflict)

		list, err := rels.List(ctx, pkg.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1.10.0", "1.9.0", "1.0.0"}, []string{list[0].Version, list[1].Version, list[2].Version}, "semver order, not lexical")

		r9, _ := rels.Get(ctx, pkg.ID, "1.9.0")
		require.NoError(t, rels.UpdateDraft(ctx, r9.ID, []byte(`{"changed":true}`), digest("b"), "notes"))

		publish := func(r *domain.Release, arts ...domain.Artifact) error {
			return rels.Publish(ctx, domain.PublishInput{ReleaseID: r.ID, PackageID: pkg.ID, PublishedBy: owner.ID, Artifacts: arts, At: now,
				Terms: []domain.TaxonomyTerm{{Domain: "genre", Slug: "lo-fi", Weight: 1}, {Domain: "mood", Slug: "warm", Weight: 1}}})
		}
		art := func(path string, size int64) domain.Artifact {
			return domain.Artifact{ID: uuid.New(), Kind: "sample", Path: path, Digest: digest("c"), SizeBytes: size, ContentType: "audio/wav", StorageKey: "blobs/" + strings.Repeat("c", 64)}
		}
		require.NoError(t, publish(r9, art("a.wav", 10), art("b.wav", 32)))
		assert.ErrorContains(t, publish(r9), "only draft releases", "double publish")

		v, err := pkgs.Get(ctx, "acme", "kit")
		require.NoError(t, err)
		require.NotNil(t, v.LatestVersion)
		assert.Equal(t, "1.9.0", *v.LatestVersion)
		assert.JSONEq(t, `{"changed":true}`, string(v.LatestManifest))
		assert.Equal(t, 2, v.SampleCount)
		assert.EqualValues(t, 42, v.TotalSize)

		r10, _ := rels.Get(ctx, pkg.ID, "1.10.0")
		require.NoError(t, publish(r10, art("a.wav", 10)))
		v, _ = pkgs.Get(ctx, "acme", "kit")
		assert.Equal(t, "1.10.0", *v.LatestVersion, "numeric semver max")

		assert.ErrorContains(t, rels.UpdateDraft(ctx, r10.ID, manifest, digest("d"), ""), "only draft releases")
		assert.ErrorContains(t, rels.DeleteDraft(ctx, r10.ID), "only draft releases")

		// Artifacts of published releases are frozen at the DB level too.
		_, err = pool.Exec(ctx, `INSERT INTO artifacts (release_id, kind, path, digest, size_bytes, storage_key) VALUES ($1,'sample','x.wav',$2,1,'k')`, r10.ID, digest("e"))
		assert.ErrorContains(t, err, "cannot be modified")
		_, err = pool.Exec(ctx, `DELETE FROM artifacts WHERE release_id = $1`, r10.ID)
		assert.ErrorContains(t, err, "cannot be modified")

		require.NoError(t, rels.Yank(ctx, r10.ID, pkg.ID, now))
		v, _ = pkgs.Get(ctx, "acme", "kit")
		assert.Equal(t, "1.9.0", *v.LatestVersion, "yank falls back to the next published version")
		assert.ErrorContains(t, rels.Yank(ctx, r10.ID, pkg.ID, now), "only published")

		r1, _ := rels.Get(ctx, pkg.ID, "1.0.0")
		require.NoError(t, rels.DeleteDraft(ctx, r1.ID))

		arts, err := rels.Artifacts(ctx, r9.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"a.wav", "b.wav"}, []string{arts[0].Path, arts[1].Path})

		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM package_taxonomy WHERE release_id = $1`, r9.ID).Scan(&n))
		assert.Equal(t, 2, n)

		views, total, err := pkgs.List(ctx, domain.PackageFilter{Kinds: []string{"sample-pack"}})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, "1.9.0", *views[0].LatestVersion)
		_, total, _ = pkgs.List(ctx, domain.PackageFilter{Kinds: []string{"effect"}})
		assert.Equal(t, 0, total)

		// Deleting a user keeps their releases (audit columns are nulled).
		_, err = pool.Exec(ctx, `DELETE FROM publisher_members WHERE user_id = $1`, owner.ID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, err)
		got, _ := rels.Get(ctx, pkg.ID, "1.9.0")
		assert.Nil(t, got.PublishedBy)
	})

	t.Run("uploads", func(t *testing.T) {
		u := &domain.Upload{ID: uuid.New(), UserID: other.ID, Digest: digest("f"), Size: 20, ContentType: "audio/wav", Status: domain.UploadPending, MultipartID: "mp", PartSize: 8, CreatedAt: now}
		require.NoError(t, ups.Create(ctx, u))
		assert.Equal(t, 3, u.PartCount())
		assert.EqualValues(t, 4, u.ExpectedPartSize(3))
		require.NoError(t, ups.PutPart(ctx, u.ID, domain.UploadPart{Number: 2, ETag: "e2", Size: 8}))
		require.NoError(t, ups.PutPart(ctx, u.ID, domain.UploadPart{Number: 1, ETag: "old", Size: 8}))
		require.NoError(t, ups.PutPart(ctx, u.ID, domain.UploadPart{Number: 1, ETag: "e1", Size: 8}))
		parts, err := ups.Parts(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.UploadPart{{1, "e1", 8}, {2, "e2", 8}}, parts)
		require.NoError(t, ups.Finish(ctx, u.ID, domain.UploadDone, "", now))
		assert.Error(t, ups.Finish(ctx, u.ID, domain.UploadFailed, "x", now))
		got, err := ups.Get(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UploadDone, got.Status)
	})
}
