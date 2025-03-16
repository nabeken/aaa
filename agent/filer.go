package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nabeken/aws-go-s3/v2/bucket"
	"github.com/nabeken/aws-go-s3/v2/bucket/option"
)

var (
	// ErrFileNotFound is the error returned by Filer interface when file is not found.
	ErrFileNotFound = errors.New("aaa: file not found")
)

// Filer interface represents a file storage layer for AAA.
type Filer interface {
	WriteFile(context.Context, string, []byte) error
	ReadFile(context.Context, string) ([]byte, error)
	ListDir(context.Context, string) ([]string, error)
	Join(elem ...string) string
	Split(elem string) []string
}

// OSFiler implements Filer interface backed by *os.File.
type OSFiler struct {
	// BaseDir is prepended into given filename.
	BaseDir string
}

// WriteFile writes data to filename. WriteFile also creates any directories by using
// os.MkdirAll.
func (f *OSFiler) WriteFile(_ context.Context, filename string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(f.Join(f.BaseDir, filename)), 0700); err != nil {
		return err
	}

	return os.WriteFile(f.Join(f.BaseDir, filename), data, 0600)
}

func (f *OSFiler) ReadFile(_ context.Context, filename string) ([]byte, error) {
	data, err := os.ReadFile(f.Join(f.BaseDir, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}

		return nil, err
	}

	return data, err
}

// ListDir returns directories that has the given prefix.
// See https://golang.org/pkg/os/#File.Readdirnames (n <= 0)
func (f *OSFiler) ListDir(_ context.Context, prefix string) ([]string, error) {
	fi, err := os.Open(f.Join(f.BaseDir, prefix))
	if err != nil {
		return nil, err
	}

	return fi.Readdirnames(-1)
}

func (s *OSFiler) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (s *OSFiler) Split(path string) []string {
	return strings.Split(path, string(os.PathSeparator))
}

type S3Filer struct {
	bucket *bucket.Bucket
	keyId  string
}

func NewS3Filer(bucket *bucket.Bucket, keyId string) *S3Filer {
	return &S3Filer{
		bucket: bucket,
		keyId:  keyId,
	}
}

func (f *S3Filer) WriteFile(ctx context.Context, key string, data []byte) error {
	cl := int64(len(data))

	_, err := f.bucket.PutObject(
		ctx,
		key,
		bytes.NewReader(data),
		option.SSEKMSKeyID(f.keyId),
		option.ContentLength(cl),
		option.ACLPrivate(),
	)

	return err
}

func (s *S3Filer) ReadFile(ctx context.Context, key string) ([]byte, error) {
	object, err := s.bucket.GetObject(ctx, key)
	if err != nil {
		var notFoundErr *types.NoSuchKey
		if errors.As(err, &notFoundErr) {
			return nil, ErrFileNotFound
		}

		return nil, err
	}

	defer object.Body.Close()

	return io.ReadAll(object.Body)
}

func (s *S3Filer) ListDir(ctx context.Context, prefix string) ([]string, error) {
	resp, err := s.bucket.ListObjects(
		ctx,
		prefix+"/",
		option.ListDelimiter("/"),
	)
	if err != nil {
		return nil, err
	}

	dirs := make([]string, len(resp.CommonPrefixes))
	for i, v := range resp.CommonPrefixes {
		dir := aws.ToString(v.Prefix)

		// removing trailing '/'
		if dir[len(dir)-1] == '/' {
			dir = dir[:len(dir)-1]
		}

		dirs[i] = dir
	}

	return dirs, nil
}

func (s *S3Filer) Join(elem ...string) string {
	return strings.Join(elem, "/")
}

func (s *S3Filer) Split(prefix string) []string {
	return strings.Split(prefix, "/")
}
