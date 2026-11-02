package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
	"github.com/MertzZzzz/OpenAudio/api/internal/semver"
)

/* ----------------------------------------------------------- validation */

// prepared is a normalized, validated manifest ready to store.
type prepared struct {
	canonical []byte
	digest    string
	version   string
	issues    []domain.Issue
}

func target(v *domain.PackageView, version string) manifest.Target {
	return manifest.Target{
		PackageID: v.CanonicalID(), PublisherID: v.PublisherSlug, PublisherName: v.PublisherName,
		PackageName: v.Name, Kind: v.Kind, Version: version,
	}
}

func (s *Service) prepare(raw json.RawMessage, v *domain.PackageView, version string) (*prepared, error) {
	if len(raw) == 0 {
		return nil, domain.Validation(domain.FieldError{Field: "manifest", Message: "manifest is required"})
	}
	if len(raw) > manifest.MaxSize {
		return nil, domain.Validation(domain.FieldError{Field: "manifest", Message: "manifest is larger than 4 MiB"})
	}
	m, err := manifest.Parse(raw)
	if err != nil {
		return nil, domain.Validation(domain.FieldError{Field: "manifest", Message: err.Error()})
	}
	if version == "" {
		version, _ = m["version"].(string)
	}
	if !semver.Valid(version) {
		return nil, domain.Validation(domain.FieldError{Field: "version", Message: "version must be MAJOR.MINOR.PATCH, e.g. 1.0.0"})
	}
	issues := manifest.Normalize(m, target(v, version))
	if len(issues) == 0 {
		issues = s.Validator.Validate(m)
	}
	if err := domain.IssuesError("manifest_invalid", "the manifest has errors", prefix(issues)); err != nil {
		return nil, err
	}
	canonical, digest, err := manifest.Canonical(m)
	if err != nil {
		return nil, internal(err)
	}
	return &prepared{canonical: canonical, digest: digest, version: version, issues: issues}, nil
}

// prefix scopes manifest issue paths under "manifest." for request-level error reporting.
func prefix(issues []domain.Issue) []domain.Issue {
	out := make([]domain.Issue, len(issues))
	for i, is := range issues {
		is.Path = "manifest." + strings.TrimPrefix(is.Path, "(root)")
		is.Path = strings.TrimSuffix(is.Path, ".")
		out[i] = is
	}
	return out
}

// ValidateManifest checks a manifest without storing it. With a package it is normalized
// against that package first (as a draft would be).
func (s *Service) ValidateManifest(ctx context.Context, actor Actor, raw json.RawMessage, publisherSlug, slug string) ([]domain.Issue, error) {
	m, err := manifest.Parse(raw)
	if err != nil {
		return []domain.Issue{{Severity: "error", Path: "(root)", Message: err.Error()}}, nil
	}
	if publisherSlug != "" {
		v, _, err := s.GetPackage(ctx, actor, publisherSlug, slug)
		if err != nil {
			return nil, err
		}
		version, _ := m["version"].(string)
		if issues := manifest.Normalize(m, target(v, version)); len(issues) > 0 {
			return issues, nil
		}
	}
	return s.Validator.Validate(m), nil
}

/* --------------------------------------------------------------- drafts */

type DraftInput struct {
	Version   string
	Changelog string
	Manifest  json.RawMessage
}

// DraftResult carries non-blocking warnings along with the stored draft.
type DraftResult struct {
	Release  *domain.Release
	Warnings []domain.Issue
}

func (s *Service) packageFor(ctx context.Context, actor Actor, publisherSlug, slug string, perm domain.Permission) (*domain.PackageView, error) {
	v, _, err := s.GetPackage(ctx, actor, publisherSlug, slug)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, actor, v.PublisherID, perm); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) CreateDraft(ctx context.Context, actor Actor, publisherSlug, slug string, in DraftInput) (*DraftResult, error) {
	v, err := s.packageFor(ctx, actor, publisherSlug, slug, domain.PermDraft)
	if err != nil {
		return nil, err
	}
	if len(in.Changelog) > 5000 {
		return nil, domain.Validation(domain.FieldError{Field: "changelog", Message: "must be at most 5000 characters"})
	}
	p, err := s.prepare(in.Manifest, v, in.Version)
	if err != nil {
		return nil, err
	}
	by := actor.UserID
	r := &domain.Release{
		ID: uuid.New(), PackageID: v.ID, Version: p.version, Manifest: p.canonical, ManifestDigest: p.digest,
		Changelog: strings.TrimSpace(in.Changelog), Status: domain.ReleaseDraft, CreatedBy: &by, CreatedAt: s.Now(),
	}
	if err := s.Releases.CreateDraft(ctx, r); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, domain.Conflict("version_exists", fmt.Sprintf("version %s already exists — releases are immutable, pick a new version", p.version))
		}
		return nil, internal(err)
	}
	return &DraftResult{Release: r, Warnings: p.issues}, nil
}

