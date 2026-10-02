package lettuce

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidateKey validates a portable object key. Lettuce deliberately rejects
// ambiguous path forms instead of normalizing them because normalization can
// change object identity across storage backends.
func ValidateKey(key string) error {
	if key == "" {
		return invalidKey(key, "key is empty")
	}
	if strings.HasSuffix(key, "/") {
		return invalidKey(key, "object key must not end with slash")
	}
	return validatePathLike(key, false)
}

// ValidatePrefix validates a listing prefix. The empty prefix and a single
// trailing slash are allowed.
func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	return validatePathLike(prefix, true)
}

func validatePathLike(value string, allowTrailingSlash bool) error {
	if !utf8.ValidString(value) {
		return invalidKey(value, "must be valid UTF-8")
	}
	if strings.HasPrefix(value, "/") {
		return invalidKey(value, "must be relative")
	}
	if strings.ContainsRune(value, '\x00') {
		return invalidKey(value, "must not contain NUL")
	}
	if strings.Contains(value, "\\") {
		return invalidKey(value, "must use forward slashes")
	}

	parts := strings.Split(value, "/")
	for i, part := range parts {
		if part == "" {
			if allowTrailingSlash && i == len(parts)-1 {
				continue
			}
			return invalidKey(value, "must not contain empty path segments")
		}
		if part == "." || part == ".." {
			return invalidKey(value, "must not contain dot path segments")
		}
	}
	return nil
}

func invalidKey(key, reason string) error {
	return &Error{
		Op:  "validate key",
		Key: key,
		Err: fmt.Errorf("%s: %w", reason, ErrInvalid),
	}
}
