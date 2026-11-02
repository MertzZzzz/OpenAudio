package memory

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// Blobs is an in-memory S3 stand-in implementing domain.BlobStore.
type Blobs struct {
	mu        sync.Mutex
	objects   map[string][]byte
	multipart map[string]map[int][]byte // uploadID → parts
}

func NewBlobs() *Blobs {
	return &Blobs{objects: map[string][]byte{}, multipart: map[string]map[int][]byte{}}
}

// Object returns a copy of a stored object (tests).
func (b *Blobs) Object(key string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.objects[key]
	return append([]byte(nil), v...), ok
}

// Keys lists stored object keys (tests).
func (b *Blobs) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for k := range b.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (b *Blobs) Stat(_ context.Context, key string) (int64, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.objects[key]
	return int64(len(v)), ok, nil
}

func (b *Blobs) Put(_ context.Context, key string, data []byte, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = append([]byte(nil), data...)
	return nil
}

func (b *Blobs) CreateMultipart(_ context.Context, _, _ string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := uuid.NewString()
	b.multipart[id] = map[int][]byte{}
	return id, nil
}

func (b *Blobs) UploadPart(_ context.Context, _, uploadID string, n int, r io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, size+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) != size {
		return "", fmt.Errorf("part %d: got %d bytes, want %d", n, len(data), size)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	parts, ok := b.multipart[uploadID]
	if !ok {
		return "", fmt.Errorf("no such upload %s", uploadID)
	}
	parts[n] = data
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}

func (b *Blobs) CompleteMultipart(_ context.Context, key, uploadID string, parts []domain.CompletedPart) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	stored, ok := b.multipart[uploadID]
	if !ok {
		return fmt.Errorf("no such upload %s", uploadID)
	}
	var buf bytes.Buffer
	for _, p := range parts {
		data, ok := stored[p.Number]
		if !ok {
			return fmt.Errorf("part %d missing", p.Number)
		}
		sum := md5.Sum(data)
		if hex.EncodeToString(sum[:]) != p.ETag {
			return fmt.Errorf("part %d etag mismatch", p.Number)
		}
		buf.Write(data)
	}
	b.objects[key] = buf.Bytes()
	delete(b.multipart, uploadID)
	return nil
}

func (b *Blobs) AbortMultipart(_ context.Context, _, uploadID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.multipart, uploadID)
	return nil
}

func (b *Blobs) Open(_ context.Context, key string) (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.objects[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), v...))), nil
}

func (b *Blobs) Copy(_ context.Context, src, dst string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.objects[src]
	if !ok {
		return domain.ErrNotFound
	}
	b.objects[dst] = append([]byte(nil), v...)
	return nil
}

func (b *Blobs) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, key)
	return nil
}

func (b *Blobs) PresignGet(_ context.Context, key string, ttl time.Duration, filename string) (string, error) {
	return fmt.Sprintf("https://blobs.test/%s?ttl=%d&filename=%s", key, int(ttl.Seconds()), filename), nil
}
