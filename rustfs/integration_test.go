//go:build integration

package rustfs

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/braver-braver/lettuce"
)

func TestIntegrationStore(t *testing.T) {
	endpoint := os.Getenv("RUSTFS_ENDPOINT")
	accessKey := os.Getenv("RUSTFS_ACCESS_KEY")
	secretKey := os.Getenv("RUSTFS_SECRET_KEY")
	if endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("RustFS integration environment is not configured")
	}

	const bucket = "lettuce-integration"
	const key = "smoke/hello.txt"

	store, err := New(t.Context(), Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	client, ok := store.client.(*s3.Client)
	if !ok {
		t.Fatalf("unexpected S3 client type %T", store.client)
	}

	if _, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
	})

	info, err := store.Put(t.Context(), key, strings.NewReader("hello world"), lettuce.PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"source": "integration"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if info.Size != 11 || info.ContentType != "text/plain" || info.Metadata["source"] != "integration" {
		t.Fatalf("Put() info = %#v", info)
	}

	obj, err := store.Open(t.Context(), key, lettuce.OpenOptions{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	data, err := io.ReadAll(obj)
	closeErr := obj.Close()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
	if got := string(data); got != "hello world" {
		t.Fatalf("Open() body = %q, want %q", got, "hello world")
	}

	ranged, err := store.Open(t.Context(), key, lettuce.OpenOptions{
		Range: &lettuce.ByteRange{Offset: 6, Length: 5},
	})
	if err != nil {
		t.Fatalf("Open(range) error = %v", err)
	}
	rangeData, err := io.ReadAll(ranged)
	rangeCloseErr := ranged.Close()
	if err != nil {
		t.Fatalf("ReadAll(range) error = %v", err)
	}
	if rangeCloseErr != nil {
		t.Fatalf("Close(range) error = %v", rangeCloseErr)
	}
	if got := string(rangeData); got != "world" {
		t.Fatalf("Open(range) body = %q, want %q", got, "world")
	}
	if ranged.Size != 11 {
		t.Fatalf("Open(range) Size = %d, want 11", ranged.Size)
	}

	var listed []string
	for item, err := range store.List(t.Context(), "smoke/", lettuce.ListOptions{}) {
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		listed = append(listed, item.Key)
	}
	if len(listed) != 1 || listed[0] != key {
		t.Fatalf("List() = %#v, want [%q]", listed, key)
	}

	if err := store.Delete(t.Context(), key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Stat(t.Context(), key); !errors.Is(err, lettuce.ErrNotFound) {
		t.Fatalf("Stat(deleted) error = %v, want ErrNotFound", err)
	}
}
