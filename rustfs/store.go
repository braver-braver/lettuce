package rustfs

import (
	"context"
	"fmt"
	"io"
	"iter"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/braver-braver/lettuce"
)

var _ lettuce.Store = (*Store)(nil)

type objectAPI interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type uploadAPI interface {
	UploadObject(context.Context, *transfermanager.UploadObjectInput, ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

// Store stores objects in one RustFS bucket.
type Store struct {
	bucket  string
	client  objectAPI
	uploads uploadAPI
}

func newStore(bucket string, client objectAPI, uploads uploadAPI) *Store {
	return &Store{
		bucket:  bucket,
		client:  client,
		uploads: uploads,
	}
}

// Put writes or replaces an object.
func (s *Store) Put(ctx context.Context, key string, src io.Reader, opts lettuce.PutOptions) (lettuce.ObjectInfo, error) {
	if err := validateKey("put", key); err != nil {
		return lettuce.ObjectInfo{}, err
	}
	if src == nil {
		return lettuce.ObjectInfo{}, invalidError("put", key, "source must not be nil")
	}

	input := &transfermanager.UploadObjectInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		Body:     src,
		Metadata: maps.Clone(opts.Metadata),
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}

	if _, err := s.uploads.UploadObject(ctx, input); err != nil {
		return lettuce.ObjectInfo{}, storageError("put", key, err)
	}

	return s.Stat(ctx, key)
}

// Open opens an object for sequential reading.
func (s *Store) Open(ctx context.Context, key string, opts lettuce.OpenOptions) (*lettuce.Object, error) {
	if err := validateKey("open", key); err != nil {
		return nil, err
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if opts.Range != nil {
		header, err := rangeHeader(*opts.Range)
		if err != nil {
			return nil, invalidError("open", key, err.Error())
		}
		input.Range = aws.String(header)
	}

	output, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, storageError("open", key, err)
	}
	if output.Body == nil {
		return nil, storageError("open", key, fmt.Errorf("empty response body: %w", lettuce.ErrUnavailable))
	}

	return &lettuce.Object{
		ObjectInfo: infoFromGet(key, output),
		Body:       output.Body,
	}, nil
}

// Stat returns object metadata without reading the body.
func (s *Store) Stat(ctx context.Context, key string) (lettuce.ObjectInfo, error) {
	if err := validateKey("stat", key); err != nil {
		return lettuce.ObjectInfo{}, err
	}

	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return lettuce.ObjectInfo{}, storageError("stat", key, err)
	}

	return infoFromHead(key, output), nil
}

// Delete removes an object. RustFS follows S3 semantics, where deleting an
// already-missing key succeeds.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := validateKey("delete", key); err != nil {
		return err
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return storageError("delete", key, err)
	}
	return nil
}

// List returns objects recursively below prefix.
func (s *Store) List(ctx context.Context, prefix string, opts lettuce.ListOptions) iter.Seq2[lettuce.ObjectInfo, error] {
	return func(yield func(lettuce.ObjectInfo, error) bool) {
		if err := lettuce.ValidatePrefix(prefix); err != nil {
			yield(lettuce.ObjectInfo{}, storageError("list", prefix, err))
			return
		}
		if opts.StartAfter != "" {
			if err := lettuce.ValidateKey(opts.StartAfter); err != nil {
				yield(lettuce.ObjectInfo{}, storageError("list", opts.StartAfter, err))
				return
			}
		}
		if opts.Limit < 0 {
			yield(lettuce.ObjectInfo{}, invalidError("list", prefix, "limit must not be negative"))
			return
		}

		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
		}
		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}
		if opts.StartAfter != "" {
			input.StartAfter = aws.String(opts.StartAfter)
		}

		remaining := opts.Limit
		seenTokens := make(map[string]struct{})

		for {
			if opts.Limit > 0 {
				pageSize := min(remaining, 1000)
				maxKeys := int32(pageSize)
				input.MaxKeys = &maxKeys
			}

			output, err := s.client.ListObjectsV2(ctx, input)
			if err != nil {
				yield(lettuce.ObjectInfo{}, storageError("list", prefix, err))
				return
			}

			for _, object := range output.Contents {
				if object.Key == nil {
					continue
				}
				if !yield(infoFromListObject(object), nil) {
					return
				}
				if opts.Limit > 0 {
					remaining--
					if remaining == 0 {
						return
					}
				}
			}

			if !aws.ToBool(output.IsTruncated) {
				return
			}

			token := aws.ToString(output.NextContinuationToken)
			if token == "" {
				yield(lettuce.ObjectInfo{}, storageError(
					"list",
					prefix,
					fmt.Errorf("truncated response without continuation token: %w", lettuce.ErrUnavailable),
				))
				return
			}
			if _, exists := seenTokens[token]; exists {
				yield(lettuce.ObjectInfo{}, storageError(
					"list",
					prefix,
					fmt.Errorf("repeated continuation token: %w", lettuce.ErrUnavailable),
				))
				return
			}
			seenTokens[token] = struct{}{}
			input.ContinuationToken = aws.String(token)
			input.StartAfter = nil
		}
	}
}

func validateKey(op, key string) error {
	if err := lettuce.ValidateKey(key); err != nil {
		return storageError(op, key, err)
	}
	return nil
}

func invalidError(op, key, message string) error {
	return &lettuce.Error{
		Provider: providerName,
		Op:       op,
		Key:      key,
		Err:      fmt.Errorf("%s: %w", message, lettuce.ErrInvalid),
	}
}

func rangeHeader(r lettuce.ByteRange) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	if r.Length == 0 {
		return fmt.Sprintf("bytes=%d-", r.Offset), nil
	}
	if r.Offset > math.MaxInt64-(r.Length-1) {
		return "", fmt.Errorf("byte range overflows int64: %w", lettuce.ErrInvalid)
	}
	return fmt.Sprintf("bytes=%d-%d", r.Offset, r.Offset+r.Length-1), nil
}

func infoFromHead(key string, output *s3.HeadObjectOutput) lettuce.ObjectInfo {
	return lettuce.ObjectInfo{
		Key:         key,
		Size:        int64Value(output.ContentLength),
		ModTime:     timeValue(output.LastModified),
		ETag:        aws.ToString(output.ETag),
		ContentType: aws.ToString(output.ContentType),
		Metadata:    maps.Clone(output.Metadata),
	}
}

func infoFromGet(key string, output *s3.GetObjectOutput) lettuce.ObjectInfo {
	return lettuce.ObjectInfo{
		Key:         key,
		Size:        responseObjectSize(output.ContentRange, output.ContentLength),
		ModTime:     timeValue(output.LastModified),
		ETag:        aws.ToString(output.ETag),
		ContentType: aws.ToString(output.ContentType),
		Metadata:    maps.Clone(output.Metadata),
	}
}

func infoFromListObject(object s3types.Object) lettuce.ObjectInfo {
	return lettuce.ObjectInfo{
		Key:     aws.ToString(object.Key),
		Size:    int64Value(object.Size),
		ModTime: timeValue(object.LastModified),
		ETag:    aws.ToString(object.ETag),
	}
}

func responseObjectSize(contentRange *string, contentLength *int64) int64 {
	if contentRange != nil {
		if slash := strings.LastIndexByte(*contentRange, '/'); slash >= 0 {
			total := strings.TrimSpace((*contentRange)[slash+1:])
			if total != "" && total != "*" {
				if size, err := strconv.ParseInt(total, 10, 64); err == nil && size >= 0 {
					return size
				}
			}
		}
	}
	return int64Value(contentLength)
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
