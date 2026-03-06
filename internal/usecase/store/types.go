// Package store provides the use case layer for store management in ingatan.
package store

import (
	"context"

	"github.com/stainedhead/ingatan/internal/domain"
)

// Repository is the persistence interface for store records.
// Defined here (use case layer) and implemented by the infrastructure layer.
type Repository interface {
	Save(ctx context.Context, s *domain.Store) error
	Get(ctx context.Context, name string) (*domain.Store, error)
	Delete(ctx context.Context, name string) error
	List(ctx context.Context) ([]*domain.Store, error)
	Exists(ctx context.Context, name string) (bool, error)
}

// Service exposes store management operations.
type Service interface {
	Create(ctx context.Context, req CreateRequest) (*domain.Store, error)
	Get(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error)
	List(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error)
	Delete(ctx context.Context, name, confirm string, principal *domain.Principal) error
}

// CreateRequest carries input for Service.Create.
type CreateRequest struct {
	Name        string
	Description string
	Principal   *domain.Principal
}
