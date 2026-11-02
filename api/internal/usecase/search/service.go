// Package search implements package and sample search (spec §7, Phase 5): parameter
// validation, controlled-vocabulary expansion (hierarchy + aliases), facets with labels.
package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type Deps struct {
	Repo     domain.SearchRepository
	Packages domain.PackageRepository
	Logger   *slog.Logger
	Now      func() time.Time
	// VocabularyTTL controls how long the in-memory vocabulary is cached.
	VocabularyTTL time.Duration
}

type Service struct {
	Deps
	mu       sync.Mutex
	vocab    *vocabulary
	loadedAt time.Time
}

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.VocabularyTTL == 0 {
		d.VocabularyTTL = 5 * time.Minute
	}
	return &Service{Deps: d}
}

/* ------------------------------------------------------------ vocabulary */

type vocabulary struct {
	terms    []domain.VocabularyTerm
	byKey    map[string]domain.VocabularyTerm // domain:slug
	children map[string][]string              // domain:slug → child slugs
	lookup   map[string][]string              // lowercase alias / slug / label → domain:slug
}

func newVocabulary(terms []domain.VocabularyTerm) *vocabulary {
	v := &vocabulary{terms: terms, byKey: map[string]domain.VocabularyTerm{}, children: map[string][]string{}, lookup: map[string][]string{}}
	for _, t := range terms {
		k := t.Domain + ":" + t.Slug
		v.byKey[k] = t
		if t.Parent != "" {
			pk := t.Domain + ":" + t.Parent
			v.children[pk] = append(v.children[pk], t.Slug)
		}
		names := append([]string{t.Slug, t.Label}, t.Aliases...)
		for _, n := range names {
			n = strings.ToLower(strings.TrimSpace(n))
			if n != "" {
				v.lookup[n] = append(v.lookup[n], k)
			}
		}
	}
	return v
}

// expand returns the slugs plus all descendants; aliases ("dnb") are resolved to slugs.
func (v *vocabulary) expand(dom string, slugs []string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(s string)
	walk = func(s string) {
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
		for _, c := range v.children[dom+":"+s] {
			walk(c)
		}
	}
	for _, s := range slugs {
		s = strings.ToLower(strings.TrimSpace(s))
		resolved := false
		for _, k := range v.lookup[s] {
			if strings.HasPrefix(k, dom+":") {
				walk(strings.TrimPrefix(k, dom+":"))
				resolved = true
			}
		}
		if !resolved && s != "" {
			walk(s)
		}
	}
	return out
}

