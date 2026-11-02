package publishing

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type CreatePackageInput struct {
	Slug        string
	Name        string
	Kind        string
	Description string
	Visibility  string
}

func validKind(k string) bool {
	for _, x := range domain.PackageKinds {
		if x == k {
			return true
		}
	}
	return false
}

func validVisibility(v string) bool { return v == "public" || v == "unlisted" || v == "private" }

func (s *Service) CreatePackage(ctx context.Context, actor Actor, publisherSlug string, in CreatePackageInput) (*domain.PackageView, error) {
	p, err := s.publisher(ctx, publisherSlug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, p.ID, domain.PermManagePackages); err != nil {
		return nil, err
	}
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if in.Visibility == "" {
		in.Visibility = "public"
	}
	var kindErr, visErr, descErr *domain.FieldError
	if !validKind(in.Kind) {
		kindErr = &domain.FieldError{Field: "kind", Message: "must be one of " + strings.Join(domain.PackageKinds, ", ")}
	}
	if !validVisibility(in.Visibility) {
		visErr = &domain.FieldError{Field: "visibility", Message: "must be public, unlisted or private"}
	}
	if len(in.Description) > 2000 {
		descErr = &domain.FieldError{Field: "description", Message: "must be at most 2000 characters"}
	}
	if err := collect(validSlug("slug", in.Slug, 2, 64), validName("name", in.Name), kindErr, visErr, descErr); err != nil {
		return nil, err
	}
	now := s.Now()
	pkg := &domain.Package{
		ID: uuid.New(), PublisherID: p.ID, PublisherSlug: p.Slug, PublisherName: p.Name, Slug: in.Slug,
		Name: strings.TrimSpace(in.Name), Kind: in.Kind, Description: strings.TrimSpace(in.Description), Visibility: in.Visibility,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Packages.Create(ctx, pkg); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("package_exists", "this publisher already has a package with this slug")
		}
		return nil, internal(err)
	}
	return &domain.PackageView{Package: *pkg}, nil
}

// GetPackage returns a package. Private packages are only visible to members.
func (s *Service) GetPackage(ctx context.Context, actor Actor, publisherSlug, slug string) (*domain.PackageView, domain.MemberRole, error) {
	v, err := s.Packages.Get(ctx, publisherSlug, slug)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, "", errPackageMissing
	}
	if err != nil {
		return nil, "", internal(err)
	}
	role, err := s.role(ctx, actor, v.PublisherID)
	if err != nil {
		return nil, "", err
	}
	if v.Visibility == "private" && role == "" {
		return nil, "", errPackageMissing
	}
	return v, role, nil
}

func (s *Service) ListPackages(ctx context.Context, f domain.PackageFilter) ([]domain.PackageView, int, error) {
	f.IncludeUnpublished = false
	list, total, err := s.Packages.List(ctx, f)
	return list, total, internalOrNil(err)
}

// PackageWithReleases is a dashboard row.
type PackageWithReleases struct {
	domain.PackageView
	Releases []domain.Release
}

// PublisherPackages lists every package of a publisher, including drafts, for its members.
func (s *Service) PublisherPackages(ctx context.Context, actor Actor, publisherSlug string) ([]PackageWithReleases, error) {
	p, err := s.publisher(ctx, publisherSlug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, p.ID, domain.PermDraft); err != nil {
		return nil, err
	}
	list, _, err := s.Packages.List(ctx, domain.PackageFilter{PublisherSlug: p.Slug, IncludeUnpublished: true, Sort: domain.SortName, Limit: 200})
	if err != nil {
		return nil, internal(err)
	}
	out := make([]PackageWithReleases, len(list))
	for i, v := range list {
		rels, err := s.Releases.List(ctx, v.ID)
		if err != nil {
			return nil, internal(err)
		}
		out[i] = PackageWithReleases{PackageView: v, Releases: rels}
	}
	return out, nil
}

type UpdatePackageInput struct {
	Name        *string
	Description *string
	Visibility  *string
}

func (s *Service) UpdatePackage(ctx context.Context, actor Actor, publisherSlug, slug string, in UpdatePackageInput) (*domain.PackageView, error) {
	v, _, err := s.GetPackage(ctx, actor, publisherSlug, slug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, v.PublisherID, domain.PermManagePackages); err != nil {
		return nil, err
	}
	name, desc, vis := v.Name, v.Description, v.Visibility
	var errs []*domain.FieldError
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		errs = append(errs, validName("name", name))
	}
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
		if len(desc) > 2000 {
			errs = append(errs, &domain.FieldError{Field: "description", Message: "must be at most 2000 characters"})
		}
	}
	if in.Visibility != nil {
		vis = *in.Visibility
		if !validVisibility(vis) {
			errs = append(errs, &domain.FieldError{Field: "visibility", Message: "must be public, unlisted or private"})
		}
	}
	if err := collect(errs...); err != nil {
		return nil, err
	}
	if err := s.Packages.Update(ctx, v.ID, name, desc, vis, s.Now()); err != nil {
		return nil, internal(err)
	}
	v.Name, v.Description, v.Visibility = name, desc, vis
	return v, nil
}
