// Package canonjson produces canonical JSON: object keys sorted, no insignificant whitespace,
// no HTML escaping, numbers kept verbatim. Equal documents always produce equal bytes, so a
// sha256 over the bytes is a stable content digest (used for manifests and lockfiles).
package canonjson

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Parse decodes JSON keeping numbers exact (json.Number).
func Parse(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("expected a single JSON value")
	}
	return v, nil
}

// Marshal returns the canonical encoding of v (maps are emitted with sorted keys by encoding/json).
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Digest returns "sha256:<hex>" of b.
func Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FromValue canonicalises any Go value: it is marshalled, re-parsed (so struct field order and
// insignificant formatting disappear) and re-encoded with sorted keys.
func FromValue(v any) ([]byte, string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, "", err
	}
	parsed, err := Parse(raw)
	if err != nil {
		return nil, "", err
	}
	b, err := Marshal(parsed)
	if err != nil {
		return nil, "", err
	}
	return b, Digest(b), nil
}
