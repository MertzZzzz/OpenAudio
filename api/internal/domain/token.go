package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RefreshToken is stored hashed; the plaintext is only ever held by the client.
// Tokens form rotation families: presenting an already-rotated token revokes the family.
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	FamilyID   uuid.UUID
	TokenHash  []byte
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
	UserAgent  string
	IP         string
	CreatedAt  time.Time
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *RefreshToken) error
	GetByHash(ctx context.Context, hash []byte) (*RefreshToken, error)
	// Rotate revokes oldID and inserts next atomically. Returns an error with code
	// "token_reused" if oldID was already revoked (i.e. a replay).
	Rotate(ctx context.Context, oldID uuid.UUID, next *RefreshToken, at time.Time) error
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
}

// OneTimePurpose scopes single-use tokens (email verification, password reset).
type OneTimePurpose string

const (
	PurposeVerifyEmail   OneTimePurpose = "verify_email"
	PurposePasswordReset OneTimePurpose = "password_reset"
)

type OneTimeToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Purpose   OneTimePurpose
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type OneTimeTokenRepository interface {
	Create(ctx context.Context, t *OneTimeToken) error
	// Consume marks the token used and returns it; fails with "token_invalid" when the
	// token is unknown, expired, already used or issued for another purpose.
	Consume(ctx context.Context, hash []byte, purpose OneTimePurpose, at time.Time) (*OneTimeToken, error)
	InvalidateForUser(ctx context.Context, userID uuid.UUID, purpose OneTimePurpose, at time.Time) error
}

// AccessClaims is the payload of a verified access token.
type AccessClaims struct {
	UserID    uuid.UUID
	Role      Role
	ExpiresAt time.Time
}

// Mailer delivers transactional emails (verification, password reset).
type Mailer interface {
	Send(ctx context.Context, to, subject, textBody string) error
}
