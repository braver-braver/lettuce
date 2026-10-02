package lettuce

import (
	"io"
	"time"
)

// ObjectInfo describes an object independently of the backing storage system.
type ObjectInfo struct {
	Key         string
	Size        int64
	ModTime     time.Time
	ETag        string
	ContentType string
	Metadata    map[string]string
}

// Object is an opened object. It forwards io.Reader and io.Closer operations to
// Body so callers can use it directly as a stream.
type Object struct {
	ObjectInfo
	Body io.ReadCloser
}

// Read reads object data from Body.
func (o *Object) Read(p []byte) (int, error) {
	if o == nil || o.Body == nil {
		return 0, ErrInvalid
	}
	return o.Body.Read(p)
}

// Close closes the object body.
func (o *Object) Close() error {
	if o == nil || o.Body == nil {
		return nil
	}
	return o.Body.Close()
}
