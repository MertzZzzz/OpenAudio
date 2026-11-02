package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type libraryRow struct {
	pinned, installed *string
	addedAt           time.Time
}

func (c *Catalog) Libraries() domain.LibraryRepository  { return (*libraries)(c) }
func (c *Catalog) Lockfiles() domain.LockfileRepository { return (*lockfiles)(c) }

type libraries Catalog

func (r *libraries) ListForUser(_ context.Context, userID uuid.UUID) ([]domain.Library, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Library{}
	for _, l := range r.libs {
		if l.UserID == userID {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *libraries) Create(_ context.Context, l *domain.Library) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.libs {
		if x.UserID == l.UserID && strings.EqualFold(x.Name, l.Name) {
			return domain.ErrConflict
		}
	}
	r.libs[l.ID] = *l
	r.items[l.ID] = map[uuid.UUID]libraryRow{}
	return nil
}

func (r *libraries) Get(_ context.Context, id uuid.UUID) (*domain.Library, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.libs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &l, nil
}

func (r *libraries) Rename(_ context.Context, id uuid.UUID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.libs[id]
	if !ok {
		return domain.ErrNotFound
	}
	for _, x := range r.libs {
		if x.ID != id && x.UserID == l.UserID && strings.EqualFold(x.Name, name) {
			return domain.ErrConflict
		}
	}
	l.Name = name
	r.libs[id] = l
	return nil
}

func (r *libraries) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.libs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.libs, id)
	delete(r.items, id)
	return nil
}

func (r *libraries) Items(_ context.Context, libraryID uuid.UUID) ([]domain.LibraryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.LibraryItem{}
	for pkgID, row := range r.items[libraryID] {
		p, ok := r.packages[pkgID]
		if !ok {
			continue
		}
		out = append(out, domain.LibraryItem{LibraryID: libraryID, Package: (*packages)(r).view(p), PinnedVersion: row.pinned, InstalledVersion: row.installed, AddedAt: row.addedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt.Before(out[j].AddedAt) })
	return out, nil
}

func (r *libraries) PutItem(_ context.Context, libraryID, packageID uuid.UUID, pinned, installed *string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items, ok := r.items[libraryID]
	if !ok {
		return domain.ErrNotFound
	}
	row, exists := items[packageID]
	if !exists {
		row.addedAt = at
	}
	row.pinned, row.installed = pinned, installed
	items[packageID] = row
	return nil
}

func (r *libraries) RemoveItem(_ context.Context, libraryID, packageID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[libraryID][packageID]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items[libraryID], packageID)
	return nil
}

type lockfiles Catalog

func (r *lockfiles) Create(_ context.Context, l *domain.LockfileRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locks[l.ID] = *l
	return nil
}

func (r *lockfiles) ListForUser(_ context.Context, userID uuid.UUID) ([]domain.LockfileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.LockfileRecord{}
	for _, l := range r.locks {
		if l.OwnerUserID == userID {
			l.Content = nil
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *lockfiles) Get(_ context.Context, id uuid.UUID) (*domain.LockfileRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.locks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &l, nil
}

func (r *lockfiles) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.locks[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.locks, id)
	return nil
}
