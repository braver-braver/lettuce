package rustfs

import (
	"errors"
	"net/http"
	"testing"

	"github.com/braver-braver/lettuce"
)

func TestNormalizeConfig(t *testing.T) {
	client := &http.Client{}
	got, err := normalizeConfig(Config{
		Endpoint:  "http://localhost:9000/",
		Bucket:    "files",
		AccessKey:  "access",
		SecretKey:  "secret",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("normalizeConfig() error = %v", err)
	}

	if got.Endpoint != "http://localhost:9000" {
		t.Fatalf("Endpoint = %q, want %q", got.Endpoint, "http://localhost:9000")
	}
	if got.Region != defaultRegion {
		t.Fatalf("Region = %q, want %q", got.Region, defaultRegion)
	}
	if got.HTTPClient != client {
		t.Fatal("HTTPClient was not preserved")
	}
}

func TestNormalizeConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing endpoint", cfg: Config{Bucket: "files", AccessKey: "a", SecretKey: "s"}},
		{name: "bad scheme", cfg: Config{Endpoint: "ftp://localhost", Bucket: "files", AccessKey: "a", SecretKey: "s"}},
		{name: "endpoint path", cfg: Config{Endpoint: "http://localhost:9000/api", Bucket: "files", AccessKey: "a", SecretKey: "s"}},
		{name: "endpoint credentials", cfg: Config{Endpoint: "http://user:pass@localhost:9000", Bucket: "files", AccessKey: "a", SecretKey: "s"}},
		{name: "missing bucket", cfg: Config{Endpoint: "http://localhost:9000", AccessKey: "a", SecretKey: "s"}},
		{name: "bucket path", cfg: Config{Endpoint: "http://localhost:9000", Bucket: "a/b", AccessKey: "a", SecretKey: "s"}},
		{name: "missing access key", cfg: Config{Endpoint: "http://localhost:9000", Bucket: "files", SecretKey: "s"}},
		{name: "missing secret key", cfg: Config{Endpoint: "http://localhost:9000", Bucket: "files", AccessKey: "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeConfig(tt.cfg)
			if !errors.Is(err, lettuce.ErrInvalid) {
				t.Fatalf("normalizeConfig() error = %v, want ErrInvalid", err)
			}
		})
	}
}
