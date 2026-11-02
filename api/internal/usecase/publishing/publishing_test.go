package publishing_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/memory"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/uploads"
)

var ctx = context.Background()

type env struct {
	pub     *publishing.Service
	up      *uploads.Service
	store   *memory.Store
	catalog *memory.Catalog
	blobs   *memory.Blobs
	now     time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	store := memory.NewStore()
	cat := memory.NewCatalog(store)
	blobs := memory.NewBlobs()
	v, err := manifest.NewValidator()
	require.NoError(t, err)
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	return &env{
		pub: publishing.NewService(publishing.Deps{Users: store.Users(), Publishers: cat.Publishers(), Packages: cat.Packages(),
			Releases: cat.Releases(), Blobs: blobs, Validator: v, Now: clock}),
		up:    uploads.NewService(uploads.Deps{Uploads: cat.Uploads(), Publishers: cat.Publishers(), Blobs: blobs, Now: clock}, uploads.Config{PartSize: 4}),
		store: store, catalog: cat, blobs: blobs, now: now,
	}
}

func (e *env) user(t *testing.T, email string, verified bool, role domain.Role) *domain.AccessClaims {
	t.Helper()
	u := &domain.User{ID: uuid.New(), Email: email, DisplayName: strings.Split(email, "@")[0], Role: role, CreatedAt: e.now, UpdatedAt: e.now}
	if verified {
		u.EmailVerifiedAt = &e.now
	}
	require.NoError(t, e.store.Users().Create(ctx, u))
	return &domain.AccessClaims{UserID: u.ID, Role: role}
}

func code(t *testing.T, err error) string {
	t.Helper()
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	return de.Code
}

func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

