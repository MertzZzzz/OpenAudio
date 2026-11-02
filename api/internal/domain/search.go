package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// VocabularyTerm is an entry of the controlled vocabulary (spec §7.2).
type VocabularyTerm struct {
	Version string
	Domain  string // genre | instrument | mood | use-case | module-category
	Slug    string
	Label   string
	Parent  string
	Aliases []string
}

type SearchSort string

const (
	SearchRelevance SearchSort = "relevance"
	SearchNewest    SearchSort = "newest"
	SearchDownloads SearchSort = "downloads"
	SearchName      SearchSort = "name"
)

// SearchQuery is a normalized package search. Taxonomy slices are already expanded with
// descendants (genre "house" also lists "deep-house"), so repositories match them literally.
type SearchQuery struct {
	Text string
	// AliasSlugs are taxonomy slugs the text resolved to (e.g. "dnb" → drum-and-bass); a package
	// tagged with one of them matches the text even if the words do not appear.
	AliasSlugs []string

	Kinds       []string
	Publisher   string
	Genres      []string
	Instruments []string
	Moods       []string
	UseCases    []string
	Categories  []string
	ModuleClass []string
	Keys        []string // "F-minor"
	Licenses    []string
	Commercial  *bool
	BPMMin      *int
	BPMMax      *int
	EnergyMin   *float64
	EnergyMax   *float64
	Sort        SearchSort
	Limit       int
	Offset      int
}

// SearchHit is a package view with its relevance score and download count.
type SearchHit struct {
	PackageView
	Score float64
}

type FacetBucket struct {
	Value string
	Label string
	Count int
}

// Facet names; each facet is counted with every filter applied except its own.
const (
	FacetKind        = "kind"
	FacetGenre       = "genre"
	FacetInstrument  = "instrument"
	FacetMood        = "mood"
	FacetUseCase     = "useCase"
	FacetCategory    = "category"
	FacetModuleClass = "moduleClass"
	FacetKey         = "key"
	FacetLicense     = "license"
	FacetBPM         = "bpm"
)

type Facets map[string][]FacetBucket

type SampleQuery struct {
	Text        string
	PackageID   *uuid.UUID
	Instruments []string
	ContentType string // loop | one-shot
	Keys        []string
	BPMMin      *int
	BPMMax      *int
	DurationMax *float64
	Limit       int
	Offset      int
}

type SampleHit struct {
	PackageID     uuid.UUID
	PackageCanon  string // "<publisher>.<package>"
	PackageName   string
	PublisherSlug string
	PackageSlug   string
	Version       string
	Path          string
	Name          string
	Digest        string
	SizeBytes     int64
	ContentType   string
	Instrument    string
	Role          string
	BPM           *int
	Key           string
	DurationSec   *float64
	Moods         []string
	Tags          []string
	StorageKey    string
	PublishedAt   *time.Time
}

type SearchRepository interface {
	Search(ctx context.Context, q SearchQuery) ([]SearchHit, int, error)
	Facets(ctx context.Context, q SearchQuery) (Facets, error)
	Samples(ctx context.Context, q SampleQuery) ([]SampleHit, int, error)
	Vocabulary(ctx context.Context) ([]VocabularyTerm, error)
}
