package postgres_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/postgres"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
)

type seeder struct {
	t    *testing.T
	pubs *postgres.PublisherRepo
	pkgs *postgres.PackageRepo
	rels *postgres.ReleaseRepo
	user uuid.UUID
	ids  map[string]uuid.UUID
	at   time.Time
}

func (s *seeder) publisher(slug, name string) uuid.UUID {
	p := &domain.Publisher{ID: uuid.New(), Slug: slug, Name: name, CreatedAt: s.at}
	require.NoError(s.t, s.pubs.Create(ctx, p, s.user))
	s.ids[slug] = p.ID
	return p.ID
}

// publish creates a package and publishes one release with the given manifest fragment.
func (s *seeder) publish(pub, slug, name, kind, version string, m map[string]any) uuid.UUID {
	s.t.Helper()
	pkgID, ok := s.ids[pub+"/"+slug]
	if !ok {
		pkgID = uuid.New()
		require.NoError(s.t, s.pkgs.Create(ctx, &domain.Package{ID: pkgID, PublisherID: s.ids[pub], Slug: slug, Name: name, Kind: kind, Visibility: "public", CreatedAt: s.at, UpdatedAt: s.at}))
		s.ids[pub+"/"+slug] = pkgID
	}
	m["schema"], m["id"], m["name"], m["kind"], m["version"] = "openaudio.package/v1", pub+"."+slug, name, kind, version
	m["publisher"] = map[string]any{"id": pub, "name": pub}
	raw, err := json.Marshal(m)
	require.NoError(s.t, err)
	parsed, err := manifest.Parse(raw)
	require.NoError(s.t, err)
	canonical, digest, err := manifest.Canonical(parsed)
	require.NoError(s.t, err)
	typed, err := manifest.Decode(canonical)
	require.NoError(s.t, err)

	relID := uuid.New()
	require.NoError(s.t, s.rels.CreateDraft(ctx, &domain.Release{ID: relID, PackageID: pkgID, Version: version, Manifest: canonical, ManifestDigest: digest, CreatedAt: s.at}))
	var arts []domain.Artifact
	for _, a := range typed.Artifacts {
		key, _ := domain.BlobKey(a.Digest)
		arts = append(arts, domain.Artifact{ID: uuid.New(), Kind: a.Kind, Path: a.Path, Digest: a.Digest, SizeBytes: a.Size, StorageKey: key})
	}
	var terms []domain.TaxonomyTerm
	for _, t := range typed.Terms() {
		terms = append(terms, domain.TaxonomyTerm{Domain: t.Domain, Slug: t.Slug, Weight: t.Weight})
	}
	s.at = s.at.Add(time.Minute) // later publishes are "newer"
	require.NoError(s.t, s.rels.Publish(ctx, domain.PublishInput{ReleaseID: relID, PackageID: pkgID, PublishedBy: s.user, Artifacts: arts, Terms: terms, At: s.at}))
	return pkgID
}

func art(path string, c byte) map[string]any {
	return map[string]any{"kind": "sample", "path": path, "digest": "sha256:" + strings.Repeat(string(c), 64), "size": 100}
}

func tax(primary string, secondary, instruments, moods []string) map[string]any {
	return map[string]any{"genres": map[string]any{"primary": primary, "secondary": secondary}, "instruments": instruments, "moods": moods}
}

func names(hits []domain.SearchHit) []string {
	out := []string{}
	for _, h := range hits {
		out = append(out, h.Slug)
	}
	return out
}

func bucket(fs domain.Facets, facet, value string) int {
	for _, b := range fs[facet] {
		if b.Value == value {
			return b.Count
		}
	}
	return 0
}

func ip(v int) *int { return &v }