type UpdateDraftInput struct {
	Changelog *string
	Manifest  json.RawMessage
}

func (s *Service) UpdateDraft(ctx context.Context, actor Actor, publisherSlug, slug, version string, in UpdateDraftInput) (*DraftResult, error) {
	v, err := s.packageFor(ctx, actor, publisherSlug, slug, domain.PermDraft)
	if err != nil {
		return nil, err
	}
	r, err := s.release(ctx, v.ID, version)
	if err != nil {
		return nil, err
	}
	if r.Status != domain.ReleaseDraft {
		return nil, domain.Conflict("release_immutable", "only draft releases can be changed; publish a new version instead")
	}
	res := &DraftResult{Release: r}
	if len(in.Manifest) > 0 {
		p, err := s.prepare(in.Manifest, v, version)
		if err != nil {
			return nil, err
		}
		r.Manifest, r.ManifestDigest, res.Warnings = p.canonical, p.digest, p.issues
	}
	if in.Changelog != nil {
		r.Changelog = strings.TrimSpace(*in.Changelog)
	}
	if err := s.Releases.UpdateDraft(ctx, r.ID, r.Manifest, r.ManifestDigest, r.Changelog); err != nil {
		return nil, internal(err)
	}
	return res, nil
}

func (s *Service) DeleteDraft(ctx context.Context, actor Actor, publisherSlug, slug, version string) error {
	v, err := s.packageFor(ctx, actor, publisherSlug, slug, domain.PermDraft)
	if err != nil {
		return err
	}
	r, err := s.release(ctx, v.ID, version)
	if err != nil {
		return err
	}
	return internalOrNil(s.Releases.DeleteDraft(ctx, r.ID))
}

func (s *Service) release(ctx context.Context, packageID uuid.UUID, version string) (*domain.Release, error) {
	r, err := s.Releases.Get(ctx, packageID, version)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, errReleaseMissing
	}
	if err != nil {
		return nil, internal(err)
	}
	return r, nil
}

/* ---------------------------------------------------------------- reads */

// ListReleases hides drafts from non-members.
func (s *Service) ListReleases(ctx context.Context, actor Actor, publisherSlug, slug string) ([]domain.Release, error) {
	v, role, err := s.GetPackage(ctx, actor, publisherSlug, slug)
	if err != nil {
		return nil, err
	}
	all, err := s.Releases.List(ctx, v.ID)
	if err != nil {
		return nil, internal(err)
	}
	if role != "" {
		return all, nil
	}
	out := all[:0]
	for _, r := range all {
		if r.Status != domain.ReleaseDraft {
			out = append(out, r)
		}
	}
	return out, nil
}

// ArtifactWithURL adds a short-lived presigned download URL (published releases only).
type ArtifactWithURL struct {
	domain.Artifact
	DownloadURL string
}

type ReleaseDetails struct {
	Package   *domain.PackageView
	Release   *domain.Release
	Artifacts []ArtifactWithURL
}

func (s *Service) visibleRelease(ctx context.Context, actor Actor, publisherSlug, slug, version string) (*domain.PackageView, *domain.Release, error) {
	v, role, err := s.GetPackage(ctx, actor, publisherSlug, slug)
	if err != nil {
		return nil, nil, err
	}
	r, err := s.release(ctx, v.ID, version)
	if err != nil {
		return nil, nil, err
	}
	if r.Status == domain.ReleaseDraft && role == "" {
		return nil, nil, errReleaseMissing
	}
	return v, r, nil
}

