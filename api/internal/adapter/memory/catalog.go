package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/semver"
)

// Catalog holds publishers, packages, releases and uploads. It shares users with a Store
// (for member names/emails). Semantics mirror the Postgres adapter.
type Catalog struct {
	mu         sync.Mutex
	users      *Store
	publishers map[uuid.UUID]domain.Publisher
	members    map[uuid.UUID]map[uuid.UUID]domain.MemberRole // publisher → user → role
	packages   map[uuid.UUID]domain.Package
	releases   map[uuid.UUID]domain.Release
	artifacts  map[uuid.UUID][]domain.Artifact // release → artifacts
	terms      map[uuid.UUID][]domain.TaxonomyTerm
	uploads    map[uuid.UUID]domain.Upload
	parts      map[uuid.UUID]map[int]domain.UploadPart
	downloads  map[uuid.UUID]int64
	libs       map[uuid.UUID]domain.Library
	items      map[uuid.UUID]map[uuid.UUID]libraryRow
	locks      map[uuid.UUID]domain.LockfileRecord
}

func NewCatalog(users *Store) *Catalog {
	return &Catalog{
		users: users, publishers: map[uuid.UUID]domain.Publisher{}, members: map[uuid.UUID]map[uuid.UUID]domain.MemberRole{},
		packages: map[uuid.UUID]domain.Package{}, releases: map[uuid.UUID]domain.Release{}, artifacts: map[uuid.UUID][]domain.Artifact{},
		terms: map[uuid.UUID][]domain.TaxonomyTerm{}, uploads: map[uuid.UUID]domain.Upload{}, parts: map[uuid.UUID]map[int]domain.UploadPart{},
		downloads: map[uuid.UUID]int64{},
		libs:      map[uuid.UUID]domain.Library{}, items: map[uuid.UUID]map[uuid.UUID]libraryRow{}, locks: map[uuid.UUID]domain.LockfileRecord{},
	}
}

func (c *Catalog) Publishers() domain.PublisherRepository { return (*publishers)(c) }
func (c *Catalog) Packages() domain.PackageRepository     { return (*packages)(c) }
func (c *Catalog) Releases() domain.ReleaseRepository     { return (*releases)(c) }
func (c *Catalog) Uploads() domain.UploadRepository       { return (*uploads)(c) }

// Downloads exposes the download counter for assertions.
func (c *Catalog) Downloads(id uuid.UUID) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.downloads[id]
}

// Terms exposes indexed taxonomy rows for assertions.
func (c *Catalog) Terms(releaseID uuid.UUID) []domain.TaxonomyTerm {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]domain.TaxonomyTerm(nil), c.terms[releaseID]...)
}

/* ------------------------------------------------------------ publishers */

type publishers Catalog

func (r *publishers) Create(_ context.Context, p *domain.Publisher, owner uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.publishers {
		if strings.EqualFold(x.Slug, p.Slug) {
			return domain.ErrConflict
		}
	}
	cp := *p
	cp.OwnerUserID = &owner
	r.publishers[p.ID] = cp
	r.members[p.ID] = map[uuid.UUID]domain.MemberRole{owner: domain.MemberOwner}
	return nil
}

