package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// JWTIssuer signs and verifies HS256 access tokens.
type JWTIssuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

const MinSecretLength = 32

func NewJWTIssuer(secret []byte, issuer string, ttl time.Duration, now func() time.Time) (*JWTIssuer, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes", MinSecretLength)
	}
	if now == nil {
		now = time.Now
	}
	return &JWTIssuer{secret: secret, issuer: issuer, audience: "openaudio-api", ttl: ttl, now: now}, nil
}

type accessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (j *JWTIssuer) TTL() time.Duration { return j.ttl }

// Issue returns a signed access token and its expiry.
func (j *JWTIssuer) Issue(userID uuid.UUID, role domain.Role) (string, time.Time, error) {
	now := j.now()
	exp := now.Add(j.ttl)
	claims := accessClaims{
		Role: string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{j.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, exp, nil
}

var ErrInvalidToken = errors.New("invalid access token")

// Parse verifies signature, algorithm, issuer, audience and expiry.
func (j *JWTIssuer) Parse(token string) (*domain.AccessClaims, error) {
	var c accessClaims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience(j.audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	role := domain.Role(c.Role)
	if !role.Valid() {
		return nil, ErrInvalidToken
	}
	return &domain.AccessClaims{UserID: id, Role: role, ExpiresAt: c.ExpiresAt.Time}, nil
}
