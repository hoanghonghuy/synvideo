package s3storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	Timeout         time.Duration
}

func (c Config) Validate() error {
	endpoint := strings.TrimSpace(c.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return errors.New("storage endpoint must be an http(s) URL without path, query, fragment or credentials")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return errors.New("storage bucket is required")
	}
	if strings.TrimSpace(c.AccessKeyID) == "" || c.SecretAccessKey == "" {
		return errors.New("storage credentials are required")
	}
	if c.Timeout < 0 {
		return errors.New("storage timeout must not be negative")
	}
	return nil
}

type Storage struct {
	client  *minio.Client
	bucket  string
	timeout time.Duration
}

var _ mediaasset.ObjectStorage = (*Storage)(nil)

func New(cfg Config) (*Storage, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(strings.TrimSpace(cfg.Endpoint))
	bucketLookup := minio.BucketLookupAuto
	if cfg.UsePathStyle {
		bucketLookup = minio.BucketLookupPath
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure:       parsed.Scheme == "https",
		Region:       cfg.Region,
		BucketLookup: bucketLookup,
		MaxRetries:   1,
	})
	if err != nil {
		return nil, errors.New("storage client configuration failed")
	}
	return &Storage{client: client, bucket: strings.TrimSpace(cfg.Bucket), timeout: cfg.Timeout}, nil
}

func (s *Storage) Put(ctx context.Context, input mediaasset.PutObjectInput) (mediaasset.ObjectInfo, error) {
	if err := mediaasset.ValidateObjectKey(input.Key); err != nil {
		return mediaasset.ObjectInfo{}, fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	if input.Body == nil {
		return mediaasset.ObjectInfo{}, fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	info, err := s.client.PutObject(operationCtx, s.bucket, input.Key, input.Body, -1, minio.PutObjectOptions{ContentType: input.ContentType})
	if err != nil {
		return mediaasset.ObjectInfo{}, mapError(err)
	}
	return mediaasset.ObjectInfo{Key: input.Key, Size: info.Size, ContentType: input.ContentType}, nil
}

func (s *Storage) Stat(ctx context.Context, key string) (mediaasset.ObjectInfo, error) {
	if err := mediaasset.ValidateObjectKey(key); err != nil {
		return mediaasset.ObjectInfo{}, fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	info, err := s.client.StatObject(operationCtx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return mediaasset.ObjectInfo{}, mapError(err)
	}
	return mediaasset.ObjectInfo{Key: key, Size: info.Size, ContentType: info.ContentType}, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if _, err := s.Stat(ctx, key); err != nil {
		return nil, err
	}
	operationCtx, cancel := s.operationContext(ctx)
	object, err := s.client.GetObject(operationCtx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		cancel()
		return nil, mapError(err)
	}
	return &readCloser{ReadCloser: object, cancel: cancel}, nil
}

func (s *Storage) OpenRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || length < 1 {
		return nil, fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	info, err := s.Stat(ctx, key)
	if err != nil {
		return nil, err
	}
	if offset >= info.Size || length > info.Size-offset {
		return nil, fmt.Errorf("%w", mediaasset.ErrObjectNotFound)
	}
	operationCtx, cancel := s.operationContext(ctx)
	options := minio.GetObjectOptions{}
	if err := options.SetRange(offset, offset+length-1); err != nil {
		cancel()
		return nil, fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	object, err := s.client.GetObject(operationCtx, s.bucket, key, options)
	if err != nil {
		cancel()
		return nil, mapError(err)
	}
	return &readCloser{ReadCloser: object, cancel: cancel}, nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := mediaasset.ValidateObjectKey(key); err != nil {
		return fmt.Errorf("%w", mediaasset.ErrStorageFailed)
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.client.RemoveObject(operationCtx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return mapError(err)
	}
	return nil
}

// EnsureBucket is intended for local development and integration setup only.
func (s *Storage) EnsureBucket(ctx context.Context) error {
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.client.MakeBucket(operationCtx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code != "BucketAlreadyOwnedByYou" && response.Code != "BucketAlreadyExists" && response.StatusCode != 409 {
			return mapError(err)
		}
	}
	return nil
}

type readCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *readCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

func (s *Storage) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.timeout)
}

func mapError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, mediaasset.ErrTooLarge) {
		return mediaasset.ErrTooLarge
	}
	response := minio.ToErrorResponse(err)
	if response.StatusCode == 404 || response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.Code == "NoSuchBucket" {
		return fmt.Errorf("%w", mediaasset.ErrObjectNotFound)
	}
	return fmt.Errorf("%w", mediaasset.ErrStorageFailed)
}
