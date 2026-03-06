package backup

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeS3Server returns an httptest.Server that records PutObject paths and
// responds 200 OK to every request.
func fakeS3Server(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	return srv, &paths
}

func TestS3Backup_Name(t *testing.T) {
	b, err := NewS3Backup(S3Config{Bucket: "b", Region: "us-east-1"})
	require.NoError(t, err)
	assert.Equal(t, "s3", b.Name())
}

func TestS3Backup_EmptyDir(t *testing.T) {
	srv, paths := fakeS3Server(t)
	defer srv.Close()

	dir := t.TempDir()
	b, err := NewS3Backup(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Prefix:    "backup",
		Endpoint:  srv.URL,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	err = b.Backup(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, *paths, "no files should be uploaded for empty dir")
}

func TestS3Backup_UploadsFiles(t *testing.T) {
	srv, paths := fakeS3Server(t)
	defer srv.Close()

	dir := t.TempDir()
	// Create a nested file structure.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "stores", "default"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stores", "default", "data.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "principals.json"), []byte(`[]`), 0o600))

	b, err := NewS3Backup(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Prefix:    "backup",
		Endpoint:  srv.URL,
		AccessKey: "test",
		SecretKey: "test",
	})
	require.NoError(t, err)

	err = b.Backup(context.Background(), dir)
	require.NoError(t, err)

	require.Len(t, *paths, 2, "expected 2 files uploaded")
	for _, p := range *paths {
		assert.True(t, strings.HasPrefix(p, "/test-bucket/backup/"),
			"key should start with /test-bucket/backup/, got %s", p)
	}
}

func TestS3Backup_CorrectKeys(t *testing.T) {
	srv, paths := fakeS3Server(t)
	defer srv.Close()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0o600))

	b, err := NewS3Backup(S3Config{
		Bucket:    "mybucket",
		Region:    "eu-west-1",
		Prefix:    "myprefix",
		Endpoint:  srv.URL,
		AccessKey: "ak",
		SecretKey: "sk",
	})
	require.NoError(t, err)

	require.NoError(t, b.Backup(context.Background(), dir))
	require.Len(t, *paths, 1)
	assert.Equal(t, "/mybucket/myprefix/config.json", (*paths)[0])
}

func TestS3Backup_NoPrefix(t *testing.T) {
	srv, paths := fakeS3Server(t)
	defer srv.Close()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o600))

	b, err := NewS3Backup(S3Config{
		Bucket:   "bucket",
		Region:   "us-east-1",
		Endpoint: srv.URL,
		// No prefix, no credentials — path-style key should be /bucket/file.txt.
		AccessKey: "ak",
		SecretKey: "sk",
	})
	require.NoError(t, err)

	require.NoError(t, b.Backup(context.Background(), dir))
	require.Len(t, *paths, 1)
	assert.Equal(t, "/bucket/file.txt", (*paths)[0])
}
