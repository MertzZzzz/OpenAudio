package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

/* ------------------------------------------------------------ publishers */

// MemberRole is a user's role inside one publisher.
type MemberRole string

const (
	MemberOwner      MemberRole = "owner"
	MemberMaintainer MemberRole = "maintainer"
	MemberUploader   MemberRole = "uploader"
)

func (r MemberRole) Valid() bool {
	return r == MemberOwner || r == MemberMaintainer || r == MemberUploader
}

// Permission is an action on a publisher's resources.
type Permission int

const (
	PermDraft          Permission = iota // create / edit / delete draft releases, upload blobs
	PermPublish                          // publish and yank releases
	PermManagePackages                   // create packages, edit package metadata
	PermManageMembers                    // invite / remove members
)

// Allows encodes the publisher permission matrix:
//
//	           draft  publish  packages  members
//	owner        ✓       ✓        ✓         ✓
//	maintainer   ✓       ✓        ✓
//	uploader     ✓
func (r MemberRole) Allows(p Permission) bool {
	switch r {
	case MemberOwner:
		return true
	case MemberMaintainer:
		return p != PermManageMembers
	case MemberUploader:
		return p == PermDraft
	}
	return false
}

type Publisher struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	OwnerUserID *uuid.UUID
	CreatedAt   time.Time
}

type PublisherMembership struct {
	Publisher Publisher
	Role      MemberRole
}

type Member struct {
	UserID      uuid.UUID
	DisplayName string
	Email       string
	Role        MemberRole
}

type PublisherRepository interface {
	// Create inserts the publisher and its owner membership atomically. ErrConflict on a taken slug.
	Create(ctx context.Context, p *Publisher, owner uuid.UUID) error
	GetBySlug(ctx context.Context, slug string) (*Publisher, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]PublisherMembership, error)
	// MemberRole returns ErrNotFound when the user is not a member.
	MemberRole(ctx context.Context, publisherID, userID uuid.UUID) (MemberRole, error)
	Members(ctx context.Context, publisherID uuid.UUID) ([]Member, error)
	AddMember(ctx context.Context, publisherID, userID uuid.UUID, role MemberRole) error // ErrConflict if already a member
	RemoveMember(ctx context.Context, publisherID, userID uuid.UUID) error
}

/* -------------------------------------------------------------- packages */

var PackageKinds = []string{"sample-pack", "preset-pack", "instrument", "effect", "midi-tool", "utility", "project-template", "model"}

