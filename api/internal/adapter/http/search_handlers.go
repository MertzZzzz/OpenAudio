package http

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/search"
)

/* ------------------------------------------------------- query parsing */

type queryParser struct {
	q      url.Values
	fields []domain.FieldError
}

// list accepts both repeated (?kind=a&kind=b) and comma-separated (?kind=a,b) forms.
func (p *queryParser) list(key string) []string {
	var out []string
	for _, v := range p.q[key] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func (p *queryParser) int(key string) *int {
	v := p.q.Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.fields = append(p.fields, domain.FieldError{Field: key, Message: "must be an integer"})
		return nil
	}
	return &n
}

func (p *queryParser) float(key string) *float64 {
	v := p.q.Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.fields = append(p.fields, domain.FieldError{Field: key, Message: "must be a number"})
		return nil
	}
	return &n
}

func (p *queryParser) bool(key string) *bool {
	v := p.q.Get(key)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.fields = append(p.fields, domain.FieldError{Field: key, Message: "must be true or false"})
		return nil
	}
	return &b
}

func (p *queryParser) intOr(key string, def int) int {
	if v := p.int(key); v != nil {
		return *v
	}
	return def
}

func (p *queryParser) err() error {
	if len(p.fields) == 0 {
		return nil
	}
	return domain.Validation(p.fields...)
}

func searchParams(r *http.Request) (search.Params, error) {
	p := &queryParser{q: r.URL.Query()}
	params := search.Params{
		Q: p.q.Get("q"), Kinds: p.list("kind"), Genres: p.list("genre"), Instruments: p.list("instrument"),
		Moods: p.list("mood"), UseCases: p.list("useCase"), Categories: p.list("category"),
		ModuleClasses: p.list("moduleClass"), Keys: p.list("key"), Licenses: p.list("license"),
		Publisher: p.q.Get("publisher"), BPMMin: p.int("bpmMin"), BPMMax: p.int("bpmMax"),
		EnergyMin: p.float("energyMin"), EnergyMax: p.float("energyMax"), Commercial: p.bool("commercial"),
		Sort: p.q.Get("sort"), Page: p.intOr("page", 1), Limit: p.intOr("limit", 24),
	}
	return params, p.err()
}

/* ------------------------------------------------------------ handlers */

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	params, err := searchParams(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	res, err := a.srch.Search(r.Context(), params)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	type hit struct {
		packageDTO
		Score float64 `json:"score"`
	}
	items := make([]hit, len(res.Hits))
	for i := range res.Hits {
		items[i] = hit{packageDTO: toPackageDTO(&res.Hits[i].PackageView), Score: res.Hits[i].Score}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": res.Total, "page": res.Page, "limit": res.Limit})
}

type facetBucketDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

func (a *API) searchFacets(w http.ResponseWriter, r *http.Request) {
	params, err := searchParams(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	fs, err := a.srch.Facets(r.Context(), params)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := map[string][]facetBucketDTO{}
	for _, name := range []string{domain.FacetKind, domain.FacetGenre, domain.FacetInstrument, domain.FacetMood, domain.FacetUseCase,
		domain.FacetCategory, domain.FacetModuleClass, domain.FacetKey, domain.FacetLicense, domain.FacetBPM} {
		buckets := []facetBucketDTO{}
		for _, b := range fs[name] {
			buckets = append(buckets, facetBucketDTO{Value: b.Value, Label: b.Label, Count: b.Count})
		}
		out[name] = buckets
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, out)
}

type sampleDTO struct {
	ID          string     `json:"id"`
	PackageID   string     `json:"packageId"`
	PackageName string     `json:"packageName"`
	Version     string     `json:"version"`
	Name        string     `json:"name"`
	File        string     `json:"file"`
	Digest      string     `json:"digest"`
	Size        int64      `json:"size"`
	ContentType string     `json:"contentType"`
	Instrument  string     `json:"instrument,omitempty"`
	Role        string     `json:"role,omitempty"`
	BPM         *int       `json:"bpm"`
	Key         *keyDTO    `json:"key"`
	DurationSec *float64   `json:"durationSec"`
	Moods       []string   `json:"moods"`
	Tags        []string   `json:"tags"`
	DownloadURL string     `json:"downloadUrl"`
	PublishedAt *time.Time `json:"publishedAt"`
}

type keyDTO struct {
	Tonic string `json:"tonic"`
	Mode  string `json:"mode"`
}

func (a *API) searchSamples(w http.ResponseWriter, r *http.Request) {
	p := &queryParser{q: r.URL.Query()}
	params := search.SampleParams{
		Q: p.q.Get("q"), Package: p.q.Get("package"), Instruments: p.list("instrument"), ContentType: p.q.Get("contentType"),
		Keys: p.list("key"), BPMMin: p.int("bpmMin"), BPMMax: p.int("bpmMax"), DurationMax: p.float("durationMax"),
		Page: p.intOr("page", 1), Limit: p.intOr("limit", 50),
	}
	if err := p.err(); err != nil {
		a.writeError(w, r, err)
		return
	}
	res, err := a.srch.Samples(r.Context(), params)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	items := make([]sampleDTO, len(res.Hits))
	for i, h := range res.Hits {
		d := sampleDTO{
			ID: h.PackageCanon + "/" + h.Path, PackageID: h.PackageCanon, PackageName: h.PackageName, Version: h.Version,
			Name: h.Name, File: h.Path, Digest: h.Digest, Size: h.SizeBytes, ContentType: h.ContentType, Instrument: h.Instrument,
			Role: h.Role, BPM: h.BPM, DurationSec: h.DurationSec, Moods: h.Moods, Tags: h.Tags, PublishedAt: h.PublishedAt,
			// Downloads go through the API so they are counted.
			DownloadURL: APIPrefix + "/packages/" + url.PathEscape(h.PublisherSlug) + "/" + url.PathEscape(h.PackageSlug) +
				"/releases/" + url.PathEscape(h.Version) + "/files/" + escapePath(h.Path),
		}
		if tonic, mode, ok := strings.Cut(h.Key, "-"); ok {
			d.Key = &keyDTO{Tonic: tonic, Mode: mode}
		}
		if d.Moods == nil {
			d.Moods = []string{}
		}
		if d.Tags == nil {
			d.Tags = []string{}
		}
		items[i] = d
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": res.Total, "page": res.Page, "limit": res.Limit})
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func (a *API) taxonomy(w http.ResponseWriter, r *http.Request) {
	terms, err := a.srch.Vocabulary(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	type termDTO struct {
		Domain  string   `json:"domain"`
		Slug    string   `json:"slug"`
		Label   string   `json:"label"`
		Parent  string   `json:"parent,omitempty"`
		Aliases []string `json:"aliases"`
	}
	version := ""
	out := make([]termDTO, len(terms))
	for i, t := range terms {
		version = t.Version
		out[i] = termDTO{Domain: t.Domain, Slug: t.Slug, Label: t.Label, Parent: t.Parent, Aliases: t.Aliases}
		if out[i].Aliases == nil {
			out[i].Aliases = []string{}
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{"taxonomyVersion": version, "terms": out})
}

// downloadFile redirects to a presigned URL and counts the download; ?preview=1 serves the file
// inline for playback without counting it.
func (a *API) downloadFile(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	preview := r.URL.Query().Get("preview") == "1"
	u, err := a.pub.DownloadURL(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"), chi.URLParam(r, "*"), preview)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

// packagePreview redirects to playable audio of the latest release (used by play buttons).
func (a *API) packagePreview(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	u, err := a.pub.PreviewURL(r.Context(), actor(r), pub, slug)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}
