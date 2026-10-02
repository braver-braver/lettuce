package lettuce

import (
	"context"
	"io"
	"iter"
)

// Store is the backend-neutral object storage contract implemented by Lettuce
// providers. Keys are relative, slash-separated object names rather than OS
// filesystem paths.
type Store interface {
	Put(ctx context.Context, key string, src io.Reader, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string, opts OpenOptions) (*Object, error)
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string, opts ListOptions) iter.Seq2[ObjectInfo, error]
}

// PutOptions controls object creation and replacement.
type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

// OpenOptions controls object reads.
type OpenOptions struct {
	Range *ByteRange
}

// ListOptions controls object listing. A zero value lists all objects below the
// supplied prefix.
type ListOptions struct {
	// Delimiter groups keys at the first occurrence after prefix. Use "/" for
	// directory-like listings. An empty delimiter returns a recursive listing.
	Delimiter string

	// StartAfter excludes keys less than or equal to this value.
	StartAfter string

	// Limit bounds the number of returned objects when greater than zero.
	Limit int
}

// ByteRange selects a byte range within an object. Length <= 0 means read from
// Offset through the end of the object.
type ByteRange struct {
	Offset int64
	Length int64
}

// Validate reports whether the byte range can be represented by all supported
// backends.
func (r ByteRange) Validate() error {
	if r.Offset < 0 {
		return &Error{Op: "range", Err: ErrInvalid}
	}
	return nil
}
