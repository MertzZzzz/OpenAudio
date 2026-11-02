package library_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/memory"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/canonjson"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/library"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
)

var ctx = context.Background()

type env struct {
	lib   *library.Service
	pub   *publishing.Service
	blobs *memory.Blobs
	store *memory.Store
	owner *domain.AccessClaims
	user  *domain.AccessClaims
	n     int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	store := memory.NewStore()
	cat := memory.NewCatalog(store)
	blobs := memory.NewBlobs()
	v, err := manifest.NewValidator()
	require.NoError(t, err)
	e := &env{store: store, blobs: blobs,
		pub: publishing.NewService(publishing.Deps{Users: store.Users(), Publishers: cat.Publishers(), Packages: cat.Packages(), Releases: cat.Releases(), Blobs: blobs, Validator: v}),
		lib: library.NewService(library.Deps{Libraries: cat.Libraries(), Lockfiles: cat.Lockfiles(), Packages: cat.Packages(), Releases: cat.Releases()}),
	}
	e.owner = e.mkUser(t, "owner@x.io")
	e.user = e.mkUser(t, "user@x.io")
	_, err = e.pub.CreatePublisher(ctx, e.owner, publishing.CreatePublisherInput{Slug: "acme", Name: "Acme"})
	require.NoError(t, err)
	return e
}

func (e *env) mkUser(t *testing.T, email string) *domain.AccessClaims {
	now := time.Now()
	u := &domain.User{ID: uuid.New(), Email: email, DisplayName: "x", Role: domain.RoleUser, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, e.store.Users().Create(ctx, u))
	return &domain.AccessClaims{UserID: u.ID, Role: domain.RoleUser}
}

// release publishes slug@version with dependencies (creating the package on first use).
func (e *env) release(t *testing.T, slug, version string, deps ...[2]string) string {
	t.Helper()
	_, _ = e.pub.CreatePackage(ctx, e.owner, "acme", publishing.CreatePackageInput{Slug: slug, Name: strings.ToUpper(slug[:1]) + slug[1:], Kind: "sample-pack"})
	e.n++
	data := []byte(slug + version)
	digest := canonjson.Digest(data)
	key, _ := domain.BlobKey(digest)
	require.NoError(t, e.blobs.Put(ctx, key, data, ""))
	var ds []map[string]string
	for _, d := range deps {
		ds = append(ds, map[string]string{"id": d[0], "range": d[1]})
	}
	m := map[string]any{"version": version, "license": map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy":  map[string]any{"genres": map[string]any{"primary": "lo-fi"}, "instruments": []string{}, "moods": []string{}},
		"artifacts": []any{map[string]any{"kind": "sample", "path": "a.wav", "digest": digest, "size": len(data)}}}
	if ds != nil {
		m["dependencies"] = ds
	}
	raw, _ := json.Marshal(m)
	res, err := e.pub.CreateDraft(ctx, e.owner, "acme", slug, publishing.DraftInput{Manifest: raw})
	require.NoError(t, err)
	_, err = e.pub.Publish(ctx, e.owner, "acme", slug, version)
	require.NoError(t, err)
	return res.Release.ManifestDigest
}

func code(t *testing.T, err error) string {
	t.Helper()
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	return de.Code
}

func sp(s string) *string { return &s }

func (e *env) defaultLib(t *testing.T, a *domain.AccessClaims) uuid.UUID {
	libs, err := e.lib.Libraries(ctx, a)
	require.NoError(t, err)
	return libs[0].ID
}

