package rustfs

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/braver-braver/lettuce"
)

func TestStorageErrorClassification(t *testing.T) {
	tests := []struct {
		code string
		want error
	}{
		{code: "NoSuchKey", want: lettuce.ErrNotFound},
		{code: "AccessDenied", want: lettuce.ErrPermission},
		{code: "InvalidRange", want: lettuce.ErrInvalid},
		{code: "PreconditionFailed", want: lettuce.ErrPreconditionFailed},
		{code: "ConditionalRequestConflict", want: lettuce.ErrConflict},
		{code: "SlowDown", want: lettuce.ErrUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			apiErr := &smithy.GenericAPIError{Code: tt.code, Message: "boom"}
			err := storageError("open", "a.txt", apiErr)

			if !errors.Is(err, tt.want) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tt.want)
			}

			var gotAPI *smithy.GenericAPIError
			if !errors.As(err, &gotAPI) {
				t.Fatalf("errors.As(%v, *smithy.GenericAPIError) = false", err)
			}
		})
	}
}
