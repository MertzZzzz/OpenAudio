package search_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/adapter/memory"
	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/search"
)

type fakeRepo struct {
	last       domain.SearchQuery
	lastSample domain.SampleQuery
	facets     domain.Facets
	vocabErr   error
	vocabCalls int
}

func (f *fakeRepo) Search(_ context.Context, q domain.SearchQuery) ([]domain.SearchHit, int, error) {
	f.last = q
	return nil, 0, nil
}
func (f *fakeRepo) Facets(_ context.Context, q domain.SearchQuery) (domain.Facets, error) {
	f.last = q
	return f.facets, nil
}
func (f *fakeRepo) Samples(_ context.Context, q domain.SampleQuery) ([]domain.SampleHit, int, error) {
	f.lastSample = q
	return nil, 0, nil
}
func (f *fakeRepo) Vocabulary(context.Context) ([]domain.VocabularyTerm, error) {
	f.vocabCalls++
	if f.vocabErr != nil {
		return nil, f.vocabErr
	}
	return []domain.VocabularyTerm{
		{Domain: "genre", Slug: "electronic", Label: "Electronic"},
		{Domain: "genre", Slug: "house", Label: "House", Parent: "electronic"},
		{Domain: "genre", Slug: "deep-house", Label: "Deep House", Parent: "house"},
		{Domain: "genre", Slug: "drum-and-bass", Label: "Drum & Bass", Parent: "electronic", Aliases: []string{"dnb", "jungle"}},
		{Domain: "instrument", Slug: "drums", Label: "Drums"},
		{Domain: "instrument", Slug: "kick", Label: "Kick", Parent: "drums"},
		{Domain: "mood", Slug: "warm", Label: "Warm"},
	}, nil
}

var ctx = context.Background()

func newSvc() (*search.Service, *fakeRepo) {
	repo := &fakeRepo{}
	return search.NewService(search.Deps{Repo: repo, Packages: memory.NewCatalog(memory.NewStore()).Packages()}), repo
}

func ip(v int) *int { return &v }

func TestNormalization(t *testing.T) {
	svc, repo := newSvc()
	_, err := svc.Search(ctx, search.Params{Q: " dnb ", Genres: []string{"house"}, Instruments: []string{"drums"}, Page: 3, Limit: 500})
	require.NoError(t, err)
	q := repo.last
	assert.Equal(t, "dnb", q.Text)
	assert.Equal(t, []string{"drum-and-bass"}, q.AliasSlugs, "alias → slug")
	assert.Equal(t, []string{"house", "deep-house"}, q.Genres, "children included")
	assert.Equal(t, []string{"drums", "kick"}, q.Instruments)
	assert.Equal(t, domain.SearchRelevance, q.Sort, "text defaults to relevance")
	assert.Equal(t, 100, q.Limit, "limit clamped")
	assert.Equal(t, 200, q.Offset)

	_, _ = svc.Search(ctx, search.Params{Q: "electronic"})
	assert.ElementsMatch(t, []string{"electronic", "house", "deep-house", "drum-and-bass"}, repo.last.AliasSlugs, "a genre name in the text covers its sub-genres")

	_, _ = svc.Search(ctx, search.Params{Genres: []string{"jungle"}})
	assert.Equal(t, []string{"drum-and-bass"}, repo.last.Genres, "aliases accepted as filter values")

	_, _ = svc.Search(ctx, search.Params{})
	assert.Equal(t, domain.SearchNewest, repo.last.Sort)
	assert.Equal(t, 24, repo.last.Limit)

	_, _ = svc.Search(ctx, search.Params{Sort: "rating"})
	assert.Equal(t, domain.SearchDownloads, repo.last.Sort, "no ratings yet")
}

func TestValidation(t *testing.T) {
	svc, _ := newSvc()
	_, err := svc.Search(ctx, search.Params{
		Kinds: []string{"vst"}, Keys: []string{"H-minor"}, BPMMin: ip(200), BPMMax: ip(100), Sort: "random",
		ModuleClasses: []string{"synth"}, Genres: []string{"Deep House!"},
	})
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	fields := map[string]bool{}
	for _, f := range de.Fields {
		fields[f.Field] = true
	}
	for _, f := range []string{"kind", "key", "bpmMax", "sort", "moduleClass", "genre"} {
		assert.True(t, fields[f], f)
	}
	_, err = svc.Samples(ctx, search.SampleParams{ContentType: "drone"})
	require.ErrorAs(t, err, &de)
}

func TestFacetLabels(t *testing.T) {
	svc, repo := newSvc()
	repo.facets = domain.Facets{
		"genre": {{Value: "drum-and-bass", Count: 2}, {Value: "unknown-genre", Count: 1}},
		"kind":  {{Value: "sample-pack", Count: 3}},
		"key":   {{Value: "F#-minor", Count: 1}},
		"bpm":   {{Value: "120-129", Count: 1}, {Value: "180-400", Count: 1}},
	}
	fs, err := svc.Facets(ctx, search.Params{})
	require.NoError(t, err)
	assert.Equal(t, "Drum & Bass", fs["genre"][0].Label)
	assert.Equal(t, "Unknown Genre", fs["genre"][1].Label)
	assert.Equal(t, "Sample pack", fs["kind"][0].Label)
	assert.Equal(t, "F# minor", fs["key"][0].Label)
	assert.Equal(t, "120–129", fs["bpm"][0].Label)
	assert.Equal(t, "180+", fs["bpm"][1].Label)
}

func TestSamplesAndVocabularyCache(t *testing.T) {
	svc, repo := newSvc()
	_, err := svc.Samples(ctx, search.SampleParams{Instruments: []string{"drums"}, ContentType: "loop"})
	require.NoError(t, err)
	assert.Equal(t, []string{"drums", "kick"}, repo.lastSample.Instruments)

	_, err = svc.Samples(ctx, search.SampleParams{Package: "acme.missing"})
	var de *domain.Error
	require.ErrorAs(t, err, &de)
	assert.Equal(t, "package_not_found", de.Code)

	calls := repo.vocabCalls
	_, _ = svc.Vocabulary(ctx)
	assert.Equal(t, calls, repo.vocabCalls, "vocabulary is cached")

	broken := &fakeRepo{vocabErr: errors.New("db down")}
	_, err = search.NewService(search.Deps{Repo: broken}).Search(ctx, search.Params{})
	assert.Error(t, err)
}
