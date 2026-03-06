// Package storage provides file-based JSON persistence for ingatan.
package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when a requested file does not exist.
var ErrNotFound = errors.New("not found")

// FileStore provides atomic JSON read/write operations within a base directory.
// All paths are relative to the base directory. Writes are atomic: data is written
// to a temp file in the same directory and then renamed into place.
type FileStore struct {
	baseDir string
}

// NewFileStore creates a FileStore rooted at baseDir.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{baseDir: baseDir}
}

// Write marshals v to JSON and atomically writes it to relPath under the base directory.
// Parent directories are created as needed.
func (fs *FileStore) Write(relPath string, v any) error {
	abs := fs.abs(relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return err
	}
	return atomicWrite(abs, v)
}

// Read reads the JSON file at relPath and unmarshals it into v.
// Returns ErrNotFound if the file does not exist.
func (fs *FileStore) Read(relPath string, v any) error {
	abs := fs.abs(relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return json.Unmarshal(data, v)
}

// Delete removes the file at relPath. Returns nil if the file does not exist.
func (fs *FileStore) Delete(relPath string) error {
	abs := fs.abs(relPath)
	err := os.Remove(abs)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns the relative paths of all .json files directly within relDir.
// The returned paths are relative to the base directory.
func (fs *FileStore) List(relDir string) ([]string, error) {
	abs := fs.abs(relDir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(relDir, e.Name()))
		}
	}
	return paths, nil
}

// ListDirs returns the names of all subdirectories directly within relDir.
// Returns nil (not an error) if the directory does not exist.
func (fs *FileStore) ListDirs(relDir string) ([]string, error) {
	abs := fs.abs(relDir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

// EnsureDir creates the directory at relPath if it does not exist.
func (fs *FileStore) EnsureDir(relPath string) error {
	return os.MkdirAll(fs.abs(relPath), 0o700)
}

// DeleteDir removes the directory at relPath and all its contents.
func (fs *FileStore) DeleteDir(relPath string) error {
	return os.RemoveAll(fs.abs(relPath))
}

// Abs returns the absolute path for a relative path within the base directory.
func (fs *FileStore) Abs(relPath string) string {
	return fs.abs(relPath)
}

func (fs *FileStore) abs(relPath string) string {
	return filepath.Join(fs.baseDir, relPath)
}

// atomicWrite marshals v to JSON and writes it atomically using a temp file + rename.
func atomicWrite(dst string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err = os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
