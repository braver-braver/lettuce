package rustfs

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/braver-braver/lettuce"
)

type fakeObjectAPI struct {
	getFn    func(context.Context, *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	headFn   func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	deleteFn func(context.Context, *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	listFn   func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

func (f *fakeObjectAPI) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return f.getFn(ctx, in)
}

func (f *fakeObjectAPI) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return f.headFn(ctx, in)
}

func (f *fakeObjectAPI) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return f.deleteFn(ctx, in)
}

func (f *fakeObjectAPI) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return f.listFn(ctx, in)
}

type fakeUploadAPI struct {
	uploadFn func(context.Context, *transfermanager.UploadObjectInput) (*transfermanager.UploadObjectOutput, error)
}

func (f *fakeUploadAPI) UploadObject(ctx context.Context, in *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	return f.uploadFn(ctx, in)
}

func TestRangeHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   lettuce.ByteRange
		want    string
		wantErr bool
	}{
		{name: "to end", input: lettuce.ByteRange{Offset: 5}, want: "bytes=5-"},
		{name: "bounded", input: lettuce.ByteRange{Offset: 5, Length: 10}, want: "bytes=5-14"},
		{name: "negative offset", input: lettuce.ByteRange{Offset: -1}, wantErr: true},
		{name: "negative length", input: lettuce.ByteRange{Length: -1}, wantErr: true},
		{name: "overflow", input: lettuce.ByteRange{Offset: int64(^uint64(0) >> 1), Length: 2}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rangeHeader(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("rangeHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, lettuce.ErrInvalid) {
					t.Fatalf("rangeHeader() error = %v, want ErrInvalid", err)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("rangeHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPut(t *testing.T) {
	modTime := time.Unix(123, 0)
	var uploaded *transfermanager.UploadObjectInput

	client := &fakeObjectAPI{
		headFn: func(_ context.Context, in *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
			if aws.ToString(in.Key) != "people/1/avatar.jpg" {
				t.Fatalf("HeadObject key = %q", aws.ToString(in.Key))
			}
			return &s3.HeadObjectOutput{
				ContentLength: aws.Int64(3),
				ContentType:   aws.String("image/jpeg"),
				ETag:          aws.String("etag"),
				LastModified:  &modTime,
				Metadata:      map[string]string{"owner": "1"},
			}, nil
		},
	}
	uploads := &fakeUploadAPI{
		uploadFn: func(_ context.Context, in *transfermanager.UploadObjectInput) (*transfermanager.UploadObjectOutput, error) {
			uploaded = in
			return &transfermanager.UploadObjectOutput{}, nil
		},
	}

	store := newStore("files", client, uploads)
	info, err := store.Put(t.Context(), "people/1/avatar.jpg", strings.NewReader("abc"), lettuce.PutOptions{
		ContentType: "image/jpeg",
		Metadata:    map[string]string{"owner": "1"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if uploaded == nil {
		t.Fatal("UploadObject was not called")
	}
	if aws.ToString(uploaded.Bucket) != "files" || aws.ToString(uploaded.Key) != "people/1/avatar.jpg" {
		t.Fatalf("UploadObject target = %q/%q", aws.ToString(uploaded.Bucket), aws.ToString(uploaded.Key))
	}
	if aws.ToString(uploaded.ContentType) != "image/jpeg" || uploaded.Metadata["owner"] != "1" {
		t.Fatalf("UploadObject metadata = %#v content-type = %q", uploaded.Metadata, aws.ToString(uploaded.ContentType))
	}
	if info.Size != 3 || info.ETag != "etag" || info.ContentType != "image/jpeg" {
		t.Fatalf("Put() info = %#v", info)
	}
}

func TestOpenRangeUsesFullObjectSize(t *testing.T) {
	var gotRange string
	client := &fakeObjectAPI{
		getFn: func(_ context.Context, in *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
			gotRange = aws.ToString(in.Range)
			return &s3.GetObjectOutput{
				Body:          io.NopCloser(strings.NewReader("world")),
				ContentLength: aws.Int64(5),
				ContentRange:  aws.String("bytes 5-9/20"),
				ContentType:   aws.String("text/plain"),
				ETag:          aws.String("etag"),
			}, nil
		},
	}

	store := newStore("files", client, nil)
	obj, err := store.Open(t.Context(), "greeting.txt", lettuce.OpenOptions{
		Range: &lettuce.ByteRange{Offset: 5, Length: 5},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer obj.Close()

	if gotRange != "bytes=5-9" {
		t.Fatalf("Range = %q, want %q", gotRange, "bytes=5-9")
	}
	if obj.Size != 20 {
		t.Fatalf("Object.Size = %d, want 20", obj.Size)
	}
}

func TestListPaginates(t *testing.T) {
	calls := 0
	client := &fakeObjectAPI{
		listFn: func(_ context.Context, in *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			calls++
			switch calls {
			case 1:
				if aws.ToString(in.Prefix) != "people/" {
					t.Fatalf("Prefix = %q", aws.ToString(in.Prefix))
				}
				return &s3.ListObjectsV2Output{
					Contents: []s3types.Object{
						{Key: aws.String("people/a"), Size: aws.Int64(1)},
					},
					IsTruncated:           aws.Bool(true),
					NextContinuationToken: aws.String("next"),
				}, nil
			case 2:
				if aws.ToString(in.ContinuationToken) != "next" {
					t.Fatalf("ContinuationToken = %q", aws.ToString(in.ContinuationToken))
				}
				return &s3.ListObjectsV2Output{
					Contents: []s3types.Object{
						{Key: aws.String("people/b"), Size: aws.Int64(2)},
					},
				}, nil
			default:
				t.Fatalf("unexpected ListObjectsV2 call %d", calls)
				return nil, nil
			}
		},
	}

	store := newStore("files", client, nil)
	var keys []string
	for info, err := range store.List(t.Context(), "people/", lettuce.ListOptions{}) {
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		keys = append(keys, info.Key)
	}

	if got, want := strings.Join(keys, ","), "people/a,people/b"; got != want {
		t.Fatalf("List() keys = %q, want %q", got, want)
	}
	if calls != 2 {
		t.Fatalf("ListObjectsV2 calls = %d, want 2", calls)
	}
}
