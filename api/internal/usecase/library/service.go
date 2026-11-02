// Package library implements personal libraries (installed packages and pinned versions),
// dependency resolution and project lockfiles (spec Phase 6).
package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/platform/canonjson"
	"github.com/MertzZzzz/OpenAudio/api/internal/semver"
)

type Deps struct {
	Libraries domain.LibraryRepository
	Lockfiles domain.LockfileRepository
	Packages  domain.PackageRepository
	Releases  domain.ReleaseRepository
	Logger    *slog.Logger
	Now       func() time.Time
}

type Service struct{ Deps }

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d}
}

type Actor = *domain.AccessClaims

const DefaultLibraryName = "My Library"

var (
	errUnauthenticated = domain.Unauthorized("unauthenticated", "sign in required")
	errLibraryMissing  = domain.NotFound("library_not_found", "library not found")
	errPackageMissing  = domain.NotFound("package_not_found", "package not found")
	errItemMissing     = domain.NotFound("item_not_found", "this package is not in the library")
)

func internal(err error) error {
	var de *domain.Error
	if errors.As(err, &de) {
		return err
	}
	return domain.Internal(err)
}

/* -------------------------------------------------------------- libraries */

// Libraries lists the caller's libraries, creating the default one on first use.
func (s *Service) Libraries(ctx context.Context, actor Actor) ([]domain.Library, error) {
	if actor == nil {
		return nil, errUnauthenticated
	}
	libs, err := s.listLibraries(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if len(libs) > 0 {
		return libs, nil
	}
	l := &domain.Library{ID: uuid.New(), UserID: actor.UserID, Name: DefaultLibraryName, CreatedAt: s.Now()}
	if err := s.Deps.Libraries.Create(ctx, l); err != nil && !errors.Is(err, domain.ErrConflict) {
		return nil, internal(err)
	}
	return s.listLibraries(ctx, actor.UserID)
}

func (s *Service) listLibraries(ctx context.Context, userID uuid.UUID) ([]domain.Library, error) {
	libs, err := s.Deps.Libraries.ListForUser(ctx, userID)
	if err != nil {
		return nil, internal(err)
	}
	return libs, nil
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 80 {
		return "", domain.Validation(domain.FieldError{Field: "name", Message: "must be 1–80 characters"})
	}
	return name, nil
}

func (s *Service) CreateLibrary(ctx context.Context, actor Actor, name string) (*domain.Library, error) {
	if actor == nil {
		return nil, errUnauthenticated
	}
	name, err := validName(name)
	if err != nil {
		return nil, err
	}
	l := &domain.Library{ID: uuid.New(), UserID: actor.UserID, Name: name, CreatedAt: s.Now()}
	if err := s.Deps.Libraries.Create(ctx, l); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("library_exists", "you already have a library with this name")
		}
		return nil, internal(err)
	}
	return l, nil
}

// library loads a library owned by the caller; other people's libraries look like 404s.
func (s *Service) library(ctx context.Context, actor Actor, id uuid.UUID) (*domain.Library, error) {
	if actor == nil {
		return nil, errUnauthenticated
	}
	l, err := s.Deps.Libraries.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errLibraryMissing
	}
	if err != nil {
		return nil, internal(err)
	}
	if l.UserID != actor.UserID {
		return nil, errLibraryMissing
	}
	return l, nil
}

func (s *Service) RenameLibrary(ctx context.Context, actor Actor, id uuid.UUID, name string) (*domain.Library, error) {
	l, err := s.library(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if l.Name, err = validName(name); err != nil {
		return nil, err
	}
	if err := s.Deps.Libraries.Rename(ctx, id, l.Name); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("library_exists", "you already have a library with this name")
		}
		return nil, internal(err)
	}
	return l, nil
}

func (s *Service) DeleteLibrary(ctx context.Context, actor Actor, id uuid.UUID) error {
	if _, err := s.library(ctx, actor, id); err != nil {
		return err
	}
	return internalOrNil(s.Deps.Libraries.Delete(ctx, id))
}

func internalOrNil(err error) error {
	if err == nil {
		return nil
	}
	return internal(err)
}

/* ------------------------------------------------------------------ items */

func (s *Service) Items(ctx context.Context, actor Actor, id uuid.UUID) ([]domain.LibraryItem, error) {
	if _, err := s.library(ctx, actor, id); err != nil {
		return nil, err
	}
	items, err := s.Deps.Libraries.Items(ctx, id)
	return items, internalOrNil(err)
}

