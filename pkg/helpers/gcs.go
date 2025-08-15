package helpers

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"io"
)

// NewGCSClient creates a Google Cloud Storage client. If credsPath is empty, ADC is used.
func NewGCSClient(ctx context.Context, credsPath string) (*storage.Client, error) {
	if credsPath == "" {
		return storage.NewClient(ctx)
	}
	return storage.NewClient(ctx, option.WithCredentialsFile(credsPath))
}

// UploadObject uploads bytes from r into bucket/objectPath with the provided contentType
func UploadObject(ctx context.Context, client *storage.Client, bucket, objectPath, contentType string, r io.Reader) (string, error) {
	wc := client.Bucket(bucket).Object(objectPath).NewWriter(ctx)
	wc.ContentType = contentType
	wc.ChunkSize = 0 // disable chunking for small files
	if _, err := io.Copy(wc, r); err != nil {
		_ = wc.Close()
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}
	return PublicURL(bucket, objectPath), nil
}

// UploadImageToGCS is a small convenience wrapper that returns the public URL
func UploadImageToGCS(ctx context.Context, client *storage.Client, bucket, objectPath, contentType string, r io.Reader) (string, error) {
	return UploadObject(ctx, client, bucket, objectPath, contentType, r)
}

// PublicURL builds a public URL for an object (assuming public read access or signed URLs)
func PublicURL(bucket, objectPath string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, objectPath)
}
