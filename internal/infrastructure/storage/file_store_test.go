package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_WriteRead(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	data := map[string]any{"key": "value", "num": float64(42)}
	err := fs.Write("test/record.json", data)
	require.NoError(t, err)

	var result map[string]any
	err = fs.Read("test/record.json", &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["num"])
}

func TestFileStore_NotFound(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	var result map[string]any
	err := fs.Read("nonexistent.json", &result)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileStore_Delete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	require.NoError(t, fs.Write("item.json", map[string]any{"x": 1}))

	var v map[string]any
	require.NoError(t, fs.Read("item.json", &v))

	require.NoError(t, fs.Delete("item.json"))

	err := fs.Read("item.json", &v)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileStore_Delete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)
	assert.NoError(t, fs.Delete("does-not-exist.json"))
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	require.NoError(t, fs.Write("items/a.json", map[string]any{"n": "a"}))
	require.NoError(t, fs.Write("items/b.json", map[string]any{"n": "b"}))
	require.NoError(t, fs.Write("items/c.txt", "not json"))

	paths, err := fs.List("items")
	require.NoError(t, err)
	assert.Len(t, paths, 2)
}

func TestFileStore_List_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	paths, err := fs.List("missing")
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestFileStore_AtomicWrite_NoPartialFile(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	large := make([]byte, 1<<20) // 1MB
	payload := map[string]any{"data": string(large)}
	require.NoError(t, fs.Write("large.json", payload))

	var result map[string]any
	require.NoError(t, fs.Read("large.json", &result))
	assert.NotEmpty(t, result["data"])
}
