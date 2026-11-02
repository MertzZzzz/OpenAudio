package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
)

// actor returns the authenticated caller or nil (anonymous).
func actor(r *http.Request) *domain.AccessClaims {
	c, _ := ClaimsFrom(r.Context())
	return c
}

const manifestBodyLimit = manifest.MaxSize + 64<<10

func pathPkg(r *http.Request) (string, string) {
	return chi.URLParam(r, "publisher"), chi.URLParam(r, "slug")
}

/* ------------------------------------------------------------ schema */

func (a *API) manifestSchema(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/schema+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(manifest.SchemaJSON)
}

func (a *API) validateManifest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Manifest  json.RawMessage `json:"manifest"`
		Publisher string          `json:"publisher"`
		Package   string          `json:"package"`
	}
	if err := decodeJSONLimit(r, &in, false, manifestBodyLimit); err != nil {
		a.writeError(w, r, err)
		return
	}
	issues, err := a.pub.ValidateManifest(r.Context(), actor(r), in.Manifest, in.Publisher, in.Package)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	valid := true
	for _, i := range issues {
		if i.Severity == "error" {
			valid = false
		}
	}
	if issues == nil {
		issues = []domain.Issue{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": valid, "issues": issues})
}

/* -------------------------------------------------------- publishers */

func (a *API) createPublisher(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	p, err := a.pub.CreatePublisher(r.Context(), actor(r), publishing.CreatePublisherInput{Slug: in.Slug, Name: in.Name})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPublisherDTO(p, domain.MemberOwner))
}

func (a *API) getPublisher(w http.ResponseWriter, r *http.Request) {
	p, err := a.pub.GetPublisher(r.Context(), chi.URLParam(r, "publisher"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPublisherDTO(p, ""))
}

func (a *API) myPublishers(w http.ResponseWriter, r *http.Request) {
	ms, err := a.pub.MyPublishers(r.Context(), actor(r))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := make([]publisherDTO, len(ms))
	for i := range ms {
		out[i] = toPublisherDTO(&ms[i].Publisher, ms[i].Role)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	ms, err := a.pub.Members(r.Context(), actor(r), chi.URLParam(r, "publisher"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := make([]memberDTO, len(ms))
	for i, m := range ms {
		out[i] = toMemberDTO(m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) addMember(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string            `json:"email"`
		Role  domain.MemberRole `json:"role"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	m, err := a.pub.AddMember(r.Context(), actor(r), chi.URLParam(r, "publisher"), in.Email, in.Role)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMemberDTO(*m))
}

func (a *API) removeMember(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		a.writeError(w, r, domain.NotFound("member_not_found", "this user is not a member"))
		return
	}
	if err := a.pub.RemoveMember(r.Context(), actor(r), chi.URLParam(r, "publisher"), id); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) publisherPackages(w http.ResponseWriter, r *http.Request) {
	list, err := a.pub.PublisherPackages(r.Context(), actor(r), chi.URLParam(r, "publisher"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	type row struct {
		packageDTO
		Releases []releaseDTO `json:"releases"`
	}
	out := make([]row, len(list))
	for i := range list {
		out[i] = row{packageDTO: toPackageDTO(&list[i].PackageView), Releases: toReleaseDTOs(list[i].Releases)}
	}
	writeJSON(w, http.StatusOK, out)
}

/* ---------------------------------------------------------- packages */

func intParam(r *http.Request, key string, def, min, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func (a *API) listPackages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var kinds []string
	for _, k := range q["kind"] {
		for _, part := range strings.Split(k, ",") {
			if part = strings.TrimSpace(part); part != "" {
				kinds = append(kinds, part)
			}
		}
	}
	page := intParam(r, "page", 1, 1, 10000)
	limit := intParam(r, "limit", 24, 1, 100)
	list, total, err := a.pub.ListPackages(r.Context(), domain.PackageFilter{
		Query: q.Get("q"), Kinds: kinds, PublisherSlug: q.Get("publisher"), Sort: domain.PackageSort(q.Get("sort")),
		Limit: limit, Offset: (page - 1) * limit,
	})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	items := make([]packageDTO, len(list))
	for i := range list {
		items[i] = toPackageDTO(&list[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

func (a *API) createPackage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Publisher   string `json:"publisher"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Kind        string `json:"kind"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	v, err := a.pub.CreatePackage(r.Context(), actor(r), in.Publisher, publishing.CreatePackageInput{
		Slug: in.Slug, Name: in.Name, Kind: in.Kind, Description: in.Description, Visibility: in.Visibility})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPackageDTO(v))
}

func (a *API) getPackage(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	v, _, err := a.pub.GetPackage(r.Context(), actor(r), pub, slug)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPackageDTO(v))
}

func (a *API) updatePackage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Visibility  *string `json:"visibility"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	pub, slug := pathPkg(r)
	v, err := a.pub.UpdatePackage(r.Context(), actor(r), pub, slug, publishing.UpdatePackageInput{Name: in.Name, Description: in.Description, Visibility: in.Visibility})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPackageDTO(v))
}

func (a *API) listVersions(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	rels, err := a.pub.ListReleases(r.Context(), actor(r), pub, slug)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toReleaseDTOs(rels))
}

// getManifest serves the canonical manifest bytes; the digest doubles as a strong ETag.
func (a *API) getManifest(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	raw, digest, err := a.pub.Manifest(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	etag := `"` + digest + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Content-Digest", digest)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

/* ---------------------------------------------------------- releases */

func (a *API) writeDraft(w http.ResponseWriter, status int, res *publishing.DraftResult) {
	warnings := res.Warnings
	if warnings == nil {
		warnings = []domain.Issue{}
	}
	writeJSON(w, status, map[string]any{"release": toReleaseDTO(res.Release), "warnings": warnings})
}

func (a *API) createRelease(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version   string          `json:"version"`
		Changelog string          `json:"changelog"`
		Manifest  json.RawMessage `json:"manifest"`
	}
	if err := decodeJSONLimit(r, &in, false, manifestBodyLimit); err != nil {
		a.writeError(w, r, err)
		return
	}
	pub, slug := pathPkg(r)
	res, err := a.pub.CreateDraft(r.Context(), actor(r), pub, slug, publishing.DraftInput{Version: in.Version, Changelog: in.Changelog, Manifest: in.Manifest})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeDraft(w, http.StatusCreated, res)
}

func (a *API) updateRelease(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Changelog *string         `json:"changelog"`
		Manifest  json.RawMessage `json:"manifest"`
	}
	if err := decodeJSONLimit(r, &in, false, manifestBodyLimit); err != nil {
		a.writeError(w, r, err)
		return
	}
	pub, slug := pathPkg(r)
	res, err := a.pub.UpdateDraft(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"), publishing.UpdateDraftInput{Changelog: in.Changelog, Manifest: in.Manifest})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeDraft(w, http.StatusOK, res)
}

func (a *API) deleteRelease(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	if err := a.pub.DeleteDraft(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version")); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getRelease(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	d, err := a.pub.GetRelease(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toReleaseDetails(d))
}

func (a *API) publishRelease(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	rel, err := a.pub.Publish(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toReleaseDTO(rel))
}

func (a *API) yankRelease(w http.ResponseWriter, r *http.Request) {
	pub, slug := pathPkg(r)
	rel, err := a.pub.Yank(r.Context(), actor(r), pub, slug, chi.URLParam(r, "version"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toReleaseDTO(rel))
}