func manifestJSON(t *testing.T, version string, files map[string][]byte) json.RawMessage {
	t.Helper()
	var arts []map[string]any
	for path, data := range files {
		arts = append(arts, map[string]any{"kind": "sample", "path": path, "digest": digestOf(data), "size": len(data), "contentType": "audio/wav"})
	}
	m := map[string]any{
		"version":     version,
		"description": "Dusty drums",
		"license":     map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy":    map[string]any{"genres": map[string]any{"primary": "lo-fi"}, "instruments": []string{"drums"}, "moods": []string{"warm"}},
		"artifacts":   arts,
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

// upload pushes data through the upload API in 4-byte parts (out of order).
func (e *env) upload(t *testing.T, actor *domain.AccessClaims, data []byte) *domain.Upload {
	t.Helper()
	u, err := e.up.Create(ctx, actor, uploads.CreateInput{Digest: digestOf(data), Size: int64(len(data)), ContentType: "audio/wav"})
	require.NoError(t, err)
	if u.Status == domain.UploadDone {
		return u
	}
	for n := u.PartCount(); n >= 1; n-- {
		start := int64(n-1) * u.PartSize
		chunk := data[start : start+u.ExpectedPartSize(n)]
		_, err := e.up.PutPart(ctx, actor, u.ID, n, strings.NewReader(string(chunk)), int64(len(chunk)))
		require.NoError(t, err)
	}
	done, err := e.up.Complete(ctx, actor, u.ID)
	require.NoError(t, err)
	return done
}

func (e *env) setupPackage(t *testing.T) (owner *domain.AccessClaims) {
	t.Helper()
	owner = e.user(t, "owner@acme.audio", true, domain.RoleUser)
	_, err := e.pub.CreatePublisher(ctx, owner, publishing.CreatePublisherInput{Slug: "acme", Name: "Acme Audio"})
	require.NoError(t, err)
	_, err = e.pub.CreatePackage(ctx, owner, "acme", publishing.CreatePackageInput{Slug: "kit", Name: "Warm Kit", Kind: "sample-pack"})
	require.NoError(t, err)
	return owner
}

/* ---------------------------------------------------------------- tests */

func TestCreatePublisher(t *testing.T) {
	e := newEnv(t)
	unverified := e.user(t, "new@x.io", false, domain.RoleUser)
	_, err := e.pub.CreatePublisher(ctx, unverified, publishing.CreatePublisherInput{Slug: "newbie", Name: "Newbie"})
	assert.Equal(t, "email_unverified", code(t, err))

	u := e.user(t, "u@x.io", true, domain.RoleUser)
	_, err = e.pub.CreatePublisher(ctx, u, publishing.CreatePublisherInput{Slug: "Bad Slug!", Name: "x"})
	assert.Equal(t, "validation_failed", code(t, err))
	_, err = e.pub.CreatePublisher(ctx, u, publishing.CreatePublisherInput{Slug: "admin", Name: "Admin"})
	assert.Equal(t, "validation_failed", code(t, err), "reserved")

	p, err := e.pub.CreatePublisher(ctx, u, publishing.CreatePublisherInput{Slug: "Tape-Works", Name: "Tapeworks"})
	require.NoError(t, err)
	assert.Equal(t, "tape-works", p.Slug)
	got, _ := e.store.Users().GetByID(ctx, u.UserID)
	assert.Equal(t, domain.RolePublisher, got.Role, "plain users are promoted")

	_, err = e.pub.CreatePublisher(ctx, e.user(t, "v@x.io", true, domain.RoleUser), publishing.CreatePublisherInput{Slug: "tape-works", Name: "Copy"})
	assert.Equal(t, "publisher_slug_taken", code(t, err))

	ms, err := e.pub.MyPublishers(ctx, u)
	require.NoError(t, err)
	require.Len(t, ms, 1)
	assert.Equal(t, domain.MemberOwner, ms[0].Role)
}

func TestPermissionMatrix(t *testing.T) {
	e := newEnv(t)
	owner := e.setupPackage(t)
	maint := e.user(t, "maint@x.io", true, domain.RoleUser)
	upl := e.user(t, "upl@x.io", true, domain.RoleUser)
	stranger := e.user(t, "s@x.io", true, domain.RolePublisher)
	admin := e.user(t, "admin@x.io", true, domain.RoleAdmin)

	_, err := e.pub.AddMember(ctx, owner, "acme", "MAINT@x.io", domain.MemberMaintainer)
	require.NoError(t, err)
	_, err = e.pub.AddMember(ctx, owner, "acme", "upl@x.io", domain.MemberUploader)
	require.NoError(t, err)
	_, err = e.pub.AddMember(ctx, owner, "acme", "upl@x.io", domain.MemberUploader)
	assert.Equal(t, "member_exists", code(t, err))
	_, err = e.pub.AddMember(ctx, owner, "acme", "ghost@x.io", domain.MemberUploader)
	assert.Equal(t, "user_not_found", code(t, err))
	_, err = e.pub.AddMember(ctx, owner, "acme", "s@x.io", domain.MemberOwner)
	assert.Equal(t, "validation_failed", code(t, err), "ownership is not transferable this way")
	_, err = e.pub.AddMember(ctx, maint, "acme", "s@x.io", domain.MemberUploader)
	assert.Equal(t, "forbidden", code(t, err), "maintainers cannot manage members")

	mf := manifestJSON(t, "1.0.0", map[string][]byte{"a.wav": []byte("aaaa")})
	_, err = e.pub.CreateDraft(ctx, upl, "acme", "kit", publishing.DraftInput{Manifest: mf})
	require.NoError(t, err, "uploaders may create drafts")
	_, err = e.pub.Publish(ctx, upl, "acme", "kit", "1.0.0")
	assert.Equal(t, "forbidden", code(t, err), "uploaders cannot publish")
	_, err = e.pub.CreatePackage(ctx, upl, "acme", publishing.CreatePackageInput{Slug: "x", Name: "Xx", Kind: "effect"})
	assert.Equal(t, "forbidden", code(t, err))

	_, err = e.pub.CreateDraft(ctx, stranger, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "1.1.0", nil)})
	assert.Equal(t, "forbidden", code(t, err))
	_, err = e.pub.CreateDraft(ctx, nil, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "1.1.0", nil)})
	assert.Equal(t, "unauthenticated", code(t, err))

	_, err = e.pub.CreatePackage(ctx, maint, "acme", publishing.CreatePackageInput{Slug: "verb", Name: "Plate Verb", Kind: "effect"})
	assert.NoError(t, err, "maintainers manage packages")
	_, err = e.pub.CreatePackage(ctx, admin, "acme", publishing.CreatePackageInput{Slug: "admin-made", Name: "By Admin", Kind: "utility"})
	assert.NoError(t, err, "admins act as owners everywhere")

	ms, err := e.pub.Members(ctx, upl, "acme")
	require.NoError(t, err)
	assert.Len(t, ms, 3)
	_, err = e.pub.Members(ctx, stranger, "acme")
	assert.Equal(t, "forbidden", code(t, err))

	assert.NoError(t, e.pub.RemoveMember(ctx, upl, "acme", upl.UserID), "members may leave")
	assert.Equal(t, "validation_failed", code(t, e.pub.RemoveMember(ctx, owner, "acme", owner.UserID)), "owner stays")
	assert.Equal(t, "forbidden", code(t, e.pub.RemoveMember(ctx, maint, "acme", owner.UserID)))
}