func TestSearch(t *testing.T) {
	pool := setup(t)
	users := postgres.NewUserRepo(pool)
	search := postgres.NewSearchRepo(pool)
	u := newUser("seed@example.com")
	require.NoError(t, users.Create(ctx, u))
	s := &seeder{t: t, pubs: postgres.NewPublisherRepo(pool), pkgs: postgres.NewPackageRepo(pool), rels: postgres.NewReleaseRepo(pool), user: u.ID, ids: map[string]uuid.UUID{}, at: time.Now().UTC().Truncate(time.Second)}
	s.publisher("acme", "Acme Audio")
	s.publisher("lattice", "Lattice DSP")

	deep := s.publish("acme", "deep-kit", "Deep House Toolkit", "sample-pack", "1.0.0", map[string]any{
		"description": "Rolling drums recorded through tape",
		"license":     map[string]any{"content": "CC-BY-4.0", "commercialUse": true, "attributionRequired": true},
		"taxonomy":    tax("deep-house", []string{"tech-house"}, []string{"drums", "bass"}, []string{"warm", "hypnotic"}),
		"musical":     map[string]any{"bpm": map[string]any{"value": 124, "range": []int{122, 126}}, "key": map[string]any{"tonic": "F", "mode": "minor"}},
		"artifacts":   []any{art("loops/bass/rolling-bass.wav", 'a'), art("one-shots/kick-01.wav", 'b')},
		"samples":     []any{map[string]any{"id": "Big Kick", "file": "one-shots/kick-01.wav", "instrument": "kick", "durationSec": 0.5, "tags": []string{"punchy"}}},
		"tags":        map[string]any{"free": []string{"analog", "tape"}},
	})
	liquid := s.publish("acme", "liquid", "Liquid Essentials", "sample-pack", "2.0.0", map[string]any{
		"license":   map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy":  tax("drum-and-bass", nil, []string{"drums"}, []string{"uplifting"}),
		"musical":   map[string]any{"bpm": map[string]any{"value": 174}, "key": map[string]any{"tonic": "E", "mode": "minor"}},
		"artifacts": []any{art("loops/breaks/amen.wav", 'c')},
	})
	s.publish("lattice", "plate-verb", "Plate Verb", "effect", "2.3.0", map[string]any{
		"license":   map[string]any{"content": "MIT", "commercialUse": true, "attributionRequired": true},
		"taxonomy":  tax("electronic", nil, nil, nil),
		"module":    map[string]any{"moduleClass": "audio-effect", "categories": []string{"reverb"}, "capabilities": map[string]any{"accepts": []string{"audio"}, "produces": []string{"audio"}, "placement": []string{"insert"}}},
		"artifacts": []any{map[string]any{"kind": "binary", "path": "plate-verb.clap", "digest": "sha256:" + strings.Repeat("d", 64), "size": 10}},
	})
	// Not searchable: private, and unpublished.
	priv := s.publish("acme", "secret", "Secret Kit", "sample-pack", "1.0.0", map[string]any{
		"license":  map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy": tax("techno", nil, nil, nil), "artifacts": []any{art("x.wav", 'e')},
	})
	require.NoError(t, s.pkgs.Update(ctx, priv, "Secret Kit", "", "private", s.at))
	require.NoError(t, s.pkgs.Create(ctx, &domain.Package{ID: uuid.New(), PublisherID: s.ids["acme"], Slug: "draft-only", Name: "Draft Only", Kind: "effect", Visibility: "public", CreatedAt: s.at, UpdatedAt: s.at}))

	q := func(mod func(*domain.SearchQuery)) ([]string, int) {
		sq := domain.SearchQuery{Limit: 20, Sort: domain.SearchNewest}
		if mod != nil {
			mod(&sq)
		}
		hits, total, err := search.Search(ctx, sq)
		require.NoError(t, err)
		return names(hits), total
	}

	t.Run("only public published packages are indexed, newest first", func(t *testing.T) {
		got, total := q(nil)
		assert.Equal(t, 3, total)
		assert.Equal(t, []string{"plate-verb", "liquid", "deep-kit"}, got)
	})

	t.Run("full-text with prefixes, tags, publisher, taxonomy labels", func(t *testing.T) {
		for text, want := range map[string][]string{
			"tape":          {"deep-kit"},
			"deep hous":     {"deep-kit"},
			"acme":          {"liquid", "deep-kit"},
			"reverb":        {"plate-verb"},
			"audio-effect":  {"plate-verb"},
			"Drum & Bass":   {"liquid", "deep-kit"}, // genre label; deep-kit has instruments drums + bass
			"nothing-here!": {},
		} {
			got, _ := q(func(sq *domain.SearchQuery) { sq.Text = text })
			assert.ElementsMatch(t, want, got, text)
		}
		got, _ := q(func(sq *domain.SearchQuery) { sq.Text = "dnb"; sq.AliasSlugs = []string{"drum-and-bass"} })
		assert.Equal(t, []string{"liquid"}, got, "aliases resolve to taxonomy slugs")
	})

	t.Run("relevance ranks name matches first", func(t *testing.T) {
		got, _ := q(func(sq *domain.SearchQuery) { sq.Text = "deep"; sq.Sort = domain.SearchRelevance })
		assert.Equal(t, "deep-kit", got[0])
	})

	t.Run("filters", func(t *testing.T) {
		cases := []struct {
			name string
			mod  func(*domain.SearchQuery)
			want []string
		}{
			{"kind", func(sq *domain.SearchQuery) { sq.Kinds = []string{"effect"} }, []string{"plate-verb"}},
			{"genre (expanded by the use case)", func(sq *domain.SearchQuery) { sq.Genres = []string{"house", "deep-house", "tech-house"} }, []string{"deep-kit"}},
			{"instrument", func(sq *domain.SearchQuery) { sq.Instruments = []string{"drums"} }, []string{"liquid", "deep-kit"}},
			{"mood", func(sq *domain.SearchQuery) { sq.Moods = []string{"warm", "uplifting"} }, []string{"liquid", "deep-kit"}},
			{"category", func(sq *domain.SearchQuery) { sq.Categories = []string{"reverb"} }, []string{"plate-verb"}},
			{"module class", func(sq *domain.SearchQuery) { sq.ModuleClass = []string{"audio-effect"} }, []string{"plate-verb"}},
			{"key", func(sq *domain.SearchQuery) { sq.Keys = []string{"F-minor"} }, []string{"deep-kit"}},
			{"license", func(sq *domain.SearchQuery) { sq.Licenses = []string{"CC0-1.0"} }, []string{"liquid"}},
			{"bpm range overlaps 122–126", func(sq *domain.SearchQuery) { sq.BPMMin, sq.BPMMax = ip(126), ip(130) }, []string{"deep-kit"}},
			{"bpm 170–180", func(sq *domain.SearchQuery) { sq.BPMMin, sq.BPMMax = ip(170), ip(180) }, []string{"liquid"}},
			{"publisher", func(sq *domain.SearchQuery) { sq.Publisher = "lattice" }, []string{"plate-verb"}},
			{"combined", func(sq *domain.SearchQuery) { sq.Instruments = []string{"drums"}; sq.BPMMin = ip(150) }, []string{"liquid"}},
		}
		for _, c := range cases {
			got, _ := q(c.mod)
			assert.Equal(t, c.want, got, c.name)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		hits, total, err := search.Search(ctx, domain.SearchQuery{Limit: 2, Offset: 2, Sort: domain.SearchNewest})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, []string{"deep-kit"}, names(hits))
		require.NotNil(t, hits[0].LatestVersion)
		assert.Equal(t, 2, hits[0].SampleCount)
	})

	t.Run("facets ignore their own filter", func(t *testing.T) {
		fs, err := search.Facets(ctx, domain.SearchQuery{Kinds: []string{"sample-pack"}})
		require.NoError(t, err)
		assert.Equal(t, 2, bucket(fs, "kind", "sample-pack"))
		assert.Equal(t, 1, bucket(fs, "kind", "effect"), "other kinds stay selectable")
		assert.Equal(t, 0, bucket(fs, "genre", "electronic"), "other facets are narrowed")
		assert.Equal(t, 1, bucket(fs, "genre", "deep-house"))
		assert.Equal(t, 2, bucket(fs, "instrument", "drums"))
		assert.Equal(t, 1, bucket(fs, "bpm", "120-129"))
		assert.Equal(t, 1, bucket(fs, "bpm", "160-179"))
		assert.Equal(t, 0, bucket(fs, "bpm", "130-139"), "range 122–126 does not reach 130")
		assert.Equal(t, 1, bucket(fs, "key", "E-minor"))
		assert.Equal(t, 1, bucket(fs, "license", "CC-BY-4.0"))
		assert.Empty(t, fs["moduleClass"])

		fs, err = search.Facets(ctx, domain.SearchQuery{BPMMin: ip(170), BPMMax: ip(180)})
		require.NoError(t, err)
		assert.Equal(t, 1, bucket(fs, "bpm", "120-129"), "bpm facet ignores the bpm filter")
		assert.Equal(t, 0, bucket(fs, "genre", "deep-house"))
	})

	t.Run("downloads sort", func(t *testing.T) {
		require.NoError(t, s.pkgs.CountDownload(ctx, liquid))
		require.NoError(t, s.pkgs.CountDownload(ctx, liquid))
		require.NoError(t, s.pkgs.CountDownload(ctx, deep))
		got, _ := q(func(sq *domain.SearchQuery) { sq.Sort = domain.SearchDownloads })
		assert.Equal(t, []string{"liquid", "deep-kit", "plate-verb"}, got)
		hits, _, _ := search.Search(ctx, domain.SearchQuery{Limit: 1, Sort: domain.SearchDownloads})
		assert.EqualValues(t, 2, hits[0].Downloads)
		v, _ := s.pkgs.Get(ctx, "acme", "liquid")
		assert.EqualValues(t, 2, v.Downloads)
	})

	t.Run("samples inherit package metadata", func(t *testing.T) {
		all, total, err := search.Samples(ctx, domain.SampleQuery{Limit: 50})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		byPath := map[string]domain.SampleHit{}
		for _, h := range all {
			byPath[h.Path] = h
		}
		kick := byPath["one-shots/kick-01.wav"]
		assert.Equal(t, "Big Kick", kick.Name)
		assert.Equal(t, "kick", kick.Instrument, "sample override")
		assert.Equal(t, "one-shot", kick.ContentType)
		assert.Equal(t, "F-minor", kick.Key, "inherited from the package")
		assert.Equal(t, []string{"punchy"}, kick.Tags)
		assert.Equal(t, "acme.deep-kit", kick.PackageCanon)
		bass := byPath["loops/bass/rolling-bass.wav"]
		assert.Equal(t, "loop", bass.ContentType, "loops/ folder")
		assert.Equal(t, "drums", bass.Instrument, "first package instrument")
		require.NotNil(t, bass.BPM)
		assert.Equal(t, 124, *bass.BPM, "loops inherit the package tempo")
		assert.Nil(t, kick.BPM, "one-shots do not")
		assert.Equal(t, []string{"analog", "tape"}, bass.Tags)

		for name, sq := range map[string]domain.SampleQuery{
			"instrument": {Instruments: []string{"kick"}},
			"duration":   {DurationMax: func() *float64 { v := 1.0; return &v }()},
			"text":       {Text: "punchy"},
		} {
			sq.Limit = 10
			hits, _, err := search.Samples(ctx, sq)
			require.NoError(t, err)
			require.Len(t, hits, 1, name)
			assert.Equal(t, "one-shots/kick-01.wav", hits[0].Path, name)
		}
		hits, _, _ := search.Samples(ctx, domain.SampleQuery{BPMMin: ip(120), BPMMax: ip(130), Limit: 10})
		assert.Equal(t, []string{"loops/bass/rolling-bass.wav"}, []string{hits[0].Path}, "BPM filter skips one-shots")
		hits, _, _ = search.Samples(ctx, domain.SampleQuery{BPMMin: ip(170), Limit: 10})
		assert.Equal(t, []string{"loops/breaks/amen.wav"}, []string{hits[0].Path})
		hits, _, _ = search.Samples(ctx, domain.SampleQuery{PackageID: &liquid, ContentType: "loop", Limit: 10})
		assert.Len(t, hits, 1)
	})

	t.Run("index follows visibility, new versions and yanks", func(t *testing.T) {
		require.NoError(t, s.pkgs.Update(ctx, priv, "Secret Kit", "", "public", s.at))
		_, total := q(nil)
		assert.Equal(t, 4, total, "made public → indexed")

		s.publish("acme", "liquid", "Liquid Essentials", "sample-pack", "2.1.0", map[string]any{
			"license":   map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
			"taxonomy":  tax("drum-and-bass", nil, []string{"bass"}, []string{"dark"}),
			"musical":   map[string]any{"bpm": map[string]any{"value": 172}},
			"artifacts": []any{art("loops/reese.wav", 'f')},
		})
		got, _ := q(func(sq *domain.SearchQuery) { sq.Moods = []string{"dark"} })
		assert.Equal(t, []string{"liquid"}, got, "the latest release is indexed")
		got, _ = q(func(sq *domain.SearchQuery) { sq.Moods = []string{"uplifting"} })
		assert.Empty(t, got, "the previous release no longer is")

		r, err := s.rels.Get(ctx, liquid, "2.1.0")
		require.NoError(t, err)
		require.NoError(t, s.rels.Yank(ctx, r.ID, liquid, s.at))
		got, _ = q(func(sq *domain.SearchQuery) { sq.Moods = []string{"uplifting"} })
		assert.Equal(t, []string{"liquid"}, got, "yank falls back to 2.0.0 in the index too")
	})

	t.Run("vocabulary", func(t *testing.T) {
		v, err := search.Vocabulary(ctx)
		require.NoError(t, err)
		assert.Len(t, v, 67)
		for _, term := range v {
			if term.Slug == "drum-and-bass" {
				assert.Equal(t, []string{"dnb", "jungle"}, term.Aliases)
				assert.Equal(t, "electronic", term.Parent)
			}
		}
	})
}