func (s *Service) GetRelease(ctx context.Context, actor Actor, publisherSlug, slug, version string) (*ReleaseDetails, error) {
	v, r, err := s.visibleRelease(ctx, actor, publisherSlug, slug, version)
	if err != nil {
		return nil, err
	}
	d := &ReleaseDetails{Package: v, Release: r}
	if r.Status == domain.ReleaseDraft {
		// Drafts have no artifact rows yet; list what the manifest declares.
		m, err := manifest.Decode(r.Manifest)
		if err != nil {
			return nil, internal(err)
		}
		for _, a := range m.Artifacts {
			d.Artifacts = append(d.Artifacts, ArtifactWithURL{Artifact: domain.Artifact{Kind: a.Kind, Path: a.Path, Digest: a.Digest, SizeBytes: a.Size, ContentType: a.ContentType}})
		}
		return d, nil
	}
	arts, err := s.Releases.Artifacts(ctx, r.ID)
	if err != nil {
		return nil, internal(err)
	}
	for _, a := range arts {
		u, err := s.Blobs.PresignGet(ctx, a.StorageKey, s.DownloadTTL, path.Base(a.Path))
		if err != nil {
			return nil, internal(err)
		}
		d.Artifacts = append(d.Artifacts, ArtifactWithURL{Artifact: a, DownloadURL: u})
	}
	return d, nil
}

// Manifest returns the canonical manifest bytes and digest.
func (s *Service) Manifest(ctx context.Context, actor Actor, publisherSlug, slug, version string) ([]byte, string, error) {
	_, r, err := s.visibleRelease(ctx, actor, publisherSlug, slug, version)
	if err != nil {
		return nil, "", err
	}
	return r.Manifest, r.ManifestDigest, nil
}

/* ------------------------------------------------------ publish / yank */

// Publish freezes a draft. Every artifact must already be uploaded (blobs/{sha256}) with the
// declared size; the manifest is re-validated and written to S3 next to the database copy.
func (s *Service) Publish(ctx context.Context, actor Actor, publisherSlug, slug, version string) (*domain.Release, error) {
	v, err := s.packageFor(ctx, actor, publisherSlug, slug, domain.PermPublish)
	if err != nil {
		return nil, err
	}
	r, err := s.release(ctx, v.ID, version)
	if err != nil {
		return nil, err
	}
	if r.Status != domain.ReleaseDraft {
		return nil, domain.Conflict("release_immutable", fmt.Sprintf("release %s is already %s", version, r.Status))
	}
	parsed, err := manifest.Parse(r.Manifest)
	if err != nil {
		return nil, internal(err)
	}
	issues := s.Validator.Validate(parsed)
	m, err := manifest.Decode(r.Manifest)
	if err != nil {
		return nil, internal(err)
	}

	now := s.Now()
	artifacts := make([]domain.Artifact, 0, len(m.Artifacts)+1)
	for i, a := range m.Artifacts {
		key, err := domain.BlobKey(a.Digest)
		if err != nil {
			continue // already reported by schema validation
		}
		size, ok, err := s.Blobs.Stat(ctx, key)
		if err != nil {
			return nil, internal(err)
		}
		p := fmt.Sprintf("artifacts.%d", i)
		switch {
		case !ok:
			issues = append(issues, domain.Issue{Severity: "error", Path: p + ".digest", Message: fmt.Sprintf("%s (%s) has not been uploaded", a.Digest, a.Path)})
		case size != a.Size:
			issues = append(issues, domain.Issue{Severity: "error", Path: p + ".size", Message: fmt.Sprintf("declared %d bytes but the stored blob has %d", a.Size, size)})
		}
		artifacts = append(artifacts, domain.Artifact{ID: uuid.New(), Kind: a.Kind, Path: a.Path, Digest: a.Digest, SizeBytes: a.Size, ContentType: a.ContentType, StorageKey: key})
	}
	if err := domain.IssuesError("release_incomplete", "the release cannot be published yet", prefix(issues)); err != nil {
		return nil, err
	}

	manifestKey := domain.ManifestKey(v.CanonicalID(), r.Version)
	if err := s.Blobs.Put(ctx, manifestKey, r.Manifest, "application/json"); err != nil {
		return nil, internal(fmt.Errorf("store manifest: %w", err))
	}
	artifacts = append(artifacts, domain.Artifact{ID: uuid.New(), Kind: "manifest", Path: "manifest.json", Digest: r.ManifestDigest,
		SizeBytes: int64(len(r.Manifest)), ContentType: "application/json", StorageKey: manifestKey})

	var terms []domain.TaxonomyTerm
	for _, t := range m.Terms() {
		terms = append(terms, domain.TaxonomyTerm{Domain: t.Domain, Slug: t.Slug, Weight: t.Weight})
	}
	if err := s.Releases.Publish(ctx, domain.PublishInput{ReleaseID: r.ID, PackageID: v.ID, PublishedBy: actor.UserID, Artifacts: artifacts, Terms: terms, At: now}); err != nil {
		return nil, internal(err)
	}
	by := actor.UserID
	r.Status, r.PublishedAt, r.PublishedBy = domain.ReleasePublished, &now, &by
	return r, nil
}

