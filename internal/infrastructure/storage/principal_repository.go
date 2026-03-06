package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/stainedhead/ingatan/internal/domain"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
)

// Compile-time interface check.
var _ principaluc.Repository = (*PrincipalRepo)(nil)

// principalsFile is the path within the data dir where all principals are stored.
const principalsFile = "principals.json"

// PrincipalRepo implements principal.Repository using FileStore.
// All principals are stored in principals.json as a []domain.Principal array.
type PrincipalRepo struct {
	fs *FileStore
}

// NewPrincipalRepo creates a PrincipalRepo backed by the given FileStore.
func NewPrincipalRepo(fs *FileStore) *PrincipalRepo {
	return &PrincipalRepo{fs: fs}
}

// readAll reads the full principal list from disk.
// Returns an empty slice (not an error) if the file does not exist yet.
func (r *PrincipalRepo) readAll() ([]*domain.Principal, error) {
	var principals []*domain.Principal
	if err := r.fs.Read(principalsFile, &principals); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []*domain.Principal{}, nil
		}
		return nil, fmt.Errorf("storage: read principals: %w", err)
	}
	return principals, nil
}

// writeAll persists the full principal list to disk atomically.
func (r *PrincipalRepo) writeAll(principals []*domain.Principal) error {
	return r.fs.Write(principalsFile, principals)
}

// Save appends a new principal to the list.
// Returns an error if a principal with the same ID already exists.
func (r *PrincipalRepo) Save(_ context.Context, p *domain.Principal) error {
	principals, err := r.readAll()
	if err != nil {
		return err
	}
	for _, existing := range principals {
		if existing.ID == p.ID {
			return domain.NewAppError(domain.ErrCodeConflict, fmt.Sprintf("principal %q already exists", p.ID))
		}
	}
	principals = append(principals, p)
	return r.writeAll(principals)
}

// Get finds a principal by ID.
// Returns a NOT_FOUND AppError if no matching principal exists.
func (r *PrincipalRepo) Get(_ context.Context, id string) (*domain.Principal, error) {
	principals, err := r.readAll()
	if err != nil {
		return nil, err
	}
	for _, p := range principals {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("principal not found: %s", id))
}

// GetByAPIKeyHash finds a principal by the SHA-256 hash of their API key.
// Returns a NOT_FOUND AppError if no matching principal exists or the hash is empty.
func (r *PrincipalRepo) GetByAPIKeyHash(_ context.Context, hash string) (*domain.Principal, error) {
	if hash == "" {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "no principal found for api key")
	}
	principals, err := r.readAll()
	if err != nil {
		return nil, err
	}
	for _, p := range principals {
		if p.APIKeyHash == hash {
			return p, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeNotFound, "no principal found for api key")
}

// List returns all principals.
func (r *PrincipalRepo) List(_ context.Context) ([]*domain.Principal, error) {
	return r.readAll()
}

// Update replaces an existing principal record.
// Returns a NOT_FOUND AppError if no principal with that ID exists.
func (r *PrincipalRepo) Update(_ context.Context, p *domain.Principal) error {
	principals, err := r.readAll()
	if err != nil {
		return err
	}
	for i, existing := range principals {
		if existing.ID == p.ID {
			principals[i] = p
			return r.writeAll(principals)
		}
	}
	return domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("principal not found: %s", p.ID))
}