// packageByID resolves a canonical package id ("<publisher>.<package>"). Private packages are
// treated as missing.
func (s *Service) packageByID(ctx context.Context, id string) (*domain.PackageView, error) {
	pub, slug, ok := strings.Cut(id, ".")
	if !ok || pub == "" || slug == "" {
		return nil, nil
	}
	v, err := s.Packages.Get(ctx, pub, slug)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if v.Visibility == "private" {
		return nil, nil
	}
	return v, nil
}

func (s *Service) releaseStatus(ctx context.Context, pkg *domain.PackageView, version string) (domain.ReleaseStatus, error) {
	r, err := s.Releases.Get(ctx, pkg.ID, version)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return r.Status, nil
}

func (s *Service) checkPin(ctx context.Context, pkg *domain.PackageView, version string) error {
	if !semver.Valid(version) {
		return domain.Validation(domain.FieldError{Field: "pinnedVersion", Message: "must be MAJOR.MINOR.PATCH"})
	}
	st, err := s.releaseStatus(ctx, pkg, version)
	if err != nil {
		return internal(err)
	}
	if st != domain.ReleasePublished {
		return domain.Validation(domain.FieldError{Field: "pinnedVersion", Message: fmt.Sprintf("%s@%s is not a published release", pkg.CanonicalID(), version)})
	}
	return nil
}

// AddItem adds a package (optionally pinned) to a library; adding an existing package updates its pin.
func (s *Service) AddItem(ctx context.Context, actor Actor, libID uuid.UUID, packageID string, pinned *string) (*domain.LibraryItem, error) {
	if _, err := s.library(ctx, actor, libID); err != nil {
		return nil, err
	}
	pkg, err := s.packageByID(ctx, packageID)
	if err != nil {
		return nil, internal(err)
	}
	if pkg == nil {
		return nil, errPackageMissing
	}
	if pkg.LatestVersion == nil {
		return nil, domain.Validation(domain.FieldError{Field: "packageId", Message: "this package has no published release yet"})
	}
	installed := *pkg.LatestVersion
	if pinned != nil {
		if err := s.checkPin(ctx, pkg, *pinned); err != nil {
			return nil, err
		}
		installed = *pinned
	}
	if err := s.Deps.Libraries.PutItem(ctx, libID, pkg.ID, pinned, &installed, s.Now()); err != nil {
		return nil, internal(err)
	}
	return s.item(ctx, libID, pkg.ID)
}

func (s *Service) item(ctx context.Context, libID, pkgID uuid.UUID) (*domain.LibraryItem, error) {
	items, err := s.Deps.Libraries.Items(ctx, libID)
	if err != nil {
		return nil, internal(err)
	}
	for i := range items {
		if items[i].Package.ID == pkgID {
			return &items[i], nil
		}
	}
	return nil, errItemMissing
}

func (s *Service) findItem(ctx context.Context, libID uuid.UUID, packageID string) (*domain.LibraryItem, error) {
	items, err := s.Deps.Libraries.Items(ctx, libID)
	if err != nil {
		return nil, internal(err)
	}
	for i := range items {
		if items[i].Package.CanonicalID() == packageID {
			return &items[i], nil
		}
	}
	return nil, errItemMissing
}

type UpdateItemInput struct {
	// SetPin changes the pin: PinnedVersion nil unpins.
	SetPin        bool
	PinnedVersion *string
	// Upgrade moves an unpinned item to the latest published version.
	Upgrade bool
}

func (s *Service) UpdateItem(ctx context.Context, actor Actor, libID uuid.UUID, packageID string, in UpdateItemInput) (*domain.LibraryItem, error) {
	if _, err := s.library(ctx, actor, libID); err != nil {
		return nil, err
	}
	it, err := s.findItem(ctx, libID, packageID)
	if err != nil {
		return nil, err
	}
	pinned, installed := it.PinnedVersion, it.InstalledVersion
	if in.SetPin {
		pinned = in.PinnedVersion
		if pinned != nil {
			if err := s.checkPin(ctx, &it.Package, *pinned); err != nil {
				return nil, err
			}
			installed = pinned
		}
	}
	if in.Upgrade {
		if pinned != nil {
			return nil, domain.Conflict("item_pinned", "unpin the package before upgrading it")
		}
		installed = it.Package.LatestVersion
	}
	if err := s.Deps.Libraries.PutItem(ctx, libID, it.Package.ID, pinned, installed, s.Now()); err != nil {
		return nil, internal(err)
	}
	return s.item(ctx, libID, it.Package.ID)
}

func (s *Service) RemoveItem(ctx context.Context, actor Actor, libID uuid.UUID, packageID string) error {
	if _, err := s.library(ctx, actor, libID); err != nil {
		return err
	}
	it, err := s.findItem(ctx, libID, packageID)
	if err != nil {
		return err
	}
	return internalOrNil(s.Deps.Libraries.RemoveItem(ctx, libID, it.Package.ID))
}

