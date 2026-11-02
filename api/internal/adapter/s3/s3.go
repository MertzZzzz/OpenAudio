// Package s3 stores content-addressed blobs in S3-compatible storage (AWS S3 / MinIO).
package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

type Config struct {
	Endpoint  string // host:port or URL, e.g. "http://minio:9000"
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

// Storage implements domain.BlobStore on an S3-compatible service.
type Storage struct {
	client *minio.Client
	core   minio.Core
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
	return &Storage{client: client, core: minio.Core{Client: client}, bucket: cfg.Bucket}, nil
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

func isNotFound(err error) bool {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey" || resp.Code == "NoSuchBucket"
	}
	return false
}

func (s *Storage) Stat(ctx context.Context, key string) (int64, bool, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(minio.ToErrorResponse(err)) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return info.Size, true, nil
}

func (s *Storage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *Storage) CreateMultipart(ctx context.Context, key, contentType string) (string, error) {
	return s.core.NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{ContentType: contentType})
}

// UploadPart buffers one part (parts are bounded by the upload part size), and sends it with
// its MD5 and SHA-256 so S3 verifies the bytes in transit. Sending the payload hash also
// avoids aws-chunked streaming signatures over plain HTTP.
func (s *Storage) UploadPart(ctx context.Context, key, uploadID string, n int, r io.Reader, size int64) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read part %d: %w", n, err)
	}
	md5sum := md5.Sum(buf)
	shasum := sha256.Sum256(buf)
	part, err := s.core.PutObjectPart(ctx, s.bucket, key, uploadID, n, bytes.NewReader(buf), size, minio.PutObjectPartOptions{
		Md5Base64: base64.StdEncoding.EncodeToString(md5sum[:]),
		Sha256Hex: hex.EncodeToString(shasum[:]),
		// The payload hash is signed directly, so the aws-chunked streaming signature is unnecessary.
		DisableContentSha256: true,
	})
	if err != nil {
		return "", err
	}
	return part.ETag, nil
}

func (s *Storage) CompleteMultipart(ctx context.Context, key, uploadID string, parts []domain.CompletedPart) error {
	cp := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		cp[i] = minio.CompletePart{PartNumber: p.Number, ETag: p.ETag}
	}
	_, err := s.core.CompleteMultipartUpload(ctx, s.bucket, key, uploadID, cp, minio.PutObjectOptions{})
	return err
}

func (s *Storage) AbortMultipart(ctx context.Context, key, uploadID string) error {
	return s.core.AbortMultipartUpload(ctx, s.bucket, key, uploadID)
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// GetObject is lazy; Stat surfaces a missing key now rather than on first Read.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		if isNotFound(minio.ToErrorResponse(err)) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return obj, nil
}

func (s *Storage) Copy(ctx context.Context, src, dst string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dst},
		minio.CopySrcOptions{Bucket: s.bucket, Object: src})
	return err
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

var _ domain.BlobStore = (*Storage)(nil)