func TestDraftValidationAndNormalization(t *testing.T) {
	e := newEnv(t)
	owner := e.setupPackage(t)

	res, err := e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Changelog: " first ", Manifest: manifestJSON(t, "1.0.0", map[string][]byte{"a.wav": []byte("x")})})
	require.NoError(t, err)
	r := res.Release
	assert.Equal(t, "1.0.0", r.Version, "version taken from the manifest")
	assert.Equal(t, "first", r.Changelog)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, r.ManifestDigest)
	var stored map[string]any
	require.NoError(t, json.Unmarshal(r.Manifest, &stored))
	assert.Equal(t, "acme.kit", stored["id"], "server fills identity fields")
	assert.Equal(t, map[string]any{"id": "acme", "name": "Acme Audio"}, stored["publisher"])
	assert.Equal(t, "sample-pack", stored["kind"])
	assert.Equal(t, r.ManifestDigest, digestOf(r.Manifest), "digest covers the stored canonical bytes")
	assert.NotEmpty(t, res.Warnings, "info about missing package BPM")

	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "1.0.0", map[string][]byte{"z.wav": []byte("z")})})
	assert.Equal(t, "version_exists", code(t, err))

	bad := json.RawMessage(`{"version":"2.0.0","kind":"effect","taxonomy":{"genres":{"primary":"Lo Fi"},"instruments":[],"moods":[]},"artifacts":[]}`)
	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: bad})
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	assert.Equal(t, "manifest_invalid", de.Code)
	assert.Equal(t, "manifest.kind", de.Fields[0].Field, "identity conflicts are reported first")

	bad = json.RawMessage(`{"version":"2.0.0","taxonomy":{"genres":{"primary":"Lo Fi"},"instruments":[],"moods":[]},"artifacts":[]}`)
	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: bad})
	require.ErrorAs(t, err, &de)
	var paths []string
	for _, f := range de.Fields {
		paths = append(paths, f.Field)
	}
	assert.Contains(t, paths, "manifest.taxonomy.genres.primary")
	assert.Contains(t, paths, "manifest.artifacts")
	assert.Contains(t, paths, "manifest", "missing license reported at the manifest root")

	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: json.RawMessage(`[1]`)})
	assert.Equal(t, "validation_failed", code(t, err))
	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Version: "v2", Manifest: manifestJSON(t, "", nil)})
	assert.Equal(t, "validation_failed", code(t, err))

	cl := "updated notes"
	upd, err := e.pub.UpdateDraft(ctx, owner, "acme", "kit", "1.0.0", publishing.UpdateDraftInput{Changelog: &cl, Manifest: manifestJSON(t, "1.0.0", map[string][]byte{"b.wav": []byte("yy")})})
	require.NoError(t, err)
	assert.Equal(t, "updated notes", upd.Release.Changelog)
	assert.NotEqual(t, r.ManifestDigest, upd.Release.ManifestDigest)

	issues, err := e.pub.ValidateManifest(ctx, owner, manifestJSON(t, "3.0.0", map[string][]byte{"c.wav": []byte("z")}), "acme", "kit")
	require.NoError(t, err)
	for _, i := range issues {
		assert.NotEqual(t, "error", i.Severity, i)
	}
}

