package lettuce

import (
	"io"
	"strings"
	"testing"
)

func TestObjectReadAndClose(t *testing.T) {
	obj := &Object{Body: io.NopCloser(strings.NewReader("lettuce"))}
	got, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != "lettuce" {
		t.Fatalf("ReadAll() = %q, want %q", got, "lettuce")
	}
	if err := obj.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNilObjectClose(t *testing.T) {
	var obj *Object
	if err := obj.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
