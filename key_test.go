package lettuce

import (
	"errors"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "simple", key: "avatar.jpg"},
		{name: "nested", key: "people/123/avatar.jpg"},
		{name: "unicode", key: "people/123/照片.jpg"},
		{name: "empty", key: "", wantErr: true},
		{name: "absolute", key: "/people/123", wantErr: true},
		{name: "trailing slash", key: "people/123/", wantErr: true},
		{name: "empty segment", key: "people//123", wantErr: true},
		{name: "dot segment", key: "people/./123", wantErr: true},
		{name: "dot dot segment", key: "people/../123", wantErr: true},
		{name: "backslash", key: `people\123`, wantErr: true},
		{name: "nul", key: "people/\x00/123", wantErr: true},
		{name: "invalid utf8", key: string([]byte{0xff}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateKey(%q) error = %v, want ErrInvalid", tt.key, err)
			}
		})
	}
}

func TestValidatePrefix(t *testing.T) {
	tests := []struct {
		prefix  string
		wantErr bool
	}{
		{prefix: ""},
		{prefix: "people"},
		{prefix: "people/123"},
		{prefix: "people/123/"},
		{prefix: "/people", wantErr: true},
		{prefix: "people//123", wantErr: true},
		{prefix: "people/../123", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			err := ValidatePrefix(tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePrefix(%q) error = %v, wantErr %v", tt.prefix, err, tt.wantErr)
			}
		})
	}
}
