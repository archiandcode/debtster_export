package clients

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	Region          string
	Prefix          string
}

type S3Client struct {
	raw    *minio.Client
	bucket string
	prefix string
}

func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %w", err)
	}

	return &S3Client{
		raw:    client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func (c *S3Client) UploadXLSX(ctx context.Context, fileName string, data []byte) (string, error) {
	if c.raw == nil {
		return "", fmt.Errorf("s3 client is nil")
	}

	key := c.prefix + fileName

	reader := bytes.NewReader(data)
	size := int64(len(data))

	// retry logic for transient failures
	attempts := 3
	backoff := 500 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		if _, err := c.raw.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
			ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		}); err != nil {
			lastErr = err
			log.Printf("s3: put object attempt %d/%d failed for key=%s: %v", attempt, attempts, key, err)
			if attempt < attempts {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
					continue
				}
			}
		} else {
			return key, nil
		}
	}

	return "", fmt.Errorf("put object %q failed after %d attempts: %w", key, attempts, lastErr)
}

func (c *S3Client) GetTemporaryURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if c.raw == nil {
		return "", fmt.Errorf("s3 client is nil")
	}

	// retries for presign (sometimes transient network errors or minio hiccups)
	attempts := 3
	backoff := 300 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		if u, err := c.raw.PresignedGetObject(ctx, c.bucket, key, ttl, nil); err != nil {
			lastErr = err
			log.Printf("s3: presign attempt %d/%d failed for key=%s: %v", attempt, attempts, key, err)
			if attempt < attempts {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
					continue
				}
			}
		} else {
			return u.String(), nil
		}
	}

	return "", fmt.Errorf("presign get object %q failed after %d attempts: %w", key, attempts, lastErr)
}
