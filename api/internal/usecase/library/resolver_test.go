package library

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

func pub(v string, deps ...Dependency) ReleaseInfo {
	return ReleaseInfo{Version: v, Status: domain.ReleasePublished, ManifestDigest: "sha256:" + v, Dependencies: deps}
}

func yanked(v string) ReleaseInfo {
	return ReleaseInfo{Version: v, Status: domain.ReleaseYanked, ManifestDigest: "sha256:" + v}
}

func dep(id, r string) Dependency { return Dependency{ID: id, Range: r} }

func loader(pkgs map[string][]ReleaseInfo) Loader {
	return func(_ context.Context, id string) (*PackageInfo, error) {
		rels, ok := pkgs[id]
		if !ok {
			return nil, nil
		}
		return &PackageInfo{ID: id, Releases: rels}, nil
	}
}

func versions(r *Resolution) map[string]string {
	out := map[string]string{}
	for id, e := range r.Entries {
		out[id] = e.Version
	}
	return out
}

var ctx = context.Background()

func TestNewestSatisfyingVersionsAndTransitives(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{
		"acme.template": {pub("1.0.0", dep("tape.drums", "^2.0.0"), dep("null.sat", "^1.5.0"))},
		"tape.drums":    {pub("2.1.0"), pub("2.0.1"), pub("1.9.0")},
		"null.sat":      {pub("1.6.0", dep("lib.dsp", "~0.3.0")), pub("1.4.0")},
		"lib.dsp":       {pub("0.4.0"), pub("0.3.7"), pub("0.3.2")},
	})
	r, err := Resolve(ctx, []Root{{ID: "acme.template"}}, l)
	require.NoError(t, err)
	assert.Empty(t, r.Conflicts)
	assert.Equal(t, map[string]string{"acme.template": "1.0.0", "tape.drums": "2.1.0", "null.sat": "1.6.0", "lib.dsp": "0.3.7"}, versions(r))
	assert.True(t, r.Entries["acme.template"].Direct)
	assert.False(t, r.Entries["lib.dsp"].Direct)
	assert.Equal(t, "sha256:0.3.7", r.Entries["lib.dsp"].ManifestDigest)
}

func TestInstalledVersionIsKeptWhileValid(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{"a.kit": {pub("1.2.0"), pub("1.1.0")}})
	r, err := Resolve(ctx, []Root{{ID: "a.kit", Installed: "1.1.0"}}, l)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", r.Entries["a.kit"].Version, "like a lockfile: no silent upgrades")
}

func TestPinConflictWithDependencyRange(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{
		"t.template": {pub("1.0.0", dep("t.drums", "^2.0.0"))},
		"t.drums":    {pub("2.0.1"), pub("1.0.0")},
	})
	r, err := Resolve(ctx, []Root{{ID: "t.template"}, {ID: "t.drums", Pinned: "1.0.0"}}, l)
	require.NoError(t, err)
	require.Len(t, r.Conflicts, 1)
	c := r.Conflicts[0]
	assert.Equal(t, "t.drums", c.PackageID)
	assert.Equal(t, "no_matching_version", c.Reason)
	assert.ElementsMatch(t, []Request{{From: "library", Range: "1.0.0"}, {From: "t.template", Range: "^2.0.0"}}, c.RequestedBy)
	assert.Equal(t, []string{"2.0.1", "1.0.0"}, c.Candidates)
}

func TestYankedReleases(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{"a.kit": {yanked("1.3.0"), pub("1.2.0")}, "a.app": {pub("1.0.0", dep("a.kit", "^1.0.0"))}})
	r, err := Resolve(ctx, []Root{{ID: "a.app"}}, l)
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", r.Entries["a.kit"].Version, "yanked versions are never chosen by ranges")

	r, err = Resolve(ctx, []Root{{ID: "a.kit", Pinned: "1.3.0"}}, l)
	require.NoError(t, err)
	assert.Empty(t, r.Conflicts)
	assert.True(t, r.Entries["a.kit"].Yanked, "explicit pins keep old lockfiles installable")
	require.Len(t, r.Warnings, 1)
	assert.Contains(t, r.Warnings[0].Message, "yanked")
}

func TestMissingDependency(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{"a.app": {pub("1.0.0", dep("ghost.pkg", "^1.0.0"))}})
	r, err := Resolve(ctx, []Root{{ID: "a.app"}}, l)
	require.NoError(t, err)
	require.Len(t, r.Conflicts, 1)
	assert.Equal(t, "package_not_found", r.Conflicts[0].Reason)
	assert.Equal(t, "1.0.0", r.Entries["a.app"].Version)
}

func TestGraphChangesWithChosenVersion(t *testing.T) {
	// a.app 2.0 depends on x.lib, 1.0 did not. Pinning 1.0 drops x.lib from the lockfile.
	l := loader(map[string][]ReleaseInfo{
		"a.app": {pub("2.0.0", dep("x.lib", "*")), pub("1.0.0")},
		"x.lib": {pub("1.0.0")},
	})
	r, err := Resolve(ctx, []Root{{ID: "a.app"}}, l)
	require.NoError(t, err)
	assert.Contains(t, r.Entries, "x.lib")
	r, err = Resolve(ctx, []Root{{ID: "a.app", Pinned: "1.0.0"}}, l)
	require.NoError(t, err)
	assert.NotContains(t, r.Entries, "x.lib", "dependencies are per version")
}

func TestCycles(t *testing.T) {
	l := loader(map[string][]ReleaseInfo{
		"a.a": {pub("1.0.0", dep("b.b", "^1.0.0"))},
		"b.b": {pub("1.0.0", dep("a.a", "^1.0.0"))},
	})
	r, err := Resolve(ctx, []Root{{ID: "a.a"}}, l)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a.a": "1.0.0", "b.b": "1.0.0"}, versions(r))
}

func TestLoaderErrorsPropagate(t *testing.T) {
	_, err := Resolve(ctx, []Root{{ID: "x.y"}}, func(context.Context, string) (*PackageInfo, error) { return nil, errors.New("db down") })
	assert.EqualError(t, err, "db down")
}
