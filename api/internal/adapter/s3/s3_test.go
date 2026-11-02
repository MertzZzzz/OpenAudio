package s3

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

func TestBlobKey(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	k, err := domain.BlobKey("sha256:" + hex)
	require.NoError(t, err)
	assert.Equal(t, "blobs/"+hex, k)

	for _, bad := range []string{"", "sha256:xyz", "md5:" + hex, "sha256:" + strings.ToUpper(hex), "sha256:" + hex + "/../x"} {
		_, err := domain.BlobKey(bad)
		assert.Error(t, err, bad)
	}
}

func TestPresignIsOffline(t *testing.T) {
	// Presigning is pure signing; it must not need a reachable server.
	s, err := New(Config{Endpoint: "http://minio.invalid:9000", AccessKey: "ak", SecretKey: "sk", Bucket: "blobs-test", Region: "us-east-1"})
	require.NoError(t, err)
	u, err := s.PresignGet(context.Background(), "blobs/abc", 5*time.Minute, "kick.wav")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "http://minio.invalid:9000/blobs-test/blobs/abc?"))
	assert.Contains(t, u, "X-Amz-Expires=300")
	assert.Contains(t, u, "kick.wav")
}

func TestPresignUsesPublicEndpoint(t *testing.T) {
	s, err := New(Config{Endpoint: "http://minio:9000", PublicEndpoint: "https://cdn.example.com", AccessKey: "ak", SecretKey: "sk", Bucket: "blobs-test", Region: "us-east-1"})
	require.NoError(t, err)
	u, err := s.PresignGet(context.Background(), "blobs/abc", time.Minute, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "https://cdn.example.com/blobs-test/blobs/abc?"), u)
}

func TestEnsureBucketAndPingAgainstFakeS3(t *testing.T) {
	backend := s3mem.New()
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	defer srv.Close()

	s, err := New(Config{Endpoint: srv.URL, AccessKey: "ak", SecretKey: "sk", Bucket: "openaudio-test", Region: "us-east-1"})
	require.NoError(t, err)
	ctx := context.Background()

	assert.Error(t, s.Ping(ctx), "bucket does not exist yet")
	require.NoError(t, s.EnsureBucket(ctx))
	require.NoError(t, s.EnsureBucket(ctx), "idempotent")
	assert.NoError(t, s.Ping(ctx))
}

func TestBlobStoreAgainstFakeS3(t *testing.T) {
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	defer srv.Close()
	s, err := New(Config{Endpoint: srv.URL, AccessKey: "ak", SecretKey: "sk", Bucket: "openaudio-test", Region: "us-east-1"})
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, s.EnsureBucket(ctx))

	_, ok, err := s.Stat(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)
	_, err = s.Open(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	require.NoError(t, s.Put(ctx, domain.ManifestKey("acme.kit", "1.0.0"), []byte(`{"a":1}`), "application/json"))
	size, ok, err := s.Stat(ctx, "manifests/acme.kit/1.0.0/manifest.json")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.EqualValues(t, 7, size)

	// Multipart: two 5 MiB parts (S3 minimum for non-final parts) + a small tail.
	part := func(b byte, n int) []byte { return bytes.Repeat([]byte{b}, n) }
	chunks := [][]byte{part('a', 5<<20), part('b', 5<<20), part('c', 123)}
	key := domain.UploadKey("u1")
	id, err := s.CreateMultipart(ctx, key, "audio/wav")
	require.NoError(t, err)
	var done []domain.CompletedPart
	for i := len(chunks) - 1; i >= 0; i-- { // out of order on purpose
		etag, err := s.UploadPart(ctx, key, id, i+1, bytes.NewReader(chunks[i]), int64(len(chunks[i])))
		require.NoError(t, err)
		done = append([]domain.CompletedPart{{Number: i + 1, ETag: etag}}, done...)
	}
	require.NoError(t, s.CompleteMultipart(ctx, key, id, done))

	r, err := s.Open(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	r.Close()
	assert.Equal(t, bytes.Join(chunks, nil), got)

	require.NoError(t, s.Copy(ctx, key, "blobs/abc"))
	require.NoError(t, s.Delete(ctx, key))
	_, ok, _ = s.Stat(ctx, key)
	assert.False(t, ok)
	size, ok, _ = s.Stat(ctx, "blobs/abc")
	assert.True(t, ok)
	assert.EqualValues(t, 10<<20+123, size)

	id2, err := s.CreateMultipart(ctx, domain.UploadKey("u2"), "")
	require.NoError(t, err)
	assert.NoError(t, s.AbortMultipart(ctx, domain.UploadKey("u2"), id2))
}
