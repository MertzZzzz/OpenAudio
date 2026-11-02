// Package memory provides thread-safe in-memory implementations of the domain
// repositories. Used by unit and HTTP tests; semantics mirror the Postgres adapter.
package memory

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type Store struct {
	mu         sync.Mutex
	users      map[uuid.UUID]domain.User
	identities map[uuid.UUID]domain.Identity
	refresh    map[uuid.UUID]domain.RefreshToken
	oneTime    map[uuid.UUID]domain.OneTimeToken
}

func NewStore() *Store {
	return &Store{
		users:      map[uuid.UUID]domain.User{},
		identities: map[uuid.UUID]domain.Identity{},
		refresh:    map[uuid.UUID]domain.RefreshToken{},
		oneTime:    map[uuid.UUID]domain.OneTimeToken{},
	}
}

func (s *Store) Users() domain.UserRepository           { return (*users)(s) }
func (s *Store) Identities() domain.IdentityRepository  { return (*identities)(s) }
func (s *Store) Refresh() domain.RefreshTokenRepository { return (*refresh)(s) }
func (s *Store) OneTime() domain.OneTimeTokenRepository { return (*oneTime)(s) }

// RefreshTokens returns a snapshot, for assertions in tests.
func (s *Store) RefreshTokens() []domain.RefreshToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.RefreshToken, 0, len(s.refresh))
	for _, t := range s.refresh {
		out = append(out, t)
	}
	return out
}

/* ------------------------------------------------------------------ users */

type users Store

func (r *users) emailTaken(email string) bool {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return true
		}
	}
	return false
}

func (r *users) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.emailTaken(u.Email) {
		return domain.ErrConflict
	}
	r.users[u.ID] = *u
	return nil
}

func (r *users) CreateWithIdentity(_ context.Context, u *domain.User, id *domain.Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.emailTaken(u.Email) {
		return domain.ErrConflict
	}
	for _, i := range r.identities {
		if i.Provider == id.Provider && i.Subject == id.Subject {
			return domain.ErrConflict
		}
	}
	r.users[u.ID] = *u
	r.identities[id.ID] = *id
	return nil
}

func (r *users) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (r *users) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return &u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *users) update(id uuid.UUID, f func(*domain.User)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	f(&u)
	r.users[id] = u
	return nil
}

func (r *users) UpdateRole(_ context.Context, id uuid.UUID, role domain.Role) error {
	return r.update(id, func(u *domain.User) { u.Role = role })
}

func (r *users) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	return r.update(id, func(u *domain.User) { u.PasswordHash = &hash })
}

func (r *users) MarkEmailVerified(_ context.Context, id uuid.UUID, at time.Time) error {
	return r.update(id, func(u *domain.User) {
		if u.EmailVerifiedAt == nil {
			u.EmailVerifiedAt = &at
		}
	})
}

/* ------------------------------------------------------------- identities */

type identities Store

func (r *identities) Get(_ context.Context, p domain.AuthProvider, subject string) (*domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, i := range r.identities {
		if i.Provider == p && i.Subject == subject {
			return &i, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *identities) Create(_ context.Context, id *domain.Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, i := range r.identities {
		if (i.Provider == id.Provider && i.Subject == id.Subject) || (i.Provider == id.Provider && i.UserID == id.UserID) {
			return domain.ErrConflict
		}
	}
	r.identities[id.ID] = *id
	return nil
}

func (r *identities) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Identity{}
	for _, i := range r.identities {
		if i.UserID == userID {
			out = append(out, i)
		}
	}
	return out, nil
}

/* ---------------------------------------------------------------- refresh */

type refresh Store

func (r *refresh) Create(_ context.Context, t *domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refresh[t.ID] = *t
	return nil
}

func (r *refresh) GetByHash(_ context.Context, hash []byte) (*domain.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.refresh {
		if bytes.Equal(t.TokenHash, hash) {
			return &t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *refresh) Rotate(_ context.Context, oldID uuid.UUID, next *domain.RefreshToken, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.refresh[oldID]
	if !ok || old.RevokedAt != nil {
		return domain.Conflict("token_reused", "refresh token already rotated")
	}
	old.RevokedAt, old.ReplacedBy = &at, &next.ID
	r.refresh[oldID] = old
	r.refresh[next.ID] = *next
	return nil
}

func (r *refresh) revokeWhere(at time.Time, match func(domain.RefreshToken) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.refresh {
		if match(t) && t.RevokedAt == nil {
			t.RevokedAt = &at
			r.refresh[id] = t
		}
	}
}

func (r *refresh) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	r.revokeWhere(at, func(t domain.RefreshToken) bool { return t.ID == id })
	return nil
}

func (r *refresh) RevokeFamily(_ context.Context, family uuid.UUID, at time.Time) error {
	r.revokeWhere(at, func(t domain.RefreshToken) bool { return t.FamilyID == family })
	return nil
}

func (r *refresh) RevokeAllForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	r.revokeWhere(at, func(t domain.RefreshToken) bool { return t.UserID == userID })
	return nil
}

/* --------------------------------------------------------------- one-time */

type oneTime Store

func (r *oneTime) Create(_ context.Context, t *domain.OneTimeToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.oneTime[t.ID] = *t
	return nil
}

func (r *oneTime) Consume(_ context.Context, hash []byte, purpose domain.OneTimePurpose, at time.Time) (*domain.OneTimeToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.oneTime {
		if bytes.Equal(t.TokenHash, hash) && t.Purpose == purpose && t.UsedAt == nil && at.Before(t.ExpiresAt) {
			t.UsedAt = &at
			r.oneTime[id] = t
			return &t, nil
		}
	}
	return nil, domain.Unauthorized("token_invalid", "token invalid")
}

func (r *oneTime) InvalidateForUser(_ context.Context, userID uuid.UUID, purpose domain.OneTimePurpose, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.oneTime {
		if t.UserID == userID && t.Purpose == purpose && t.UsedAt == nil {
			t.UsedAt = &at
			r.oneTime[id] = t
		}
	}
	return nil
}