/* ---------------------------------------------------------------- resolve */

func (s *Service) loader() Loader {
	return func(ctx context.Context, id string) (*PackageInfo, error) {
		pkg, err := s.packageByID(ctx, id)
		if err != nil || pkg == nil {
			return nil, err
		}
		rels, err := s.Releases.List(ctx, pkg.ID)
		if err != nil {
			return nil, err
		}
		info := &PackageInfo{ID: id}
		for _, r := range rels {
			if r.Status == domain.ReleaseDraft {
				continue
			}
			ri := ReleaseInfo{Version: r.Version, Status: r.Status, ManifestDigest: r.ManifestDigest}
			if m, err := manifest.Decode(r.Manifest); err == nil {
				for _, d := range m.Deps {
					ri.Dependencies = append(ri.Dependencies, Dependency{ID: d.ID, Range: d.Range})
				}
			}
			info.Releases = append(info.Releases, ri)
		}
		return info, nil
	}
}

// Lockfile is the exported document (lockfileVersion 1).
type Lockfile struct {
	LockfileVersion int                      `json:"lockfileVersion"`
	Name            string                   `json:"name"`
	GeneratedAt     time.Time                `json:"generatedAt"`
	Packages        map[string]LockfileEntry `json:"packages"`
}

type LockfileEntry struct {
	ID             string            `json:"id"`
	Version        string            `json:"version"`
	ManifestDigest string            `json:"manifestDigest"`
	Resolved       string            `json:"resolved"`
	Direct         bool              `json:"direct"`
	Yanked         bool              `json:"yanked,omitempty"`
	Dependencies   map[string]string `json:"dependencies,omitempty"`
}

type ResolveResult struct {
	Lockfile  Lockfile
	Conflicts []Conflict
	Warnings  []domain.Issue
}

func (s *Service) Resolve(ctx context.Context, actor Actor, libID uuid.UUID) (*ResolveResult, error) {
	l, err := s.library(ctx, actor, libID)
	if err != nil {
		return nil, err
	}
	items, err := s.Deps.Libraries.Items(ctx, libID)
	if err != nil {
		return nil, internal(err)
	}
	roots := make([]Root, len(items))
	for i, it := range items {
		roots[i] = Root{ID: it.Package.CanonicalID(), Pinned: deref(it.PinnedVersion), Installed: deref(it.InstalledVersion)}
	}
	res, err := Resolve(ctx, roots, s.loader())
	if err != nil {
		return nil, internal(err)
	}
	lf := Lockfile{LockfileVersion: 1, Name: l.Name, GeneratedAt: s.Now().UTC().Truncate(time.Second), Packages: map[string]LockfileEntry{}}
	for id, e := range res.Entries {
		pub, slug, _ := strings.Cut(id, ".")
		le := LockfileEntry{ID: id, Version: e.Version, ManifestDigest: e.ManifestDigest, Direct: e.Direct, Yanked: e.Yanked,
			Resolved: fmt.Sprintf("openaudio://%s/%s@%s", pub, slug, e.Version)}
		if len(e.Dependencies) > 0 {
			le.Dependencies = map[string]string{}
			for _, d := range e.Dependencies {
				le.Dependencies[d.ID] = d.Range
			}
		}
		lf.Packages[id] = le
	}
	conflicts := res.Conflicts
	if conflicts == nil {
		conflicts = []Conflict{}
	}
	return &ResolveResult{Lockfile: lf, Conflicts: conflicts, Warnings: res.Warnings}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/* -------------------------------------------------------------- lockfiles */

// SaveLockfile resolves the library and stores the lockfile (canonical JSON + sha256).
// Libraries with conflicts cannot be locked.
func (s *Service) SaveLockfile(ctx context.Context, actor Actor, libID uuid.UUID) (*domain.LockfileRecord, *ResolveResult, error) {
	res, err := s.Resolve(ctx, actor, libID)
	if err != nil {
		return nil, nil, err
	}
	if len(res.Conflicts) > 0 {
		var fields []domain.FieldError
		for _, c := range res.Conflicts {
			fields = append(fields, domain.FieldError{Field: c.PackageID, Message: strings.ReplaceAll(c.Reason, "_", " ")})
		}
		e := domain.Validation(fields...)
		e.Kind, e.Code, e.Message = domain.KindConflict, "resolve_conflicts", "resolve the version conflicts before exporting a lockfile"
		return nil, res, e
	}
	content, digest, err := canonjson.FromValue(res.Lockfile)
	if err != nil {
		return nil, nil, internal(err)
	}
	rec := &domain.LockfileRecord{ID: uuid.New(), OwnerUserID: actor.UserID, LibraryID: &libID, Name: res.Lockfile.Name, Content: content, ContentDigest: digest, CreatedAt: s.Now()}
	if err := s.Lockfiles.Create(ctx, rec); err != nil {
		return nil, nil, internal(err)
	}
	return rec, res, nil
}

func (s *Service) ListLockfiles(ctx context.Context, actor Actor) ([]domain.LockfileRecord, error) {
	if actor == nil {
		return nil, errUnauthenticated
	}
	list, err := s.Lockfiles.ListForUser(ctx, actor.UserID)
	return list, internalOrNil(err)
}

func (s *Service) Lockfile(ctx context.Context, actor Actor, id uuid.UUID) (*domain.LockfileRecord, error) {
	if actor == nil {
		return nil, errUnauthenticated
	}
	l, err := s.Lockfiles.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && l.OwnerUserID != actor.UserID) {
		return nil, domain.NotFound("lockfile_not_found", "lockfile not found")
	}
	return l, internalOrNil(err)
}

