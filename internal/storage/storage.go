package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client uploads files to an S3-compatible object store (MinIO locally;
// real S3 or R2 later, by changing only the endpoint and credentials).
type Client struct {
	s3     *s3.Client
	bucket string
}

// Config holds the connection details for the object store.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

// New connects to the object store and ensures the configured bucket
// exists, creating it if necessary.
func New(ctx context.Context, cfg Config) (*Client, error) {
	awsCfg := aws.Config{
		Region:      "us-east-1", // required by the SDK, unused by MinIO
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // MinIO requires path-style addressing
	})

	c := &Client{s3: s3Client, bucket: cfg.Bucket}

	if err := c.ensureBucket(ctx); err != nil {
		return nil, fmt.Errorf("ensuring bucket exists: %w", err)
	}

	return c, nil
}

func (c *Client) ensureBucket(ctx context.Context) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	if err == nil {
		return nil
	}

	_, err = c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.bucket)})
	if err != nil {
		return fmt.Errorf("creating bucket %s: %w", c.bucket, err)
	}
	return nil
}

// Upload satisfies worker.Uploader: it puts the file at localPath into the
// bucket under a key namespaced by jobID, and returns that key.
func (c *Client) Upload(ctx context.Context, jobID, localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	key := fmt.Sprintf("jobs/%s/%s", jobID, filepath.Base(localPath))

	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("uploading to %s: %w", key, err)
	}

	return key, nil
}
