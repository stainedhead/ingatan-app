package principal

import (
	"context"
	"errors"
	"fmt"
	"time"

	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
)

// serviceImpl is the concrete implementation of Service.
type serviceImpl struct {
	repo      Repository
	storeRepo StoreRepository
}

// NewService constructs a Service backed by the given repositories.
func NewService(repo Repository, storeRepo StoreRepository) Service {
	return &serviceImpl{repo: repo, storeRepo: storeRepo}
}

// GetOrCreate resolves a principal from JWT claims, creating it on first sight.
// If the principal already exists, LastSeenAt is updated.
// A personal store is auto-created for new principals if one does not already exist.
func (s *serviceImpl) GetOrCreate(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	p, err := s.repo.Get(ctx, claims.Subject)
	if err == nil {
		// Existing principal — update LastSeenAt.
		p.LastSeenAt = time.Now().UTC()
		if updateErr := s.repo.Update(ctx, p); updateErr != nil {
			return nil, fmt.Errorf("principal: update last seen: %w", updateErr)
		}
		return p, nil
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeNotFound {
		return nil, fmt.Errorf("principal: get: %w", err)
	}

	// New principal.
	now := time.Now().UTC()
	p = &domain.Principal{
		ID:         claims.Subject,
		Name:       claims.Name,
		Type:       claims.Type,
		Role:       claims.Role,
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if err := s.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("principal: save: %w", err)
	}

	// Auto-create personal store if it does not already exist.
	exists, err := s.storeRepo.Exists(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("principal: check personal store: %w", err)
	}
	if !exists {
		personalStore := &domain.Store{
			Name:      p.ID,
			OwnerID:   p.ID,
			CreatedAt: now,
			Members: []domain.StoreMember{
				{PrincipalID: p.ID, Role: domain.StoreRoleOwner},
			},
		}
		if err := s.storeRepo.Save(ctx, personalStore); err != nil {
			return nil, fmt.Errorf("principal: save personal store: %w", err)
		}
	}

	return p, nil
}

// WhoAmI returns the principal's identity, store memberships, and derived capabilities.
func (s *serviceImpl) WhoAmI(ctx context.Context, p *domain.Principal) (*WhoAmIResponse, error) {
	stores, err := s.storeRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("principal: list stores: %w", err)
	}

	var memberships []StoreMembership
	capSet := make(map[string]struct{})

	for _, st := range stores {
		role := st.MemberRole(p.ID)
		if role == "" {
			continue
		}
		memberships = append(memberships, StoreMembership{StoreName: st.Name, Role: role})

		switch role {
		case domain.StoreRoleOwner, domain.StoreRoleWriter:
			capSet["memory:read"] = struct{}{}
			capSet["memory:write"] = struct{}{}
		case domain.StoreRoleReader:
			capSet["memory:read"] = struct{}{}
		}
	}

	caps := make([]string, 0, len(capSet))
	for c := range capSet {
		caps = append(caps, c)
	}

	return &WhoAmIResponse{
		Principal:        p,
		StoreMemberships: memberships,
		Capabilities:     caps,
	}, nil
}

// List returns all principals. Only admins may call this operation.
func (s *serviceImpl) List(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error) {
	if caller.Role != domain.InstanceRoleAdmin {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "only admins may list principals")
	}
	principals, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("principal: list: %w", err)
	}
	return principals, nil
}
