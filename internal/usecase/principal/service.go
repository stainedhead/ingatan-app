package principal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

// Create creates a new principal with a generated API key. Only admins may call this operation.
// The plaintext API key is returned once in CreateResponse and is never stored.
func (s *serviceImpl) Create(ctx context.Context, caller *domain.Principal, req CreateRequest) (*CreateResponse, error) {
	if caller.Role != domain.InstanceRoleAdmin {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "only admins may create principals")
	}

	id := req.ID
	if id == "" {
		var err error
		id, err = generateID()
		if err != nil {
			return nil, fmt.Errorf("principal: generate id: %w", err)
		}
	}

	plaintext, hash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("principal: generate api key: %w", err)
	}

	now := time.Now().UTC()
	p := &domain.Principal{
		ID:         id,
		Name:       req.Name,
		Type:       req.Type,
		Role:       req.Role,
		Email:      req.Email,
		APIKeyHash: hash,
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if err := s.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("principal: save: %w", err)
	}

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

	return &CreateResponse{Principal: p, APIKey: plaintext}, nil
}

// ReissueAPIKey rotates the API key for the given principal. Only admins may call this operation.
// Returns the new plaintext API key, shown once.
func (s *serviceImpl) ReissueAPIKey(ctx context.Context, caller *domain.Principal, id string) (string, error) {
	if caller.Role != domain.InstanceRoleAdmin {
		return "", domain.NewAppError(domain.ErrCodeForbidden, "only admins may reissue api keys")
	}

	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("principal: get: %w", err)
	}

	plaintext, hash, err := generateAPIKey()
	if err != nil {
		return "", fmt.Errorf("principal: generate api key: %w", err)
	}

	p.APIKeyHash = hash
	if err := s.repo.Update(ctx, p); err != nil {
		return "", fmt.Errorf("principal: update: %w", err)
	}

	return plaintext, nil
}

// RevokeAPIKey clears the API key for the given principal. Only admins may call this operation.
// After revocation the principal can no longer exchange an API key for a JWT.
func (s *serviceImpl) RevokeAPIKey(ctx context.Context, caller *domain.Principal, id string) error {
	if caller.Role != domain.InstanceRoleAdmin {
		return domain.NewAppError(domain.ErrCodeForbidden, "only admins may revoke api keys")
	}

	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("principal: get: %w", err)
	}

	p.APIKeyHash = ""
	if err := s.repo.Update(ctx, p); err != nil {
		return fmt.Errorf("principal: update: %w", err)
	}

	return nil
}

// AuthenticateByAPIKey validates a plaintext API key and returns the matching principal.
// Returns UNAUTHORIZED if the key is invalid or has been revoked.
func (s *serviceImpl) AuthenticateByAPIKey(ctx context.Context, apiKey string) (*domain.Principal, error) {
	sum := sha256.Sum256([]byte(apiKey))
	hash := hex.EncodeToString(sum[:])

	p, err := s.repo.GetByAPIKeyHash(ctx, hash)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "invalid api key")
	}

	p.LastSeenAt = time.Now().UTC()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("principal: update last seen: %w", err)
	}

	return p, nil
}

// generateAPIKey creates a cryptographically random API key and its SHA-256 hash.
// The key is prefixed with "igt_" for easy identification.
func generateAPIKey() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}
	plaintext = "igt_" + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])
	return plaintext, hash, nil
}

// generateID creates a random principal ID in the form "p-<16-hex-chars>".
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "p-" + hex.EncodeToString(b), nil
}