func TestPublishLifecycle(t *testing.T) {
	e := newEnv(t)
	owner := e.setupPackage(t)
	files := map[string][]byte{"loops/a.wav": []byte("drum loop bytes"), "one-shots/b.wav": []byte("kick")}
	_, err := e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "1.0.0", files)})
	require.NoError(t, err)

	// Nothing uploaded yet → publishing is refused with per-artifact issues.
	_, err = e.pub.Publish(ctx, owner, "acme", "kit", "1.0.0")
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	assert.Equal(t, "release_incomplete", de.Code)
	assert.Len(t, de.Fields, 2)
	assert.Contains(t, de.Fields[0].Message, "has not been uploaded")

	e.upload(t, owner, files["loops/a.wav"])
	_, err = e.pub.Publish(ctx, owner, "acme", "kit", "1.0.0")
	require.ErrorAs(t, err, &de)
	assert.Len(t, de.Fields, 1, "one blob still missing")

	e.upload(t, owner, files["one-shots/b.wav"])
	rel, err := e.pub.Publish(ctx, owner, "acme", "kit", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, domain.ReleasePublished, rel.Status)
	require.NotNil(t, rel.PublishedBy)

	// The manifest is stored immutably in S3 with the same bytes / digest as in the DB.
	obj, ok := e.blobs.Object("manifests/acme.kit/1.0.0/manifest.json")
	require.True(t, ok)
	assert.Equal(t, rel.ManifestDigest, digestOf(obj))

	v, _, err := e.pub.GetPackage(ctx, nil, "acme", "kit")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", *v.LatestVersion)
	assert.Equal(t, 2, v.SampleCount)
	assert.Contains(t, e.catalog.Terms(rel.ID), domain.TaxonomyTerm{Domain: "genre", Slug: "lo-fi", Weight: 1})

	d, err := e.pub.GetRelease(ctx, nil, "acme", "kit", "1.0.0")
	require.NoError(t, err)
	require.Len(t, d.Artifacts, 3, "two samples + manifest")
	for _, a := range d.Artifacts {
		assert.Contains(t, a.DownloadURL, "https://blobs.test/")
	}

	// Immutability.
	_, err = e.pub.Publish(ctx, owner, "acme", "kit", "1.0.0")
	assert.Equal(t, "release_immutable", code(t, err))
	_, err = e.pub.UpdateDraft(ctx, owner, "acme", "kit", "1.0.0", publishing.UpdateDraftInput{Manifest: manifestJSON(t, "1.0.0", nil)})
	assert.Equal(t, "release_immutable", code(t, err))
	assert.Equal(t, "release_immutable", code(t, e.pub.DeleteDraft(ctx, owner, "acme", "kit", "1.0.0")))

	// 1.1.0 reuses a blob: no upload needed thanks to content addressing.
	_, err = e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "1.1.0", map[string][]byte{"loops/a.wav": files["loops/a.wav"]})})
	require.NoError(t, err)
	_, err = e.pub.Publish(ctx, owner, "acme", "kit", "1.1.0")
	require.NoError(t, err)
	v, _, _ = e.pub.GetPackage(ctx, nil, "acme", "kit")
	assert.Equal(t, "1.1.0", *v.LatestVersion)

	_, err = e.pub.Yank(ctx, owner, "acme", "kit", "1.1.0")
	require.NoError(t, err)
	v, _, _ = e.pub.GetPackage(ctx, nil, "acme", "kit")
	assert.Equal(t, "1.0.0", *v.LatestVersion, "yank falls back")
	_, err = e.pub.Yank(ctx, owner, "acme", "kit", "1.1.0")
	assert.Equal(t, "not_published", code(t, err))

	// Yanked releases stay readable for existing lockfiles.
	_, _, err = e.pub.Manifest(ctx, nil, "acme", "kit", "1.1.0")
	assert.NoError(t, err)

	list, total, err := e.pub.ListPackages(ctx, domain.PackageFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "kit", list[0].Slug)
}

