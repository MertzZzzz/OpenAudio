package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Role is a coarse-grained platform role. Publisher-level permissions
// (owner / maintainer / uploader) live in publisher_members.
type Role string

const (
	RoleUser      Role = "user"
	RolePublisher Role = "publisher"
	RoleAdmin     Role = "admin"
)

func (r Role) Valid() bool { return r == RoleUser || r == RolePublisher || r == RoleAdmin }

// AtLeast reports whether r grants the privileges of min (admin ⊃ publisher ⊃ user).
func (r Role) AtLeast(min Role) bool { return r.rank() >= min.rank() }

func (r Role) rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RolePublisher:
		return 2
	case RoleUser:
		return 1
	}
	return 0
}

// AuthProvider identifies an external identity provider.
type AuthProvider string

const (
	ProviderGoogle  AuthProvider = "google"
	ProviderDiscord AuthProvider = "discord"
)

type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    *string // nil for accounts created through OAuth only
	DisplayName     string
	Role            Role
	AvatarURL       *string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (u *User) EmailVerified() bool { return u.EmailVerifiedAt != nil }
func (u *User) HasPassword() bool   { return u.PasswordHash != nil }

// Identity links a user to an account at an external provider.
type Identity struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Provider  AuthProvider
	Subject   string // stable provider user id ("sub" for Google, snowflake for Discord)
	Email     string
	CreatedAt time.Time
}

// ExternalProfile is what an OAuth provider tells us about the signed-in person.
type ExternalProfile struct {
	Provider      AuthProvider
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

type UserRepository interface {
	Create(ctx context.Context, u *User) error // ErrConflict-coded "email_taken" on duplicate email
	CreateWithIdentity(ctx context.Context, u *User, id *Identity) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role Role) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
	MarkEmailVerified(ctx context.Context, id uuid.UUID, at time.Time) error
}

type IdentityRepository interface {
	Get(ctx context.Context, provider AuthProvider, subject string) (*Identity, error)
	Create(ctx context.Context, id *Identity) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Identity, error)
}
