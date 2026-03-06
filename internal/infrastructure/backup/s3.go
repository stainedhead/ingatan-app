package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds S3 backup configuration.
type S3Config struct {
	// Bucket is the S3 bucket name.
	Bucket string
	// Region is the AWS region (e.g., "us-east-1").
	Region string
	// Prefix is an optional path prefix for all uploaded keys.
	Prefix string
	// Endpoint overrides the default S3 endpoint (for MinIO or other S3-compatible services).
	Endpoint string
	// AccessKey is the AWS access key ID (optional; falls back to default credential chain).
	AccessKey string
	// SecretKey is the AWS secret access key (optional; falls back to default credential chain).
	SecretKey string
}

// S3Backup backs up the data directory to an S3-compatible bucket.
// Implements Backuper.
type S3Backup struct {
	cfg    S3Config
	client *s3.Client
}

// NewS3Backup creates a new S3Backup with the provided configuration.
func NewS3Backup(cfg S3Config) (*S3Backup, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		loadOpts = append(loadOpts,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
			),
		)
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3 backup: load aws config: %w", err)
	}

	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return &S3Backup{cfg: cfg, client: client}, nil
}

// Backup walks dataDir recursively and uploads each file to the S3 bucket.
// The S3 key for each file is: [prefix/]relPath (relative to dataDir).
func (b *S3Backup) Backup(ctx context.Context, dataDir string) error {
	return filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("s3 backup: walk %s: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return fmt.Errorf("s3 backup: relative path for %s: %w", path, err)
		}

		key := buildKey(b.cfg.Prefix, rel)
		if err := b.uploadFile(ctx, path, key); err != nil {
			return fmt.Errorf("s3 backup: upload %s: %w", rel, err)
		}
		return nil
	})
}

// Name returns "s3".
func (b *S3Backup) Name() string { return "s3" }

// uploadFile opens the file at localPath and uploads it to S3 under key.
func (b *S3Backup) uploadFile(ctx context.Context, localPath, key string) error {
	f, err := os.Open(localPath) //nolint:gosec // path is constructed from filepath.WalkDir, not user input
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(b.cfg.Bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(fi.Size()),
	})
	return err
}

// buildKey joins a prefix and a relative path into an S3 key.
// If prefix is empty, the relative path is used directly.
func buildKey(prefix, rel string) string {
	// Normalize to forward slashes (S3 always uses /).
	rel = filepath.ToSlash(rel)
	if prefix == "" {
		return rel
	}
	return prefix + "/" + rel
}