func TestDraftsAndPrivatePackagesAreHidden(t *testing.T) {
	e := newEnv(t)
	owner := e.setupPackage(t)
	_, err := e.pub.CreateDraft(ctx, owner, "acme", "kit", publishing.DraftInput{Manifest: manifestJSON(t, "0.1.0", map[string][]byte{"a.wav": []byte("a")})})
	require.NoError(t, err)

	rels, err := e.pub.ListReleases(ctx, nil, "acme", "kit")
	require.NoError(t, err)
	assert.Empty(t, rels)
	rels, _ = e.pub.ListReleases(ctx, owner, "acme", "kit")
	assert.Len(t, rels, 1)
	_, err = e.pub.GetRelease(ctx, nil, "acme", "kit", "0.1.0")
	assert.Equal(t, "release_not_found", code(t, err))
	d, err := e.pub.GetRelease(ctx, owner, "acme", "kit", "0.1.0")
	require.NoError(t, err)
	require.Len(t, d.Artifacts, 1, "drafts list the declared artifacts")
	assert.Empty(t, d.Artifacts[0].DownloadURL, "but nothing is downloadable yet")

	_, total, _ := e.pub.ListPackages(ctx, domain.PackageFilter{})
	assert.Equal(t, 0, total, "unpublished packages are not listed")

	priv := "private"
	_, err = e.pub.UpdatePackage(ctx, owner, "acme", "kit", publishing.UpdatePackageInput{Visibility: &priv})
	require.NoError(t, err)
	_, _, err = e.pub.GetPackage(ctx, nil, "acme", "kit")
	assert.Equal(t, "package_not_found", code(t, err))
	_, _, err = e.pub.GetPackage(ctx, owner, "acme", "kit")
	assert.NoError(t, err)

	dash, err := e.pub.PublisherPackages(ctx, owner, "acme")
	require.NoError(t, err)
	require.Len(t, dash, 1)
	assert.Len(t, dash[0].Releases, 1)
}

