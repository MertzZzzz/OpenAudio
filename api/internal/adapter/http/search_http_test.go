package http_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// fakeSearch records the normalized queries; SQL behaviour is covered by the Postgres tests.
type fakeSearch struct {
	last       domain.SearchQuery
	lastSample domain.SampleQuery
}

func (f *fakeSearch) Search(_ context.Context, q domain.SearchQuery) ([]domain.SearchHit, int, error) {
	f.last = q
	v := "1.0.0"
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	hit := domain.SearchHit{Score: 0.5, PackageView: domain.PackageView{
		Package:   domain.Package{ID: uuid.New(), PublisherSlug: "acme", PublisherName: "Acme", Slug: "kit", Name: "Kit", Kind: "sample-pack", LatestVersion: &v, Visibility: "public", CreatedAt: now, UpdatedAt: now},
		Downloads: 7, LatestManifest: []byte(`{"taxonomy":{"genres":{"primary":"lo-fi"}},"tags":{"free":["dusty"]}}`),
	}}
	return []domain.SearchHit{hit}, 41, nil
}

func (f *fakeSearch) Facets(_ context.Context, q domain.SearchQuery) (domain.Facets, error) {
	f.last = q
	return domain.Facets{"genre": {{Value: "lo-fi", Count: 3}}, "bpm": {{Value: "80-99", Count: 2}}}, nil
}

func (f *fakeSearch) Samples(_ context.Context, q domain.SampleQuery) ([]domain.SampleHit, int, error) {
	f.lastSample = q
	bpm := 90
	return []domain.SampleHit{{PackageCanon: "acme.kit", PackageName: "Kit", PublisherSlug: "acme", PackageSlug: "kit", Version: "1.0.0",
		Path: "loops/dusty loop.wav", Name: "dusty loop", ContentType: "loop", BPM: &bpm, Key: "F#-minor"}}, 1, nil
}

func (f *fakeSearch) Vocabulary(context.Context) ([]domain.VocabularyTerm, error) {
	return []domain.VocabularyTerm{
		{Version: "2026.11", Domain: "genre", Slug: "hip-hop", Label: "Hip Hop"},
		{Version: "2026.11", Domain: "genre", Slug: "lo-fi", Label: "Lo-Fi", Parent: "hip-hop", Aliases: []string{"lofi"}},
	}, nil
}

func TestSearchEndpoint(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "GET", path: "/api/v1/search?q=lofi&kind=sample-pack,effect&kind=preset-pack&genre=hip-hop&bpmMin=80&bpmMax=100&key=F-minor&commercial=true&page=2&limit=20"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	q := r.search.last
	assert.Equal(t, []string{"sample-pack", "effect", "preset-pack"}, q.Kinds, "comma-separated and repeated values")
	assert.Equal(t, []string{"hip-hop", "lo-fi"}, q.Genres)
	assert.Equal(t, []string{"lo-fi"}, q.AliasSlugs)
	assert.Equal(t, 80, *q.BPMMin)
	assert.True(t, *q.Commercial)
	assert.Equal(t, 20, q.Offset)

	body := decode(t, w)
	assert.EqualValues(t, 41, body["total"])
	assert.EqualValues(t, 2, body["page"])
	item := body["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "acme.kit", item["id"])
	assert.EqualValues(t, 7, item["downloads"])
	assert.EqualValues(t, 0.5, item["score"])
	assert.Equal(t, []any{"dusty"}, item["tags"])

	w = r.do(t, req{method: "GET", path: "/api/v1/search?bpmMin=fast&key=Z-minor"})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "validation_failed", decode(t, w)["code"])
}

func TestFacetsEndpoint(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "GET", path: "/api/v1/search/facets?mood=warm"})
	require.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	for _, k := range []string{"kind", "genre", "instrument", "mood", "useCase", "category", "moduleClass", "key", "license", "bpm"} {
		assert.Contains(t, body, k)
	}
	assert.Equal(t, map[string]any{"value": "lo-fi", "label": "Lo-Fi", "count": float64(3)}, body["genre"].([]any)[0])
	assert.Equal(t, "80–99", body["bpm"].([]any)[0].(map[string]any)["label"])
	assert.Equal(t, []string{"warm"}, r.search.last.Moods)
}

func TestSamplesEndpoint(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "GET", path: "/api/v1/search/samples?q=dusty&contentType=loop&instrument=bass&durationMax=4.5"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "loop", r.search.lastSample.ContentType)
	assert.InDelta(t, 4.5, *r.search.lastSample.DurationMax, 0.001)
	item := decode(t, w)["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "acme.kit/loops/dusty loop.wav", item["id"])
	assert.Equal(t, map[string]any{"tonic": "F#", "mode": "minor"}, item["key"])
	assert.Equal(t, "/api/v1/packages/acme/kit/releases/1.0.0/files/loops/dusty%20loop.wav", item["downloadUrl"])

	assert.Equal(t, http.StatusUnprocessableEntity, r.do(t, req{method: "GET", path: "/api/v1/search/samples?contentType=drone"}).Code)
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/search/samples?package=acme.nope"}).Code)
}

func TestTaxonomyEndpoint(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "GET", path: "/api/v1/taxonomy"})
	require.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.Equal(t, "2026.11", body["taxonomyVersion"])
	terms := body["terms"].([]any)
	assert.Len(t, terms, 2)
	assert.Equal(t, "hip-hop", terms[1].(map[string]any)["parent"])
}
