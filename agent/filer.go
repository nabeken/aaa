package agent

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/nabeken/aws-go-s3/bucket/option"
)

var (
	// ErrFileNotFound is the error returned by Filer interface when file is not found.
	ErrFileNotFound = errors.New("aaa: file not found")
)

// Filer interface represents a file storage layer for AAA.
type Filer interface {
	WriteFile(string, []byte) error
	ReadFile(string) ([]byte, error)
	Join(elem ...string) string
	Split(elem string) []string
	ListDir(string) ([]string, error)
}

// OSFiler implements Filer interface backed by *os.File.
type OSFiler struct {
	// BaseDir is prepended into given filename.
	BaseDir string
}

// WriteFile writes data to filename. WriteFile also creates any directories by using
// os.MkdirAll.
func (f *OSFiler) WriteFile(filename string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(f.Join(f.BaseDir, filename)), 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(f.Join(f.BaseDir, filename), data, 0600)
}

func (f *OSFiler) ReadFile(filename string) ([]byte, error) {
	data, err := ioutil.ReadFile(f.Join(f.BaseDir, filename))
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
func (f *OSFiler) ListDir(prefix string) ([]string, error) {
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

func (f *S3Filer) WriteFile(key string, data []byte) error {
	cl := int64(len(data))
	_, err := f.bucket.PutObject(
		key,
		bytes.NewReader(data),
		option.SSEKMSKeyID(f.keyId),
		option.ContentLength(cl),
		option.ACLPrivate(),
	)
	return err
}

func (s *S3Filer) ReadFile(key string) ([]byte, error) {
	object, err := s.bucket.GetObject(key)
	if err != nil {
		s3err, ok := err.(awserr.RequestFailure)
		if ok && s3err.StatusCode() == http.StatusNotFound {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	defer object.Body.Close()
	return ioutil.ReadAll(object.Body)
}

func (s *S3Filer) ListDir(prefix string) ([]string, error) {
	resp, err := s.bucket.ListObjects(
		prefix+"/",
		option.ListDelimiter("/"),
	)
	if err != nil {
		return nil, err
	}

	dirs := make([]string, len(resp.CommonPrefixes))
	for i, v := range resp.CommonPrefixes {
		dir := aws.StringValue(v.Prefix)

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
