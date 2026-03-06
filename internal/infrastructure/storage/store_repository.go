package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/stainedhead/ingatan/internal/domain"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

// Compile-time interface check.
var _ storeuc.Repository = (*StoreRepo)(nil)

// StoreRepo implements store.Repository using FileStore.
// Records are persisted as stores/{name}/store.json.
type StoreRepo struct {
	fs *FileStore
}

// NewStoreRepo creates a StoreRepo backed by the given FileStore.
func NewStoreRepo(fs *FileStore) *StoreRepo {
	return &StoreRepo{fs: fs}
}

// storePath returns the relative path for a store record.
func storePath(name string) string {
	return filepath.Join("stores", name, "store.json")
}

// Save persists a store record to disk.
func (r *StoreRepo) Save(_ context.Context, s *domain.Store) error {
	return r.fs.Write(storePath(s.Name), s)
}

// Get reads a store record from disk.
// Returns a NOT_FOUND AppError if the store does not exist.
func (r *StoreRepo) Get(_ context.Context, name string) (*domain.Store, error) {
	var s domain.Store
	if err := r.fs.Read(storePath(name), &s); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("store not found: %s", name))
		}
		return nil, fmt.Errorf("storage: read store %q: %w", name, err)
	}
	return &s, nil
}

// Delete removes a store record from disk.
// Returns a NOT_FOUND AppError if the store does not exist.
func (r *StoreRepo) Delete(_ context.Context, name string) error {
	if err := r.fs.Delete(storePath(name)); err != nil {
		return fmt.Errorf("storage: delete store %q: %w", name, err)
	}
	return nil
}

// List returns all store records by walking the stores/ directory.
func (r *StoreRepo) List(_ context.Context) ([]*domain.Store, error) {
	entries, err := r.fs.ListDirs("stores")
	if err != nil {
		return nil, fmt.Errorf("storage: list stores dir: %w", err)
	}

	var stores []*domain.Store
	for _, entry := range entries {
		p := filepath.Join("stores", entry, "store.json")
		var s domain.Store
		if err := r.fs.Read(p, &s); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // store dir exists but no store.json yet — skip
			}
			return nil, fmt.Errorf("storage: read store %q: %w", entry, err)
		}
		stores = append(stores, &s)
	}

	return stores, nil
}

// Exists reports whether a store record exists on disk.
func (r *StoreRepo) Exists(ctx context.Context, name string) (bool, error) {
	_, err := r.Get(ctx, name)
	if err == nil {
		return true, nil
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeNotFound {
		return false, nil
	}
	return false, err
}
