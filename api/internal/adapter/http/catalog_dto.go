package http

import (
	"encoding/json"
	"time"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/publishing"
)

type publisherDTO struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
	Role      string    `json:"role,omitempty"`
}

func toPublisherDTO(p *domain.Publisher, role domain.MemberRole) publisherDTO {
	return publisherDTO{ID: p.Slug, Slug: p.Slug, Name: p.Name, CreatedAt: p.CreatedAt, Role: string(role)}
}

type memberDTO struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func toMemberDTO(m domain.Member) memberDTO {
	return memberDTO{UserID: m.UserID.String(), Name: m.DisplayName, Email: m.Email, Role: string(m.Role)}
}

type packageStatsDTO struct {
	Artifacts int   `json:"artifacts"`
	Samples   int   `json:"samples"`
	SizeBytes int64 `json:"sizeBytes"`
}

// packageDTO flattens the package row and the metadata of its latest published manifest.
type packageDTO struct {
	ID            string          `json:"id"`
	Publisher     publisherDTO    `json:"publisher"`
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	Description   string          `json:"description"`
	Visibility    string          `json:"visibility"`
	LatestVersion *string         `json:"latestVersion"`
	PublishedAt   *time.Time      `json:"publishedAt"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	Stats         packageStatsDTO `json:"stats"`
	Downloads     int64           `json:"downloads"`
	License       json.RawMessage `json:"license"`
	Taxonomy      json.RawMessage `json:"taxonomy"`
	Musical       json.RawMessage `json:"musical"`
	Technical     json.RawMessage `json:"technical"`
	Module        json.RawMessage `json:"module"`
	Dependencies  json.RawMessage `json:"dependencies"`
	Tags          []string        `json:"tags"`
}

var jsonNull = json.RawMessage("null")

func toPackageDTO(v *domain.PackageView) packageDTO {
	d := packageDTO{
		ID: v.CanonicalID(), Publisher: publisherDTO{ID: v.PublisherSlug, Slug: v.PublisherSlug, Name: v.PublisherName},
		Slug: v.Slug, Name: v.Name, Kind: v.Kind, Description: v.Description, Visibility: v.Visibility,
		LatestVersion: v.LatestVersion, PublishedAt: v.LatestPublishedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		Stats:     packageStatsDTO{Artifacts: v.ArtifactCount, Samples: v.SampleCount, SizeBytes: v.TotalSize},
		Downloads: v.Downloads,
		License:   jsonNull, Taxonomy: jsonNull, Musical: jsonNull, Technical: jsonNull, Module: jsonNull, Dependencies: json.RawMessage("[]"), Tags: []string{},
	}
	if len(v.LatestManifest) == 0 {
		return d
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(v.LatestManifest, &m) != nil {
		return d
	}
	pick := func(dst *json.RawMessage, key string) {
		if raw, ok := m[key]; ok {
			*dst = raw
		}
	}
	pick(&d.License, "license")
	pick(&d.Taxonomy, "taxonomy")
	pick(&d.Musical, "musical")
	pick(&d.Technical, "technical")
	pick(&d.Module, "module")
	pick(&d.Dependencies, "dependencies")
	var tags struct {
		Free []string `json:"free"`
	}
	if raw, ok := m["tags"]; ok && json.Unmarshal(raw, &tags) == nil && tags.Free != nil {
		d.Tags = tags.Free
	}
	return d
}

type releaseDTO struct {
	Version        string     `json:"version"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"publishedAt"`
	Changelog      string     `json:"changelog"`
	ManifestDigest string     `json:"manifestDigest"`
	CreatedAt      time.Time  `json:"createdAt"`
}

func toReleaseDTO(r *domain.Release) releaseDTO {
	return releaseDTO{Version: r.Version, Status: string(r.Status), PublishedAt: r.PublishedAt, Changelog: r.Changelog, ManifestDigest: r.ManifestDigest, CreatedAt: r.CreatedAt}
}

func toReleaseDTOs(rs []domain.Release) []releaseDTO {
	out := make([]releaseDTO, len(rs))
	for i := range rs {
		out[i] = toReleaseDTO(&rs[i])
	}
	return out
}

type artifactDTO struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

type releaseDetailsDTO struct {
	releaseDTO
	Package   string        `json:"package"`
	Artifacts []artifactDTO `json:"artifacts"`
}

func toReleaseDetails(d *publishing.ReleaseDetails) releaseDetailsDTO {
	out := releaseDetailsDTO{releaseDTO: toReleaseDTO(d.Release), Package: d.Package.CanonicalID(), Artifacts: []artifactDTO{}}
	for _, a := range d.Artifacts {
		out.Artifacts = append(out.Artifacts, artifactDTO{Kind: a.Kind, Path: a.Path, Digest: a.Digest, Size: a.SizeBytes, ContentType: a.ContentType, DownloadURL: a.DownloadURL})
	}
	return out
}

type uploadPartDTO struct {
	Number int   `json:"number"`
	Size   int64 `json:"size"`
}

type uploadDTO struct {
	ID          string          `json:"id"`
	Digest      string          `json:"digest"`
	Size        int64           `json:"size"`
	ContentType string          `json:"contentType,omitempty"`
	Status      string          `json:"status"`
	PartSize    int64           `json:"partSize"`
	PartCount   int             `json:"partCount"`
	Parts       []uploadPartDTO `json:"parts"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	CompletedAt *time.Time      `json:"completedAt"`
}

func toUploadDTO(u *domain.Upload, parts []domain.UploadPart) uploadDTO {
	d := uploadDTO{ID: u.ID.String(), Digest: u.Digest, Size: u.Size, ContentType: u.ContentType, Status: string(u.Status),
		PartSize: u.PartSize, PartCount: u.PartCount(), Parts: []uploadPartDTO{}, Error: u.Error, CreatedAt: u.CreatedAt, CompletedAt: u.CompletedAt}
	if u.Status == domain.UploadDone {
		d.PartCount = 0 // nothing left to send
	}
	for _, p := range parts {
		d.Parts = append(d.Parts, uploadPartDTO{Number: p.Number, Size: p.Size})
	}
	return d
}
