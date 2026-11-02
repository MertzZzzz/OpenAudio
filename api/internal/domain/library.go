package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Library struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	CreatedAt time.Time
}

// LibraryItem is a package in a library. PinnedVersion fixes the version; otherwise the
// resolver keeps InstalledVersion while it satisfies every range, else picks the newest.
type LibraryItem struct {
	LibraryID        uuid.UUID
	Package          PackageView
	PinnedVersion    *string
	InstalledVersion *string
	AddedAt          time.Time
}

type LibraryRepository interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Library, error) // oldest first
	Create(ctx context.Context, l *Library) error                         // ErrConflict on a duplicate name
	Get(ctx context.Context, id uuid.UUID) (*Library, error)
	Rename(ctx context.Context, id uuid.UUID, name string) error
	Delete(ctx context.Context, id uuid.UUID) error
	Items(ctx context.Context, libraryID uuid.UUID) ([]LibraryItem, error) // ordered by added_at
	// PutItem inserts or updates an item (the added_at of existing items is kept).
	PutItem(ctx context.Context, libraryID, packageID uuid.UUID, pinned, installed *string, at time.Time) error
	RemoveItem(ctx context.Context, libraryID, packageID uuid.UUID) error
}

// LockfileRecord is a saved project lockfile (canonical JSON bytes + sha256).
type LockfileRecord struct {
	ID            uuid.UUID
	OwnerUserID   uuid.UUID
	LibraryID     *uuid.UUID
	Name          string
	Content       []byte
	ContentDigest string
	CreatedAt     time.Time
}

type LockfileRepository interface {
	Create(ctx context.Context, l *LockfileRecord) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]LockfileRecord, error) // newest first, without content
	Get(ctx context.Context, id uuid.UUID) (*LockfileRecord, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