func TestUploads(t *testing.T) {
	e := newEnv(t)
	owner := e.setupPackage(t)
	data := []byte("0123456789") // 3 parts of 4/4/2 bytes

	t.Run("happy path, parts out of order", func(t *testing.T) {
		u := e.upload(t, owner, data)
		assert.Equal(t, domain.UploadDone, u.Status)
		key, _ := domain.BlobKey(digestOf(data))
		got, ok := e.blobs.Object(key)
		require.True(t, ok)
		assert.Equal(t, data, got)
		for _, k := range e.blobs.Keys() {
			assert.False(t, strings.HasPrefix(k, "uploads/"), "temporary objects are removed: %s", k)
		}
	})

	t.Run("existing blob is deduplicated", func(t *testing.T) {
		u, err := e.up.Create(ctx, owner, uploads.CreateInput{Digest: digestOf(data), Size: int64(len(data))})
		require.NoError(t, err)
		assert.Equal(t, domain.UploadDone, u.Status)
		_, err = e.up.Create(ctx, owner, uploads.CreateInput{Digest: digestOf(data), Size: 99})
		assert.Equal(t, "digest_size_mismatch", code(t, err))
	})

	t.Run("tampered content is rejected", func(t *testing.T) {
		claimed := []byte("honest-bytes")
		u, err := e.up.Create(ctx, owner, uploads.CreateInput{Digest: digestOf(claimed), Size: int64(len(claimed))})
		require.NoError(t, err)
		evil := []byte("EVIL-bytes!!")
		for n := 1; n <= u.PartCount(); n++ {
			start := int64(n-1) * u.PartSize
			chunk := evil[start : start+u.ExpectedPartSize(n)]
			_, err := e.up.PutPart(ctx, owner, u.ID, n, strings.NewReader(string(chunk)), int64(len(chunk)))
			require.NoError(t, err)
		}
		_, err = e.up.Complete(ctx, owner, u.ID)
		assert.Equal(t, "digest_mismatch", code(t, err))
		key, _ := domain.BlobKey(digestOf(claimed))
		_, ok := e.blobs.Object(key)
		assert.False(t, ok, "nothing stored under the claimed digest")
		st, err := e.up.Get(ctx, owner, u.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UploadFailed, st.Upload.Status)
		_, err = e.up.PutPart(ctx, owner, u.ID, 1, strings.NewReader("aaaa"), 4)
		assert.Equal(t, "upload_finished", code(t, err))
	})

	t.Run("part validation, missing parts, ownership", func(t *testing.T) {
		d := []byte("abcdefghij")
		u, err := e.up.Create(ctx, owner, uploads.CreateInput{Digest: digestOf(d), Size: 10})
		require.NoError(t, err)
		_, err = e.up.PutPart(ctx, owner, u.ID, 1, strings.NewReader("abc"), 3)
		assert.Equal(t, "validation_failed", code(t, err), "wrong part size")
		_, err = e.up.PutPart(ctx, owner, u.ID, 4, strings.NewReader("a"), 1)
		assert.Equal(t, "validation_failed", code(t, err), "part out of range")
		_, err = e.up.PutPart(ctx, owner, u.ID, 1, strings.NewReader("abcd"), 4)
		require.NoError(t, err)
		_, err = e.up.Complete(ctx, owner, u.ID)
		var de *domain.Error
		require.ErrorAs(t, err, &de)
		assert.Equal(t, "missing parts: [2 3]", de.Fields[0].Message)

		other := e.user(t, "other@x.io", true, domain.RolePublisher)
		_, err = e.up.Get(ctx, other, u.ID)
		assert.Equal(t, "upload_not_found", code(t, err))
		require.NoError(t, e.up.Abort(ctx, owner, u.ID))
		assert.Equal(t, "upload_finished", code(t, e.up.Abort(ctx, owner, u.ID)))
	})

	t.Run("who may upload", func(t *testing.T) {
		plain := e.user(t, "plain@x.io", true, domain.RoleUser)
		_, err := e.up.Create(ctx, plain, uploads.CreateInput{Digest: digestOf([]byte("q")), Size: 1})
		assert.Equal(t, "not_a_publisher", code(t, err))
		_, err = e.up.Create(ctx, nil, uploads.CreateInput{Digest: digestOf([]byte("q")), Size: 1})
		assert.Equal(t, "unauthenticated", code(t, err))
		_, err = e.up.Create(ctx, owner, uploads.CreateInput{Digest: "md5:x", Size: 0})
		var de *domain.Error
		require.ErrorAs(t, err, &de)
		assert.Len(t, de.Fields, 2)
	})

	t.Run("completing twice is idempotent", func(t *testing.T) {
		d := []byte(fmt.Sprintf("unique-%d", time.Now().UnixNano()))
		u := e.upload(t, owner, d)
		again, err := e.up.Complete(ctx, owner, u.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UploadDone, again.Status)
	})
}
