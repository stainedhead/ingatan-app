package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/stainedhead/ingatan/internal/domain"
)

// storeNameRE validates store names: only lowercase alphanumeric and hyphens.
var storeNameRE = regexp.MustCompile(`^[a-z0-9-]+$`)

// serviceImpl is the concrete implementation of Service.
type serviceImpl struct {
	repo Repository
}

// NewService constructs a Service backed by the given Repository.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// Create validates the request and creates a new store owned by the requesting principal.
// Returns INVALID_REQUEST if the name is not a valid slug, or CONFLICT if it already exists.
func (s *serviceImpl) Create(ctx context.Context, req CreateRequest) (*domain.Store, error) {
	if !storeNameRE.MatchString(req.Name) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store name must match [a-z0-9-]+")
	}

	exists, err := s.repo.Exists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("store: check exists: %w", err)
	}
	if exists {
		return nil, domain.NewAppError(domain.ErrCodeConflict, fmt.Sprintf("store %q already exists", req.Name))
	}

	st := &domain.Store{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     req.Principal.ID,
		CreatedAt:   time.Now().UTC(),
		Members: []domain.StoreMember{
			{PrincipalID: req.Principal.ID, Role: domain.StoreRoleOwner},
		},
	}

	if err := s.repo.Save(ctx, st); err != nil {
		return nil, fmt.Errorf("store: save: %w", err)
	}

	return st, nil
}

// Get retrieves a store by name, enforcing access control.
// Admins can see any store; non-admins must be members.
func (s *serviceImpl) Get(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error) {
	st, err := s.repo.Get(ctx, name)
	if err != nil {
		return nil, wrapNotFound(err, name)
	}

	if principal.Role == domain.InstanceRoleAdmin {
		return st, nil
	}

	if st.MemberRole(principal.ID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "access to store denied")
	}

	return st, nil
}

// List returns all stores visible to the principal.
// Admins see all stores; non-admins see only stores they are members of.
func (s *serviceImpl) List(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error) {
	all, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}

	if principal.Role == domain.InstanceRoleAdmin {
		return all, nil
	}

	var visible []*domain.Store
	for _, st := range all {
		if st.MemberRole(principal.ID) != "" {
			visible = append(visible, st)
		}
	}
	return visible, nil
}

// Delete removes a store after validating the confirm field and access rights.
// Personal stores (name == ownerID) cannot be deleted.
// Only the store owner or an instance admin may delete a store.
func (s *serviceImpl) Delete(ctx context.Context, name, confirm string, principal *domain.Principal) error {
	st, err := s.repo.Get(ctx, name)
	if err != nil {
		return wrapNotFound(err, name)
	}

	if confirm != name {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "confirm must equal the store name")
	}

	if st.IsPersonal() {
		return domain.NewAppError(domain.ErrCodeStoreDeleteForbidden, "personal stores cannot be deleted")
	}

	if principal.Role != domain.InstanceRoleAdmin && st.OwnerID != principal.ID {
		return domain.NewAppError(domain.ErrCodeForbidden, "only the store owner or an admin may delete a store")
	}

	if err := s.repo.Delete(ctx, name); err != nil {
		return fmt.Errorf("store: delete: %w", err)
	}

	return nil
}

// wrapNotFound converts a storage not-found error into a domain NOT_FOUND AppError.
func wrapNotFound(err error, name string) error {
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeNotFound {
		return appErr
	}
	return domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("store not found: %s", name))
}
