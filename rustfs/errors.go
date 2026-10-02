package rustfs

import (
	"errors"

	"github.com/aws/smithy-go"
	"github.com/braver-braver/lettuce"
)

const providerName = "rustfs"

type classifiedError struct {
	kind error
	err  error
}

func (e *classifiedError) Error() string {
	return e.err.Error()
}

func (e *classifiedError) Unwrap() error {
	return e.err
}

func (e *classifiedError) Is(target error) bool {
	return target == e.kind || errors.Is(e.err, target)
}

func storageError(op, key string, err error) error {
	if err == nil {
		return nil
	}

	if kind := errorKind(err); kind != nil && !errors.Is(err, kind) {
		err = &classifiedError{kind: kind, err: err}
	}

	return &lettuce.Error{
		Provider: providerName,
		Op:       op,
		Key:      key,
		Err:      err,
	}
}

func errorKind(err error) error {
	if err == nil {
		return nil
	}

	var statusError interface {
		HTTPStatusCode() int
	}
	if errors.As(err, &statusError) {
		switch statusError.HTTPStatusCode() {
		case 400:
			return lettuce.ErrInvalid
		case 401, 403:
			return lettuce.ErrPermission
		case 404:
			return lettuce.ErrNotFound
		case 409:
			return lettuce.ErrConflict
		case 412:
			return lettuce.ErrPreconditionFailed
		case 429, 500, 502, 503, 504:
			return lettuce.ErrUnavailable
		}
	}

	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return nil
	}

	switch apiError.ErrorCode() {
	case "NoSuchKey", "NoSuchBucket", "NoSuchObject", "NotFound":
		return lettuce.ErrNotFound
	case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "ExpiredToken", "InvalidToken":
		return lettuce.ErrPermission
	case "InvalidArgument", "InvalidRequest", "InvalidRange", "MalformedXML", "EntityTooLarge":
		return lettuce.ErrInvalid
	case "PreconditionFailed":
		return lettuce.ErrPreconditionFailed
	case "ConditionalRequestConflict", "Conflict":
		return lettuce.ErrConflict
	case "SlowDown", "RequestTimeout", "ServiceUnavailable", "InternalError":
		return lettuce.ErrUnavailable
	default:
		return nil
	}
}
