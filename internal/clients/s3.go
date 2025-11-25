package clients

import (
	"bytes"
	"context"
	"fmt"
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

	_, err := c.raw.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	})
	if err != nil {
		return "", fmt.Errorf("put object %q failed: %w", key, err)
	}

	return key, nil
}

func (c *S3Client) GetTemporaryURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if c.raw == nil {
		return "", fmt.Errorf("s3 client is nil")
	}

	u, err := c.raw.PresignedGetObject(ctx, c.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presign get object %q failed: %w", key, err)
	}

	return u.String(), nil
}