func (s *Service) Yank(ctx context.Context, actor Actor, publisherSlug, slug, version string) (*domain.Release, error) {
	v, err := s.packageFor(ctx, actor, publisherSlug, slug, domain.PermPublish)
	if err != nil {
		return nil, err
	}
	r, err := s.release(ctx, v.ID, version)
	if err != nil {
		return nil, err
	}
	if err := s.Releases.Yank(ctx, r.ID, v.ID, s.Now()); err != nil {
		return nil, internal(err)
	}
	r.Status = domain.ReleaseYanked
	return r, nil
}

/* ------------------------------------------------------------- download */

// DownloadURL returns a short-lived presigned URL for one artifact of a published (or yanked)
// release and counts the download. Manifest downloads and previews (inline playback) are not counted.
func (s *Service) DownloadURL(ctx context.Context, actor Actor, publisherSlug, slug, version, artifactPath string, preview bool) (string, error) {
	v, r, err := s.visibleRelease(ctx, actor, publisherSlug, slug, version)
	if err != nil {
		return "", err
	}
	if r.Status == domain.ReleaseDraft {
		return "", domain.NotFound("artifact_not_found", "drafts have no downloadable files yet")
	}
	arts, err := s.Releases.Artifacts(ctx, r.ID)
	if err != nil {
		return "", internal(err)
	}
	for _, a := range arts {
		if a.Path != artifactPath {
			continue
		}
		filename := path.Base(a.Path)
		if preview {
			filename = "" // inline, for the browser's audio element
		}
		u, err := s.Blobs.PresignGet(ctx, a.StorageKey, s.DownloadTTL, filename)
		if err != nil {
			return "", internal(err)
		}
		if a.Kind != "manifest" && !preview {
			if err := s.Packages.CountDownload(ctx, v.ID); err != nil {
				s.Logger.WarnContext(ctx, "count download", "package", v.CanonicalID(), "err", err)
			}
		}
		return u, nil
	}
	return "", domain.NotFound("artifact_not_found", "no such file in this release")
}

// PreviewURL returns a presigned URL for audio that represents the package: a "preview" artifact of
// the latest release if there is one, otherwise its first sample. Previews are not counted as downloads.
func (s *Service) PreviewURL(ctx context.Context, actor Actor, publisherSlug, slug string) (string, error) {
	v, _, err := s.GetPackage(ctx, actor, publisherSlug, slug)
	if err != nil {
		return "", err
	}
	if v.LatestVersion == nil {
		return "", domain.NotFound("preview_not_found", "this package has no published audio")
	}
	r, err := s.release(ctx, v.ID, *v.LatestVersion)
	if err != nil {
		return "", err
	}
	arts, err := s.Releases.Artifacts(ctx, r.ID)
	if err != nil {
		return "", internal(err)
	}
	var pick *domain.Artifact
	for i := range arts {
		a := &arts[i]
		if a.Kind == "preview" {
			pick = a
			break
		}
		if a.Kind == "sample" && pick == nil {
			pick = a
		}
	}
	if pick == nil {
		return "", domain.NotFound("preview_not_found", "this package has no published audio")
	}
	u, err := s.Blobs.PresignGet(ctx, pick.StorageKey, s.DownloadTTL, "")
	if err != nil {
		return "", internal(err)
	}
	return u, nil
}
