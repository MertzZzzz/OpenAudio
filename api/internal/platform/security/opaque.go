package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewOpaqueToken returns a random URL-safe token (256 bits) and its SHA-256 hash.
// Only the hash is persisted, so a database leak does not leak usable tokens.
func NewOpaqueToken() (plain string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, HashToken(plain), nil
}

func HashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
