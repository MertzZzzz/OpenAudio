package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/library"
)

type libraryDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func toLibraryDTO(l *domain.Library) libraryDTO {
	return libraryDTO{ID: l.ID.String(), Name: l.Name, CreatedAt: l.CreatedAt}
}

type libraryItemDTO struct {
	PackageID        string     `json:"packageId"`
	Package          packageDTO `json:"package"`
	PinnedVersion    *string    `json:"pinnedVersion"`
	InstalledVersion *string    `json:"installedVersion"`
	AddedAt          time.Time  `json:"addedAt"`
}

func toItemDTO(it *domain.LibraryItem) libraryItemDTO {
	return libraryItemDTO{PackageID: it.Package.CanonicalID(), Package: toPackageDTO(&it.Package), PinnedVersion: it.PinnedVersion,
		InstalledVersion: it.InstalledVersion, AddedAt: it.AddedAt}
}

func libraryID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "libraryId"))
	if err != nil {
		return uuid.Nil, domain.NotFound("library_not_found", "library not found")
	}
	return id, nil
}

func (a *API) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := a.lib.Libraries(r.Context(), actor(r))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := make([]libraryDTO, len(libs))
	for i := range libs {
		out[i] = toLibraryDTO(&libs[i])
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createLibrary(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	l, err := a.lib.CreateLibrary(r.Context(), actor(r), in.Name)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLibraryDTO(l))
}

func (a *API) renameLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	l, err := a.lib.RenameLibrary(r.Context(), actor(r), id, in.Name)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLibraryDTO(l))
}

func (a *API) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err == nil {
		err = a.lib.DeleteLibrary(r.Context(), actor(r), id)
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	items, err := a.lib.Items(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := make([]libraryItemDTO, len(items))
	for i := range items {
		out[i] = toItemDTO(&items[i])
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) addItem(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var in struct {
		PackageID     string  `json:"packageId"`
		PinnedVersion *string `json:"pinnedVersion"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	it, err := a.lib.AddItem(r.Context(), actor(r), id, in.PackageID, in.PinnedVersion)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemDTO(it))
}

// updateItem: {"pinnedVersion": "1.2.0"} pins, {"pinnedVersion": null} unpins, {"upgrade": true} moves to latest.
func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var in struct {
		PinnedVersion json.RawMessage `json:"pinnedVersion"`
		Upgrade       bool            `json:"upgrade"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	upd := library.UpdateItemInput{Upgrade: in.Upgrade}
	if len(in.PinnedVersion) > 0 {
		upd.SetPin = true
		if string(in.PinnedVersion) != "null" {
			var v string
			if err := json.Unmarshal(in.PinnedVersion, &v); err != nil {
				a.writeError(w, r, domain.Validation(domain.FieldError{Field: "pinnedVersion", Message: "must be a version string or null"}))
				return
			}
			upd.PinnedVersion = &v
		}
	}
	it, err := a.lib.UpdateItem(r.Context(), actor(r), id, chi.URLParam(r, "packageId"), upd)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemDTO(it))
}

func (a *API) removeItem(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err == nil {
		err = a.lib.RemoveItem(r.Context(), actor(r), id, chi.URLParam(r, "packageId"))
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func issuesOrEmpty(is []domain.Issue) []domain.Issue {
	if is == nil {
		return []domain.Issue{}
	}
	return is
}

func (a *API) resolveLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	res, err := a.lib.Resolve(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lockfile": res.Lockfile, "conflicts": res.Conflicts, "warnings": issuesOrEmpty(res.Warnings)})
}

type lockfileDTO struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	LibraryID     *string         `json:"libraryId"`
	ContentDigest string          `json:"contentDigest"`
	CreatedAt     time.Time       `json:"createdAt"`
	Lockfile      json.RawMessage `json:"lockfile,omitempty"`
}

func toLockfileDTO(l *domain.LockfileRecord) lockfileDTO {
	d := lockfileDTO{ID: l.ID.String(), Name: l.Name, ContentDigest: l.ContentDigest, CreatedAt: l.CreatedAt}
	if l.LibraryID != nil {
		s := l.LibraryID.String()
		d.LibraryID = &s
	}
	if len(l.Content) > 0 {
		d.Lockfile = l.Content
	}
	return d
}

func (a *API) saveLockfile(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	rec, _, err := a.lib.SaveLockfile(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLockfileDTO(rec))
}

func (a *API) importLockfile(w http.ResponseWriter, r *http.Request) {
	id, err := libraryID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var in struct {
		Lockfile json.RawMessage `json:"lockfile"`
	}
	if err := decodeJSONLimit(r, &in, false, 8<<20); err != nil {
		a.writeError(w, r, err)
		return
	}
	res, err := a.lib.Import(r.Context(), actor(r), id, in.Lockfile)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pinned": res.Pinned, "warnings": issuesOrEmpty(res.Warnings)})
}

func lockfileID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "lockfileId"))
	if err != nil {
		return uuid.Nil, domain.NotFound("lockfile_not_found", "lockfile not found")
	}
	return id, nil
}

func (a *API) listLockfiles(w http.ResponseWriter, r *http.Request) {
	list, err := a.lib.ListLockfiles(r.Context(), actor(r))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := make([]lockfileDTO, len(list))
	for i := range list {
		out[i] = toLockfileDTO(&list[i])
	}
	writeJSON(w, http.StatusOK, out)
}

// getLockfile serves the exact canonical bytes as a downloadable openaudio.lock.json.
func (a *API) getLockfile(w http.ResponseWriter, r *http.Request) {
	id, err := lockfileID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	l, err := a.lib.Lockfile(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Digest", l.ContentDigest)
	w.Header().Set("ETag", `"`+l.ContentDigest+`"`)
	w.Header().Set("Content-Disposition", `attachment; filename="openaudio.lock.json"`)
	_, _ = w.Write(l.Content)
}

func (a *API) deleteLockfile(w http.ResponseWriter, r *http.Request) {
	id, err := lockfileID(r)
	if err == nil {
		err = a.lib.DeleteLockfile(r.Context(), actor(r), id)
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