func (s *Service) DeleteLockfile(ctx context.Context, actor Actor, id uuid.UUID) error {
	if _, err := s.Lockfile(ctx, actor, id); err != nil {
		return err
	}
	return internalOrNil(s.Lockfiles.Delete(ctx, id))
}

/* ----------------------------------------------------------------- import */

type ImportResult struct {
	Pinned   int
	Warnings []domain.Issue
}

// Import pins the library to the versions of a lockfile. Every entry is verified against the
// registry first — the package and version must exist and the manifest digest must match —
// so a tampered or foreign lockfile is rejected as a whole.
func (s *Service) Import(ctx context.Context, actor Actor, libID uuid.UUID, raw json.RawMessage) (*ImportResult, error) {
	if _, err := s.library(ctx, actor, libID); err != nil {
		return nil, err
	}
	var lf struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			ID             string `json:"id"`
			Version        string `json:"version"`
			ManifestDigest string `json:"manifestDigest"`
			Direct         *bool  `json:"direct"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &lf); err != nil {
		return nil, domain.Validation(domain.FieldError{Field: "lockfile", Message: "not a valid lockfile: " + err.Error()})
	}
	if lf.LockfileVersion != 1 {
		return nil, domain.Validation(domain.FieldError{Field: "lockfile.lockfileVersion", Message: "unsupported lockfile version (expected 1)"})
	}
	if len(lf.Packages) == 0 {
		return nil, domain.Validation(domain.FieldError{Field: "lockfile.packages", Message: "the lockfile lists no packages"})
	}
	keys := make([]string, 0, len(lf.Packages))
	for k := range lf.Packages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type pin struct {
		pkg     *domain.PackageView
		version string
	}
	var pins []pin
	var issues []domain.Issue
	res := &ImportResult{}
	for _, key := range keys {
		e := lf.Packages[key]
		path := "lockfile.packages." + key
		id := e.ID
		if id == "" {
			id = key
		}
		pkg, err := s.packageByID(ctx, id)
		if err != nil {
			return nil, internal(err)
		}
		if pkg == nil {
			issues = append(issues, domain.Issue{Severity: "error", Path: path, Message: fmt.Sprintf("package %s does not exist", id)})
			continue
		}
		r, err := s.Releases.Get(ctx, pkg.ID, e.Version)
		if errors.Is(err, domain.ErrNotFound) || (err == nil && r.Status == domain.ReleaseDraft) {
			issues = append(issues, domain.Issue{Severity: "error", Path: path, Message: fmt.Sprintf("%s@%s does not exist", id, e.Version)})
			continue
		}
		if err != nil {
			return nil, internal(err)
		}
		if r.ManifestDigest != e.ManifestDigest {
			issues = append(issues, domain.Issue{Severity: "error", Path: path + ".manifestDigest",
				Message: fmt.Sprintf("integrity check failed for %s@%s: lockfile has %s, registry has %s", id, e.Version, e.ManifestDigest, r.ManifestDigest)})
			continue
		}
		if r.Status == domain.ReleaseYanked {
			res.Warnings = append(res.Warnings, domain.Issue{Severity: "warning", Path: path, Message: fmt.Sprintf("%s@%s is yanked", id, e.Version)})
		}
		if e.Direct == nil || *e.Direct {
			pins = append(pins, pin{pkg, e.Version})
		}
	}
	if err := domain.IssuesError("lockfile_invalid", "the lockfile does not match the registry", issues); err != nil {
		return nil, err
	}
	now := s.Now()
	for _, p := range pins {
		v := p.version
		if err := s.Deps.Libraries.PutItem(ctx, libID, p.pkg.ID, &v, &v, now); err != nil {
			return nil, internal(err)
		}
		res.Pinned++
	}
	return res, nil
}
