// Package principal provides the use case layer for principal management in ingatan.
package principal

import (
	"context"

	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
)

// Repository is the persistence interface for principal records.
// Defined here (use case layer) and implemented by the infrastructure layer.
type Repository interface {
	Save(ctx context.Context, p *domain.Principal) error
	Get(ctx context.Context, id string) (*domain.Principal, error)
	GetByAPIKeyHash(ctx context.Context, hash string) (*domain.Principal, error)
	List(ctx context.Context) ([]*domain.Principal, error)
	Update(ctx context.Context, p *domain.Principal) error
}

// StoreRepository is a minimal dependency for looking up store memberships.
// PrincipalService depends on this to auto-create personal stores and return WhoAmI memberships.
type StoreRepository interface {
	Save(ctx context.Context, s *domain.Store) error
	Get(ctx context.Context, name string) (*domain.Store, error)
	List(ctx context.Context) ([]*domain.Store, error)
	Exists(ctx context.Context, name string) (bool, error)
}

// Service exposes principal management operations.
type Service interface {
	GetOrCreate(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error)
	WhoAmI(ctx context.Context, p *domain.Principal) (*WhoAmIResponse, error)
	List(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error)
	Create(ctx context.Context, caller *domain.Principal, req CreateRequest) (*CreateResponse, error)
	ReissueAPIKey(ctx context.Context, caller *domain.Principal, id string) (string, error)
	RevokeAPIKey(ctx context.Context, caller *domain.Principal, id string) error
	AuthenticateByAPIKey(ctx context.Context, apiKey string) (*domain.Principal, error)
}

// WhoAmIResponse carries identity, store memberships, and derived capabilities.
type WhoAmIResponse struct {
	Principal        *domain.Principal
	StoreMemberships []StoreMembership
	Capabilities     []string
}

// StoreMembership pairs a store name with the principal's role in it.
type StoreMembership struct {
	StoreName string
	Role      domain.StoreRole
}

// CreateRequest carries the fields needed to create a new principal.
// ID is optional; a random identifier is generated when empty.
type CreateRequest struct {
	ID    string
	Name  string
	Type  domain.PrincipalType
	Role  domain.InstanceRole
	Email string
}

// CreateResponse carries the newly created principal and its plaintext API key.
// The APIKey is returned only once and is never stored in plaintext.
type CreateResponse struct {
	Principal *domain.Principal
	APIKey    string
}