type Package struct {
	ID            uuid.UUID
	PublisherID   uuid.UUID
	PublisherSlug string
	PublisherName string
	Slug          string
	Name          string
	Kind          string
	Description   string
	LatestVersion *string // highest published (not yanked) version
	Visibility    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CanonicalID is the package id used in manifests and dependencies: "<publisher>.<package>".
func (p *Package) CanonicalID() string { return p.PublisherSlug + "." + p.Slug }

// PackageView is a package together with its latest published manifest and artifact stats.
type PackageView struct {
	Package
	LatestManifest    []byte // canonical JSON, nil when nothing is published
	LatestPublishedAt *time.Time
	ArtifactCount     int
	SampleCount       int
	TotalSize         int64
	Downloads         int64
}

type PackageSort string

const (
	SortNewest  PackageSort = "newest" // most recently published / updated
	SortName    PackageSort = "name"
	SortCreated PackageSort = "created"
)

type PackageFilter struct {
	Query         string
	Kinds         []string
	PublisherSlug string
	// IncludeUnpublished also lists packages without a published release (publisher dashboards).
	IncludeUnpublished bool
	Sort               PackageSort
	Limit, Offset      int
}

type PackageRepository interface {
	Create(ctx context.Context, p *Package) error // ErrConflict on (publisher, slug)
	Get(ctx context.Context, publisherSlug, slug string) (*PackageView, error)
	List(ctx context.Context, f PackageFilter) ([]PackageView, int, error)
	Update(ctx context.Context, id uuid.UUID, name, description, visibility string, at time.Time) error
	// CountDownload increments the package's download counter (does not touch updated_at).
	CountDownload(ctx context.Context, id uuid.UUID) error
}

/* -------------------------------------------------------------- releases */

type ReleaseStatus string

const (
	ReleaseDraft     ReleaseStatus = "draft"
	ReleasePublished ReleaseStatus = "published"
	ReleaseYanked    ReleaseStatus = "yanked"
)

type Release struct {
	ID             uuid.UUID
	PackageID      uuid.UUID
	Version        string
	Manifest       []byte // canonical JSON
	ManifestDigest string
	Changelog      string
	Status         ReleaseStatus
	PublishedAt    *time.Time
	CreatedBy      *uuid.UUID
	PublishedBy    *uuid.UUID
	CreatedAt      time.Time
}

type Artifact struct {
	ID          uuid.UUID
	ReleaseID   uuid.UUID
	Kind        string
	Path        string
	Digest      string
	SizeBytes   int64
	ContentType string
	StorageKey  string
	CreatedAt   time.Time
}

type TaxonomyTerm struct {
	Domain string
	Slug   string
	Weight float64
}

type PublishInput struct {
	ReleaseID   uuid.UUID
	PackageID   uuid.UUID
	PublishedBy uuid.UUID
	Artifacts   []Artifact
	Terms       []TaxonomyTerm
	At          time.Time
}

type ReleaseRepository interface {
	CreateDraft(ctx context.Context, r *Release) error // ErrConflict on an existing version
	Get(ctx context.Context, packageID uuid.UUID, version string) (*Release, error)
	List(ctx context.Context, packageID uuid.UUID) ([]Release, error) // newest version first
	// UpdateDraft replaces manifest and changelog; Conflict "release_immutable" unless status is draft.
	UpdateDraft(ctx context.Context, id uuid.UUID, manifest []byte, digest, changelog string) error
	DeleteDraft(ctx context.Context, id uuid.UUID) error
	// Publish atomically stores artifacts and taxonomy, marks the draft published and
	// recomputes packages.latest_version. Conflict "release_immutable" unless status is draft.
	Publish(ctx context.Context, in PublishInput) error
	// Yank marks a published release yanked and recomputes latest_version.
	Yank(ctx context.Context, releaseID, packageID uuid.UUID, at time.Time) error
	Artifacts(ctx context.Context, releaseID uuid.UUID) ([]Artifact, error)
}

/* --------------------------------------------------------------- uploads */

type UploadStatus string

const (
	UploadPending UploadStatus = "pending"
	UploadDone    UploadStatus = "done"
	UploadFailed  UploadStatus = "failed"
)

// Upload is a resumable multipart upload of one content-addressed blob.
type Upload struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Digest      string
	Size        int64
	ContentType string
	Status      UploadStatus
	MultipartID string // S3 multipart upload id ("" when the blob already existed)
	PartSize    int64
	Error       string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (u *Upload) PartCount() int {
	if u.PartSize <= 0 {
		return 0
	}
	return int((u.Size + u.PartSize - 1) / u.PartSize)
}

// ExpectedPartSize is the exact byte length part n (1-based) must have.
func (u *Upload) ExpectedPartSize(n int) int64 {
	if n < u.PartCount() {
		return u.PartSize
	}
	return u.Size - int64(u.PartCount()-1)*u.PartSize
}

type UploadPart struct {
	Number int
	ETag   string
	Size   int64
}

type UploadRepository interface {
	Create(ctx context.Context, u *Upload) error
	Get(ctx context.Context, id uuid.UUID) (*Upload, error)
	PutPart(ctx context.Context, uploadID uuid.UUID, p UploadPart) error // upsert by part number
	Parts(ctx context.Context, uploadID uuid.UUID) ([]UploadPart, error) // ordered by number
	Finish(ctx context.Context, id uuid.UUID, status UploadStatus, errMsg string, at time.Time) error
}

/* ------------------------------------------------------------ blob store */

type CompletedPart struct {
	Number int
	ETag   string
}

// BlobStore is S3-compatible object storage.
type BlobStore interface {
	Stat(ctx context.Context, key string) (size int64, exists bool, err error)
	Put(ctx context.Context, key string, data []byte, contentType string) error
	CreateMultipart(ctx context.Context, key, contentType string) (uploadID string, err error)
	UploadPart(ctx context.Context, key, uploadID string, number int, r io.Reader, size int64) (etag string, err error)
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipart(ctx context.Context, key, uploadID string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Copy(ctx context.Context, srcKey, dstKey string) error
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error)
}