func TestLibraries(t *testing.T) {
	e := newEnv(t)
	libs, err := e.lib.Libraries(ctx, e.user)
	require.NoError(t, err)
	require.Len(t, libs, 1)
	assert.Equal(t, library.DefaultLibraryName, libs[0].Name)
	again, _ := e.lib.Libraries(ctx, e.user)
	assert.Equal(t, libs[0].ID, again[0].ID, "the default library is created once")

	_, err = e.lib.CreateLibrary(ctx, e.user, "my library")
	assert.Equal(t, "library_exists", code(t, err))
	_, err = e.lib.CreateLibrary(ctx, e.user, "  ")
	assert.Equal(t, "validation_failed", code(t, err))
	live, err := e.lib.CreateLibrary(ctx, e.user, "Live set")
	require.NoError(t, err)

	_, err = e.lib.Items(ctx, e.owner, live.ID)
	assert.Equal(t, "library_not_found", code(t, err), "libraries are private")
	_, err = e.lib.Libraries(ctx, nil)
	assert.Equal(t, "unauthenticated", code(t, err))

	renamed, err := e.lib.RenameLibrary(ctx, e.user, live.ID, "Gig")
	require.NoError(t, err)
	assert.Equal(t, "Gig", renamed.Name)
	require.NoError(t, e.lib.DeleteLibrary(ctx, e.user, live.ID))
}

func TestItemsPinsAndUpgrades(t *testing.T) {
	e := newEnv(t)
	e.release(t, "kit", "1.0.0")
	lib := e.defaultLib(t, e.user)

	_, err := e.lib.AddItem(ctx, e.user, lib, "acme.nope", nil)
	assert.Equal(t, "package_not_found", code(t, err))
	_, _ = e.pub.CreatePackage(ctx, e.owner, "acme", publishing.CreatePackageInput{Slug: "empty", Name: "Empty", Kind: "effect"})
	_, err = e.lib.AddItem(ctx, e.user, lib, "acme.empty", nil)
	assert.Equal(t, "validation_failed", code(t, err), "nothing published yet")
	_, err = e.lib.AddItem(ctx, e.user, lib, "acme.kit", sp("9.9.9"))
	assert.Equal(t, "validation_failed", code(t, err))

	it, err := e.lib.AddItem(ctx, e.user, lib, "acme.kit", nil)
	require.NoError(t, err)
	assert.Nil(t, it.PinnedVersion)
	assert.Equal(t, "1.0.0", *it.InstalledVersion)

	e.release(t, "kit", "1.1.0")
	it, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.kit", library.UpdateItemInput{Upgrade: true})
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", *it.InstalledVersion)

	it, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.kit", library.UpdateItemInput{SetPin: true, PinnedVersion: sp("1.0.0")})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", *it.PinnedVersion)
	assert.Equal(t, "1.0.0", *it.InstalledVersion)
	_, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.kit", library.UpdateItemInput{Upgrade: true})
	assert.Equal(t, "item_pinned", code(t, err))
	it, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.kit", library.UpdateItemInput{SetPin: true})
	require.NoError(t, err)
	assert.Nil(t, it.PinnedVersion)

	require.NoError(t, e.lib.RemoveItem(ctx, e.user, lib, "acme.kit"))
	assert.Equal(t, "item_not_found", code(t, e.lib.RemoveItem(ctx, e.user, lib, "acme.kit")))
}