func (r *publishers) GetBySlug(_ context.Context, slug string) (*domain.Publisher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.publishers {
		if strings.EqualFold(p.Slug, slug) {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *publishers) ListForUser(_ context.Context, userID uuid.UUID) ([]domain.PublisherMembership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.PublisherMembership{}
	for pid, m := range r.members {
		if role, ok := m[userID]; ok {
			out = append(out, domain.PublisherMembership{Publisher: r.publishers[pid], Role: role})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Publisher.Slug < out[j].Publisher.Slug })
	return out, nil
}

func (r *publishers) MemberRole(_ context.Context, pid, uid uuid.UUID) (domain.MemberRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if role, ok := r.members[pid][uid]; ok {
		return role, nil
	}
	return "", domain.ErrNotFound
}

func (r *publishers) Members(ctx context.Context, pid uuid.UUID) ([]domain.Member, error) {
	r.mu.Lock()
	roles := map[uuid.UUID]domain.MemberRole{}
	for k, v := range r.members[pid] {
		roles[k] = v
	}
	r.mu.Unlock()
	out := []domain.Member{}
	for uid, role := range roles {
		u, err := r.users.Users().GetByID(ctx, uid)
		if err != nil {
			continue
		}
		out = append(out, domain.Member{UserID: uid, DisplayName: u.DisplayName, Email: u.Email, Role: role})
	}
	order := map[domain.MemberRole]int{domain.MemberOwner: 0, domain.MemberMaintainer: 1, domain.MemberUploader: 2}
	sort.Slice(out, func(i, j int) bool {
		if order[out[i].Role] != order[out[j].Role] {
			return order[out[i].Role] < order[out[j].Role]
		}
		return out[i].DisplayName < out[j].DisplayName
	})
	return out, nil
}

func (r *publishers) AddMember(_ context.Context, pid, uid uuid.UUID, role domain.MemberRole) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.members[pid][uid]; ok {
		return domain.ErrConflict
	}
	if r.members[pid] == nil {
		r.members[pid] = map[uuid.UUID]domain.MemberRole{}
	}
	r.members[pid][uid] = role
	return nil
}

func (r *publishers) RemoveMember(_ context.Context, pid, uid uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.members[pid][uid]; !ok {
		return domain.ErrNotFound
	}
	delete(r.members[pid], uid)
	return nil
}

/* -------------------------------------------------------------- packages */

type packages Catalog

func (r *packages) Create(_ context.Context, p *domain.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.packages {
		if x.PublisherID == p.PublisherID && strings.EqualFold(x.Slug, p.Slug) {
			return domain.ErrConflict
		}
	}
	r.packages[p.ID] = *p
	return nil
}

// view must be called with the lock held.
func (r *packages) view(p domain.Package) domain.PackageView {
	pub := r.publishers[p.PublisherID]
	p.PublisherSlug, p.PublisherName = pub.Slug, pub.Name
	v := domain.PackageView{Package: p, Downloads: r.downloads[p.ID]}
	if p.LatestVersion == nil {
		return v
	}
	for _, rel := range r.releases {
		if rel.PackageID == p.ID && rel.Version == *p.LatestVersion {
			v.LatestManifest, v.LatestPublishedAt = rel.Manifest, rel.PublishedAt
			for _, a := range r.artifacts[rel.ID] {
				v.ArtifactCount++
				v.TotalSize += a.SizeBytes
				if a.Kind == "sample" {
					v.SampleCount++
				}
			}
		}
	}
	return v
}

func (r *packages) Get(_ context.Context, publisherSlug, slug string) (*domain.PackageView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.packages {
		if strings.EqualFold(r.publishers[p.PublisherID].Slug, publisherSlug) && strings.EqualFold(p.Slug, slug) {
			v := r.view(p)
			return &v, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *packages) List(_ context.Context, f domain.PackageFilter) ([]domain.PackageView, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := strings.ToLower(strings.TrimSpace(f.Query))
	var all []domain.PackageView
	for _, p := range r.packages {
		v := r.view(p)
		if q != "" && !strings.Contains(strings.ToLower(v.Name+" "+v.Slug+" "+v.Description+" "+v.PublisherName), q) {
			continue
		}
		if len(f.Kinds) > 0 && !contains(f.Kinds, v.Kind) {
			continue
		}
		if f.PublisherSlug != "" && !strings.EqualFold(v.PublisherSlug, f.PublisherSlug) {
			continue
		}
		if !f.IncludeUnpublished && (v.LatestVersion == nil || v.Visibility != "public") {
			continue
		}
		all = append(all, v)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		switch f.Sort {
		case domain.SortName:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case domain.SortCreated:
			return a.CreatedAt.After(b.CreatedAt)
		}
		ta, tb := a.UpdatedAt, b.UpdatedAt
		if a.LatestPublishedAt != nil {
			ta = *a.LatestPublishedAt
		}
		if b.LatestPublishedAt != nil {
			tb = *b.LatestPublishedAt
		}
		return ta.After(tb)
	})
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 24
	}
	total := len(all)
	if f.Offset > total {
		f.Offset = total
	}
	end := f.Offset + f.Limit
	if end > total {
		end = total
	}
	return all[f.Offset:end], total, nil
}

// CountDownload increments the in-memory download counter.
func (r *packages) CountDownload(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.packages[id]; !ok {
		return domain.ErrNotFound
	}
	r.downloads[id]++
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (r *packages) Update(_ context.Context, id uuid.UUID, name, description, visibility string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.packages[id]
	if !ok {
		return domain.ErrNotFound
	}
	p.Name, p.Description, p.Visibility, p.UpdatedAt = name, description, visibility, at
	r.packages[id] = p
	return nil
}

/* -------------------------------------------------------------- releases */

type releases Catalog

var errImmutable = domain.Conflict("release_immutable", "only draft releases can be changed; publish a new version instead")

func (r *releases) CreateDraft(_ context.Context, rel *domain.Release) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.releases {
		if x.PackageID == rel.PackageID && x.Version == rel.Version {
			return domain.ErrConflict
		}
	}
	cp := *rel
	cp.Status = domain.ReleaseDraft
	r.releases[rel.ID] = cp
	return nil
}

func (r *releases) Get(_ context.Context, pkg uuid.UUID, version string) (*domain.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.releases {
		if x.PackageID == pkg && x.Version == version {
			return &x, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *releases) List(_ context.Context, pkg uuid.UUID) ([]domain.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Release{}
	for _, x := range r.releases {
		if x.PackageID == pkg {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return semver.Compare(out[i].Version, out[j].Version) > 0 })
	return out, nil
}

func (r *releases) UpdateDraft(_ context.Context, id uuid.UUID, manifest []byte, digest, changelog string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.releases[id]
	if !ok || x.Status != domain.ReleaseDraft {
		return errImmutable
	}
	x.Manifest, x.ManifestDigest, x.Changelog = manifest, digest, changelog
	r.releases[id] = x
	return nil
}

func (r *releases) DeleteDraft(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.releases[id]
	if !ok || x.Status != domain.ReleaseDraft {
		return errImmutable
	}
	delete(r.releases, id)
	return nil
}

// recompute must be called with the lock held.
func (r *releases) recompute(pkg uuid.UUID, at time.Time) {
	var versions []string
	for _, x := range r.releases {
		if x.PackageID == pkg && x.Status == domain.ReleasePublished {
			versions = append(versions, x.Version)
		}
	}
	p := r.packages[pkg]
	if latest := semver.Max(versions); latest != "" {
		p.LatestVersion = &latest
	} else {
		p.LatestVersion = nil
	}
	p.UpdatedAt = at
	r.packages[pkg] = p
}

func (r *releases) Publish(_ context.Context, in domain.PublishInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.releases[in.ReleaseID]
	if !ok {
		return domain.ErrNotFound
	}
	if x.Status != domain.ReleaseDraft {
		return errImmutable
	}
	at, by := in.At, in.PublishedBy
	x.Status, x.PublishedAt, x.PublishedBy = domain.ReleasePublished, &at, &by
	r.releases[x.ID] = x
	arts := make([]domain.Artifact, len(in.Artifacts))
	for i, a := range in.Artifacts {
		a.ReleaseID, a.CreatedAt = x.ID, at
		arts[i] = a
	}
	sort.Slice(arts, func(i, j int) bool { return arts[i].Path < arts[j].Path })
	r.artifacts[x.ID] = arts
	r.terms[x.ID] = append([]domain.TaxonomyTerm(nil), in.Terms...)
	r.recompute(in.PackageID, at)
	return nil
}

func (r *releases) Yank(_ context.Context, releaseID, pkg uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.releases[releaseID]
	if !ok || x.Status != domain.ReleasePublished {
		return domain.Conflict("not_published", "only published releases can be yanked")
	}
	x.Status = domain.ReleaseYanked
	r.releases[releaseID] = x
	r.recompute(pkg, at)
	return nil
}

func (r *releases) Artifacts(_ context.Context, releaseID uuid.UUID) ([]domain.Artifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Artifact{}, r.artifacts[releaseID]...), nil
}

/* --------------------------------------------------------------- uploads */

type uploads Catalog

func (r *uploads) Create(_ context.Context, u *domain.Upload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uploads[u.ID] = *u
	r.parts[u.ID] = map[int]domain.UploadPart{}
	return nil
}

func (r *uploads) Get(_ context.Context, id uuid.UUID) (*domain.Upload, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.uploads[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (r *uploads) PutPart(_ context.Context, id uuid.UUID, p domain.UploadPart) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.parts[id] == nil {
		return domain.ErrNotFound
	}
	r.parts[id][p.Number] = p
	return nil
}

func (r *uploads) Parts(_ context.Context, id uuid.UUID) ([]domain.UploadPart, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.UploadPart{}
	for _, p := range r.parts[id] {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func (r *uploads) Finish(_ context.Context, id uuid.UUID, status domain.UploadStatus, errMsg string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.uploads[id]
	if !ok || u.Status != domain.UploadPending {
		return domain.Conflict("upload_finished", "upload is already completed or failed")
	}
	u.Status, u.Error, u.CompletedAt = status, errMsg, &at
	r.uploads[id] = u
	return nil
}
