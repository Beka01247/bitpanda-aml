package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type MinIOStorage struct {
	client     *minio.Client
	presign    *minio.Client
	bucketName string
	publicURL  *url.URL
	logger     *zap.SugaredLogger
}

func NewMinIOStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool, publicURL string, logger *zap.SugaredLogger) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	parsedPublicURL, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("invalid public minio url: %w", err)
	}
	if parsedPublicURL.Scheme == "" || parsedPublicURL.Host == "" {
		return nil, fmt.Errorf("invalid public minio url: scheme and host are required")
	}

	// presigning must use the same host that the browser will request.
	// otherwise the signature won't match because the `host` header is part of the signed request.
	publicSecure := parsedPublicURL.Scheme == "https"
	presignClient, err := minio.New(parsedPublicURL.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: publicSecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create presign minio client: %w", err)
	}

	storage := &MinIOStorage{
		client:     client,
		presign:    presignClient,
		bucketName: bucketName,
		publicURL:  parsedPublicURL,
		logger:     logger,
	}

	// ensure bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Infow("bucket created", "bucket", bucketName)
	}

	logger.Infow("minio storage initialized", "endpoint", endpoint, "bucket", bucketName)

	return storage, nil
}

func (s *MinIOStorage) Put(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	// build object key with date prefix
	now := time.Now().UTC()
	objectKey := fmt.Sprintf("reports/%d/%02d/%02d/%s", now.Year(), now.Month(), now.Day(), key)

	reader := bytes.NewReader(data)

	// set metadata with expiration timestamp
	expiresAt := now.Add(ttl)
	userMetadata := map[string]string{
		"expire-at": expiresAt.Format(time.RFC3339),
	}

	_, err := s.client.PutObject(ctx, s.bucketName, objectKey, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType:  "application/pdf",
		UserMetadata: userMetadata,
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	s.logger.Debugw("report stored", "key", objectKey, "size", len(data), "expires_at", expiresAt)

	return nil
}

func (s *MinIOStorage) Get(ctx context.Context, key string) ([]byte, error) {
	// try to find object in date-prefixed paths (check last 7 days)
	now := time.Now().UTC()

	for i := 0; i < 7; i++ {
		checkDate := now.AddDate(0, 0, -i)
		objectKey := fmt.Sprintf("reports/%d/%02d/%02d/%s", checkDate.Year(), checkDate.Month(), checkDate.Day(), key)

		obj, err := s.client.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
		if err != nil {
			continue
		}

		// check if expired
		objInfo, err := obj.Stat()
		if err != nil {
			obj.Close()
			continue
		}

		if expireAt, ok := objInfo.UserMetadata["X-Amz-Meta-Expire-At"]; ok {
			expireTime, err := time.Parse(time.RFC3339, expireAt)
			if err == nil && time.Now().UTC().After(expireTime) {
				obj.Close()
				return nil, fmt.Errorf("report expired")
			}
		}

		// read object data
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(obj); err != nil {
			obj.Close()
			return nil, fmt.Errorf("failed to read object: %w", err)
		}
		obj.Close()

		return buf.Bytes(), nil
	}

	return nil, fmt.Errorf("report not found")
}

func (s *MinIOStorage) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	// try to find object in date-prefixed paths (check last 7 days)
	now := time.Now().UTC()

	for i := 0; i < 7; i++ {
		checkDate := now.AddDate(0, 0, -i)
		objectKey := fmt.Sprintf("reports/%d/%02d/%02d/%s", checkDate.Year(), checkDate.Month(), checkDate.Day(), key)

		// check if object exists
		_, err := s.client.StatObject(ctx, s.bucketName, objectKey, minio.StatObjectOptions{})
		if err != nil {
			continue
		}

		// generate presigned URL. use response overrides so browsers open it as a PDF.
		params := make(url.Values)
		params.Set("response-content-type", "application/pdf")
		params.Set("response-content-disposition", fmt.Sprintf("inline; filename=%q", key))

		presigned, err := s.presign.PresignedGetObject(ctx, s.bucketName, objectKey, expires, params)
		if err != nil {
			return "", fmt.Errorf("failed to generate presigned url: %w", err)
		}
		return presigned.String(), nil
	}

	return "", fmt.Errorf("report not found")
}

func (s *MinIOStorage) CleanupExpired(ctx context.Context, now time.Time) (int, error) {
	count := 0

	// list objects with reports/ prefix
	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    "reports/",
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			s.logger.Warnw("error listing object", "error", object.Err)
			continue
		}

		// check expiration from metadata
		if expireAt, ok := object.UserMetadata["expire-at"]; ok {
			expireTime, err := time.Parse(time.RFC3339, expireAt)
			if err == nil && now.After(expireTime) {
				err := s.client.RemoveObject(ctx, s.bucketName, object.Key, minio.RemoveObjectOptions{})
				if err != nil {
					s.logger.Warnw("failed to remove expired object", "key", object.Key, "error", err)
				} else {
					count++
				}
			}
		}
	}

	if count > 0 {
		s.logger.Infow("expired reports cleaned", "count", count)
	}

	return count, nil
}

// starts a background cleanup loop
func (s *MinIOStorage) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("storage cleanup loop stopped")
				return
			case <-ticker.C:
				_, err := s.CleanupExpired(ctx, time.Now().UTC())
				if err != nil {
					s.logger.Errorw("storage cleanup failed", "error", err)
				}
			}
		}
	}()
}
