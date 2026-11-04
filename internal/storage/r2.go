package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"adelson-backend/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2 struct {
	bucket    string
	mediaBase string
	client    *s3.Client
	presigner *s3.PresignClient
}

func NewR2(cfg config.Config) (*R2, error) {
	r := &R2{bucket: cfg.R2Bucket, mediaBase: strings.TrimRight(cfg.MediaBaseURL, "/")}
	if cfg.R2AccountID == "" || cfg.R2AccessKeyID == "" || cfg.R2SecretKey == "" || cfg.R2Bucket == "" {
		return r, nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretKey, "")),
	)
	if err != nil {
		return r, fmt.Errorf("load r2 config: %w", err)
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	r.client = client
	r.presigner = s3.NewPresignClient(client)
	return r, nil
}

func (r *R2) Enabled() bool {
	return r != nil && r.presigner != nil && r.bucket != ""
}

func (r *R2) PublicURL(key string) string {
	if r == nil || r.mediaBase == "" {
		return ""
	}
	return r.mediaBase + "/" + strings.TrimLeft(key, "/")
}

func (r *R2) PresignPut(ctx context.Context, key, contentType string, expires time.Duration) (string, error) {
	if !r.Enabled() {
		return "", fmt.Errorf("r2 is not configured")
	}
	request, err := r.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return request.URL, nil
}

func (r *R2) Delete(ctx context.Context, key string) error {
	if !r.Enabled() || key == "" {
		return nil
	}
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("delete r2 object: %w", err)
	}
	return nil
}

func KeyFromPublicURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimLeft(parsed.Path, "/")
}
