package security

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

var fastParams = Argon2Params{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestArgon2HashAndVerify(t *testing.T) {
	h := NewArgon2Hasher(fastParams)
	enc, err := h.Hash("correct horse battery")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(enc, "$argon2id$v=19$m=1024,t=1,p=1$"))

	ok, err := h.Verify("correct horse battery", enc)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = h.Verify("wrong", enc)
	require.NoError(t, err)
	assert.False(t, ok)

	enc2, _ := h.Hash("correct horse battery")
	assert.NotEqual(t, enc, enc2, "salt must be random")
}

func TestArgon2RejectsMalformedHash(t *testing.T) {
	h := NewArgon2Hasher(fastParams)
	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=18$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=1,t=1,p=1$!!$aGFzaA"} {
		_, err := h.Verify("x", bad)
		assert.ErrorIs(t, err, ErrInvalidHash, bad)
	}
}

func TestArgon2NeedsRehash(t *testing.T) {
	weak := NewArgon2Hasher(fastParams)
	enc, _ := weak.Hash("pw")
	assert.False(t, weak.NeedsRehash(enc))
	assert.True(t, NewArgon2Hasher(DefaultArgon2Params).NeedsRehash(enc))
}

func TestJWTRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	j, err := NewJWTIssuer([]byte(strings.Repeat("s", 32)), "openaudio", 15*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	id := uuid.New()
	tok, exp, err := j.Issue(id, domain.RolePublisher)
	require.NoError(t, err)
	assert.Equal(t, now.Add(15*time.Minute), exp)

	c, err := j.Parse(tok)
	require.NoError(t, err)
	assert.Equal(t, id, c.UserID)
	assert.Equal(t, domain.RolePublisher, c.Role)
}

func TestJWTRejections(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	secret := []byte(strings.Repeat("s", 32))
	j, _ := NewJWTIssuer(secret, "openaudio", time.Minute, clock)
	tok, _, _ := j.Issue(uuid.New(), domain.RoleUser)

	t.Run("expired", func(t *testing.T) {
		later, _ := NewJWTIssuer(secret, "openaudio", time.Minute, func() time.Time { return now.Add(2 * time.Minute) })
		_, err := later.Parse(tok)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})
	t.Run("wrong secret", func(t *testing.T) {
		other, _ := NewJWTIssuer([]byte(strings.Repeat("x", 32)), "openaudio", time.Minute, clock)
		_, err := other.Parse(tok)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})
	t.Run("wrong issuer", func(t *testing.T) {
		other, _ := NewJWTIssuer(secret, "someone-else", time.Minute, clock)
		_, err := other.Parse(tok)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})
	t.Run("alg none", func(t *testing.T) {
		unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": uuid.NewString(), "role": "admin", "iss": "openaudio", "aud": "openaudio-api", "exp": now.Add(time.Hour).Unix()})
		s, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)
		_, err = j.Parse(s)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})
	t.Run("short secret", func(t *testing.T) {
		_, err := NewJWTIssuer([]byte("short"), "openaudio", time.Minute, clock)
		assert.Error(t, err)
	})
}

func TestOpaqueToken(t *testing.T) {
	a, ha, err := NewOpaqueToken()
	require.NoError(t, err)
	b, _, _ := NewOpaqueToken()
	assert.NotEqual(t, a, b)
	assert.Len(t, a, 43)
	assert.Equal(t, ha, HashToken(a))
}