func TestResolveExportImport(t *testing.T) {
	e := newEnv(t)
	drumsDigest := e.release(t, "drums", "2.0.1")
	e.release(t, "drums", "1.0.0")
	e.release(t, "template", "1.0.0", [2]string{"acme.drums", "^2.0.0"})
	lib := e.defaultLib(t, e.user)
	_, err := e.lib.AddItem(ctx, e.user, lib, "acme.template", nil)
	require.NoError(t, err)

	res, err := e.lib.Resolve(ctx, e.user, lib)
	require.NoError(t, err)
	assert.Empty(t, res.Conflicts)
	require.Len(t, res.Lockfile.Packages, 2)
	d := res.Lockfile.Packages["acme.drums"]
	assert.Equal(t, "2.0.1", d.Version)
	assert.Equal(t, drumsDigest, d.ManifestDigest, "exact digest from the registry")
	assert.False(t, d.Direct)
	assert.Equal(t, "openaudio://acme/drums@2.0.1", d.Resolved)
	assert.Equal(t, map[string]string{"acme.drums": "^2.0.0"}, res.Lockfile.Packages["acme.template"].Dependencies)

	// A pin that violates the dependency range blocks the export.
	_, err = e.lib.AddItem(ctx, e.user, lib, "acme.drums", sp("1.0.0"))
	require.NoError(t, err)
	_, res, err = e.lib.SaveLockfile(ctx, e.user, lib)
	assert.Equal(t, "resolve_conflicts", code(t, err))
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, "acme.drums", res.Conflicts[0].PackageID)
	_, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.drums", library.UpdateItemInput{SetPin: true})
	require.NoError(t, err)
	require.NoError(t, e.lib.RemoveItem(ctx, e.user, lib, "acme.drums"))

	rec, _, err := e.lib.SaveLockfile(ctx, e.user, lib)
	require.NoError(t, err)
	assert.Equal(t, canonjson.Digest(rec.Content), rec.ContentDigest, "digest covers the exact stored bytes")
	list, err := e.lib.ListLockfiles(ctx, e.user)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	_, err = e.lib.Lockfile(ctx, e.owner, rec.ID)
	assert.Equal(t, "lockfile_not_found", code(t, err), "lockfiles are private")

	// Import into someone else's library: only direct entries become pins, all are verified.
	other := e.defaultLib(t, e.owner)
	imp, err := e.lib.Import(ctx, e.owner, other, rec.Content)
	require.NoError(t, err)
	assert.Equal(t, 1, imp.Pinned)
	items, _ := e.lib.Items(ctx, e.owner, other)
	require.Len(t, items, 1)
	assert.Equal(t, "acme.template", items[0].Package.CanonicalID())
	assert.Equal(t, "1.0.0", *items[0].PinnedVersion)

	tampered := strings.Replace(string(rec.Content), drumsDigest, "sha256:"+strings.Repeat("0", 64), 1)
	_, err = e.lib.Import(ctx, e.owner, other, json.RawMessage(tampered))
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	assert.Equal(t, "lockfile_invalid", de.Code)
	assert.Equal(t, "lockfile.packages.acme.drums.manifestDigest", de.Fields[0].Field)
	assert.Contains(t, de.Fields[0].Message, "integrity check failed")

	_, err = e.lib.Import(ctx, e.owner, other, json.RawMessage(`{"lockfileVersion":1,"packages":{"ghost.pkg":{"version":"1.0.0","manifestDigest":"x"}}}`))
	assert.Equal(t, "lockfile_invalid", code(t, err))
	_, err = e.lib.Import(ctx, e.owner, other, json.RawMessage(`{"lockfileVersion":2,"packages":{}}`))
	assert.Equal(t, "validation_failed", code(t, err))
}

func TestYankedPinsSurviveWithWarning(t *testing.T) {
	e := newEnv(t)
	e.release(t, "kit", "1.0.0")
	e.release(t, "kit", "1.1.0")
	lib := e.defaultLib(t, e.user)
	_, err := e.lib.AddItem(ctx, e.user, lib, "acme.kit", sp("1.1.0"))
	require.NoError(t, err)
	_, err = e.pub.Yank(ctx, e.owner, "acme", "kit", "1.1.0")
	require.NoError(t, err)

	res, err := e.lib.Resolve(ctx, e.user, lib)
	require.NoError(t, err)
	assert.Empty(t, res.Conflicts)
	assert.True(t, res.Lockfile.Packages["acme.kit"].Yanked)
	require.Len(t, res.Warnings, 1)

	_, err = e.lib.UpdateItem(ctx, e.user, lib, "acme.kit", library.UpdateItemInput{SetPin: true, PinnedVersion: sp("1.1.0")})
	assert.Equal(t, "validation_failed", code(t, err), "new pins to yanked versions are refused")
}