// textSlugs finds vocabulary terms named by the query (whole query or single words), e.g.
// "dnb" → drum-and-bass, "house" → house + deep-house + …
func (v *vocabulary) textSlugs(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	candidates := append([]string{text}, strings.Fields(text)...)
	seen := map[string]bool{}
	var out []string
	for _, c := range candidates {
		for _, k := range v.lookup[c] {
			dom, slug, _ := strings.Cut(k, ":")
			for _, s := range v.expand(dom, []string{slug}) {
				if !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func (v *vocabulary) label(dom, slug string) string {
	if t, ok := v.byKey[dom+":"+slug]; ok {
		return t.Label
	}
	return humanize(slug)
}

func (s *Service) vocabulary(ctx context.Context) (*vocabulary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vocab != nil && s.Now().Sub(s.loadedAt) < s.VocabularyTTL {
		return s.vocab, nil
	}
	terms, err := s.Repo.Vocabulary(ctx)
	if err != nil {
		if s.vocab != nil {
			s.Logger.WarnContext(ctx, "vocabulary refresh failed; using cached copy", "err", err)
			return s.vocab, nil
		}
		return nil, domain.Internal(err)
	}
	s.vocab, s.loadedAt = newVocabulary(terms), s.Now()
	return s.vocab, nil
}

func (s *Service) Vocabulary(ctx context.Context) ([]domain.VocabularyTerm, error) {
	v, err := s.vocabulary(ctx)
	if err != nil {
		return nil, err
	}
	return v.terms, nil
}

/* ---------------------------------------------------------------- params */

// Params are raw search parameters (as received over HTTP).
type Params struct {
	Q                                                       string
	Kinds, Genres, Instruments, Moods, UseCases, Categories []string
	ModuleClasses, Keys, Licenses                           []string
	Publisher                                               string
	BPMMin, BPMMax                                          *int
	EnergyMin, EnergyMax                                    *float64
	Commercial                                              *bool
	Sort                                                    string
	Page, Limit                                             int
}

var (
	keyRe   = regexp.MustCompile(`^(C|C#|D|D#|E|F|F#|G|G#|A|A#|B)-(major|minor)$`)
	slugRe  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	kinds   = map[string]bool{}
	classes = map[string]bool{"instrument": true, "audio-effect": true, "note-effect": true, "note-generator": true, "audio-generator": true,
		"analyzer": true, "utility": true, "converter": true, "macro": true, "visualizer": true}
)

func init() {
	for _, k := range domain.PackageKinds {
		kinds[k] = true
	}
}

const (
	maxLimit = 100
	maxPage  = 1000
)

func (s *Service) normalize(ctx context.Context, p Params) (domain.SearchQuery, error) {
	var fields []domain.FieldError
	bad := func(field, msg string) { fields = append(fields, domain.FieldError{Field: field, Message: msg}) }

	if len(p.Q) > 200 {
		bad("q", "query is too long (max 200 characters)")
	}
	for _, k := range p.Kinds {
		if !kinds[k] {
			bad("kind", fmt.Sprintf("unknown kind %q", k))
		}
	}
	for _, c := range p.ModuleClasses {
		if !classes[c] {
			bad("moduleClass", fmt.Sprintf("unknown module class %q", c))
		}
	}
	for _, k := range p.Keys {
		if !keyRe.MatchString(k) {
			bad("key", fmt.Sprintf("key must look like F-minor or C#-major, got %q", k))
		}
	}
	for field, list := range map[string][]string{"genre": p.Genres, "instrument": p.Instruments, "mood": p.Moods, "useCase": p.UseCases, "category": p.Categories} {
		for _, v := range list {
			if !slugRe.MatchString(strings.ToLower(v)) {
				bad(field, fmt.Sprintf("invalid value %q", v))
			}
		}
	}
	for field, v := range map[string]*int{"bpmMin": p.BPMMin, "bpmMax": p.BPMMax} {
		if v != nil && (*v < 20 || *v > 400) {
			bad(field, "BPM must be between 20 and 400")
		}
	}
	if p.BPMMin != nil && p.BPMMax != nil && *p.BPMMin > *p.BPMMax {
		bad("bpmMax", "must be ≥ bpmMin")
	}
	for field, v := range map[string]*float64{"energyMin": p.EnergyMin, "energyMax": p.EnergyMax} {
		if v != nil && (*v < 0 || *v > 1) {
			bad(field, "energy must be between 0 and 1")
		}
	}
	sort := domain.SearchSort(p.Sort)
	switch sort {
	case "":
		sort = domain.SearchNewest
		if strings.TrimSpace(p.Q) != "" {
			sort = domain.SearchRelevance
		}
	case domain.SearchRelevance, domain.SearchNewest, domain.SearchDownloads, domain.SearchName:
	case "rating":
		sort = domain.SearchDownloads // ratings do not exist yet
	default:
		bad("sort", "sort must be relevance, newest, downloads or name")
	}
	if len(fields) > 0 {
		return domain.SearchQuery{}, domain.Validation(fields...)
	}

	v, err := s.vocabulary(ctx)
	if err != nil {
		return domain.SearchQuery{}, err
	}
	page, limit := clampPage(p.Page, p.Limit)
	return domain.SearchQuery{
		Text:        strings.TrimSpace(p.Q),
		AliasSlugs:  v.textSlugs(p.Q),
		Kinds:       p.Kinds,
		Publisher:   strings.ToLower(strings.TrimSpace(p.Publisher)),
		Genres:      v.expand("genre", p.Genres),
		Instruments: v.expand("instrument", p.Instruments),
		Moods:       v.expand("mood", p.Moods),
		UseCases:    v.expand("use-case", p.UseCases),
		Categories:  v.expand("module-category", p.Categories),
		ModuleClass: p.ModuleClasses,
		Keys:        p.Keys,
		Licenses:    p.Licenses,
		Commercial:  p.Commercial,
		BPMMin:      p.BPMMin,
		BPMMax:      p.BPMMax,
		EnergyMin:   p.EnergyMin,
		EnergyMax:   p.EnergyMax,
		Sort:        sort,
		Limit:       limit,
		Offset:      (page - 1) * limit,
	}, nil
}

func clampPage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if page > maxPage {
		page = maxPage
	}
	if limit < 1 {
		limit = 24
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}

/* ---------------------------------------------------------------- search */

type Result struct {
	Hits  []domain.SearchHit
	Total int
	Page  int
	Limit int
}

func (s *Service) Search(ctx context.Context, p Params) (*Result, error) {
	q, err := s.normalize(ctx, p)
	if err != nil {
		return nil, err
	}
	hits, total, err := s.Repo.Search(ctx, q)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return &Result{Hits: hits, Total: total, Page: q.Offset/q.Limit + 1, Limit: q.Limit}, nil
}

var kindLabels = map[string]string{
	"sample-pack": "Sample pack", "preset-pack": "Preset pack", "instrument": "Instrument", "effect": "Effect",
	"midi-tool": "MIDI tool", "utility": "Utility", "project-template": "Project template", "model": "Model",
}

var facetDomains = map[string]string{
	domain.FacetGenre: "genre", domain.FacetInstrument: "instrument", domain.FacetMood: "mood",
	domain.FacetUseCase: "use-case", domain.FacetCategory: "module-category",
}

// Facets counts values per facet; every facet ignores its own selection.
func (s *Service) Facets(ctx context.Context, p Params) (domain.Facets, error) {
	q, err := s.normalize(ctx, p)
	if err != nil {
		return nil, err
	}
	fs, err := s.Repo.Facets(ctx, q)
	if err != nil {
		return nil, domain.Internal(err)
	}
	v, err := s.vocabulary(ctx)
	if err != nil {
		return nil, err
	}
	for facet, buckets := range fs {
		for i := range buckets {
			b := &buckets[i]
			switch {
			case facetDomains[facet] != "":
				b.Label = v.label(facetDomains[facet], b.Value)
			case facet == domain.FacetKind:
				b.Label = kindLabels[b.Value]
			case facet == domain.FacetKey:
				b.Label = strings.Replace(b.Value, "-", " ", 1)
			case facet == domain.FacetBPM:
				b.Label = strings.Replace(b.Value, "-", "–", 1)
				if b.Value == "0-79" {
					b.Label = "< 80"
				} else if b.Value == "180-400" {
					b.Label = "180+"
				}
			case facet == domain.FacetLicense:
				b.Label = b.Value
			default:
				b.Label = humanize(b.Value)
			}
		}
	}
	return fs, nil
}

/* --------------------------------------------------------------- samples */

type SampleParams struct {
	Q           string
	Package     string // canonical id "<publisher>.<package>"
	Instruments []string
	ContentType string
	Keys        []string
	BPMMin      *int
	BPMMax      *int
	DurationMax *float64
	Page, Limit int
}

type SampleResult struct {
	Hits  []domain.SampleHit
	Total int
	Page  int
	Limit int
}

func (s *Service) Samples(ctx context.Context, p SampleParams) (*SampleResult, error) {
	var fields []domain.FieldError
	if p.ContentType != "" && p.ContentType != "loop" && p.ContentType != "one-shot" {
		fields = append(fields, domain.FieldError{Field: "contentType", Message: "must be loop or one-shot"})
	}
	for _, k := range p.Keys {
		if !keyRe.MatchString(k) {
			fields = append(fields, domain.FieldError{Field: "key", Message: fmt.Sprintf("invalid key %q", k)})
		}
	}
	if p.BPMMin != nil && p.BPMMax != nil && *p.BPMMin > *p.BPMMax {
		fields = append(fields, domain.FieldError{Field: "bpmMax", Message: "must be ≥ bpmMin"})
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields...)
	}
	v, err := s.vocabulary(ctx)
	if err != nil {
		return nil, err
	}
	page, limit := clampPage(p.Page, p.Limit)
	q := domain.SampleQuery{
		Text: strings.TrimSpace(p.Q), Instruments: v.expand("instrument", p.Instruments), ContentType: p.ContentType,
		Keys: p.Keys, BPMMin: p.BPMMin, BPMMax: p.BPMMax, DurationMax: p.DurationMax, Limit: limit, Offset: (page - 1) * limit,
	}
	if p.Package != "" {
		pub, slug, ok := strings.Cut(p.Package, ".")
		if !ok {
			return nil, domain.NotFound("package_not_found", "package not found")
		}
		pkg, err := s.Packages.Get(ctx, pub, slug)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("package_not_found", "package not found")
		}
		if err != nil {
			return nil, domain.Internal(err)
		}
		q.PackageID = &pkg.ID
	}
	hits, total, err := s.Repo.Samples(ctx, q)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return &SampleResult{Hits: hits, Total: total, Page: page, Limit: limit}, nil
}

func humanize(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
