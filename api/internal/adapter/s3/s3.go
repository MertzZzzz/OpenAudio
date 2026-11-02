// Package s3 stores content-addressed blobs in S3-compatible storage (AWS S3 / MinIO).
package s3

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string // host:port or URL, e.g. "http://minio:9000"
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

type Storage struct {
	client *minio.Client
	bucket string
}

func New(cfg Config) (*Storage, error) {
	endpoint, secure := cfg.Endpoint, cfg.UseSSL
	if u, err := url.Parse(cfg.Endpoint); err == nil && u.Host != "" {
		endpoint, secure = u.Host, u.Scheme == "https"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

// EnsureBucket creates the bucket when missing (local development convenience).
func (s *Storage) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if ok {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

// Ping is used by the readiness probe.
func (s *Storage) Ping(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q does not exist", s.bucket)
	}
	return nil
}

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// BlobKey maps "sha256:<hex>" to the object key "blobs/<hex>" (spec §8).
func BlobKey(digest string) (string, error) {
	if !digestRe.MatchString(digest) {
		return "", fmt.Errorf("invalid digest %q", digest)
	}
	return "blobs/" + strings.TrimPrefix(digest, "sha256:"), nil
}

// PresignGet returns a short-lived download URL for an object.
func (s *Storage) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	params := url.Values{}
	if filename != "" {
		params.Set("response-content-disposition", fmt.Sprintf("attachment; filename=%q", filename))
	}
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, params)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
