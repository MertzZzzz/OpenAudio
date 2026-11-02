package s3

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlobKey(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	k, err := BlobKey("sha256:" + hex)
	require.NoError(t, err)
	assert.Equal(t, "blobs/"+hex, k)

	for _, bad := range []string{"", "sha256:xyz", "md5:" + hex, "sha256:" + strings.ToUpper(hex), "sha256:" + hex + "/../x"} {
		_, err := BlobKey(bad)
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
