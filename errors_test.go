package lettuce

import (
	"errors"
	"testing"
)

func TestErrorUnwrap(t *testing.T) {
	err := &Error{
		Op:       "stat",
		Provider: "rustfs",
		Key:      "people/123/avatar.jpg",
		Err:      ErrNotFound,
	}

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is(%v, ErrNotFound) = false", err)
	}

	want := `rustfs stat "people/123/avatar.jpg": not found`
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
