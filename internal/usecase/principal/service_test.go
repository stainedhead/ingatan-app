package principal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
)

// mockPrincipalRepo is an in-memory implementation of Repository for testing.
type mockPrincipalRepo struct {
	principals map[string]*domain.Principal
}

func newMockPrincipalRepo() *mockPrincipalRepo {
	return &mockPrincipalRepo{principals: make(map[string]*domain.Principal)}
}

func (m *mockPrincipalRepo) Save(_ context.Context, p *domain.Principal) error {
	m.principals[p.ID] = p
	return nil
}

func (m *mockPrincipalRepo) Get(_ context.Context, id string) (*domain.Principal, error) {
	p, ok := m.principals[id]
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "principal not found: "+id)
	}
	return p, nil
}

func (m *mockPrincipalRepo) List(_ context.Context) ([]*domain.Principal, error) {
	result := make([]*domain.Principal, 0, len(m.principals))
	for _, p := range m.principals {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockPrincipalRepo) GetByAPIKeyHash(_ context.Context, hash string) (*domain.Principal, error) {
	if hash == "" {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "no principal found for api key")
	}
	for _, p := range m.principals {
		if p.APIKeyHash == hash {
			return p, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeNotFound, "no principal found for api key")
}

func (m *mockPrincipalRepo) Update(_ context.Context, p *domain.Principal) error {
	if _, ok := m.principals[p.ID]; !ok {
		return domain.NewAppError(domain.ErrCodeNotFound, "principal not found: "+p.ID)
	}
	m.principals[p.ID] = p
	return nil
}

// mockStoreRepo is an in-memory implementation of StoreRepository for testing.
type mockStoreRepo struct {
	stores map[string]*domain.Store
}

func newMockStoreRepo() *mockStoreRepo {
	return &mockStoreRepo{stores: make(map[string]*domain.Store)}
}

func (m *mockStoreRepo) Save(_ context.Context, s *domain.Store) error {
	m.stores[s.Name] = s
	return nil
}

func (m *mockStoreRepo) Get(_ context.Context, name string) (*domain.Store, error) {
	s, ok := m.stores[name]
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "store not found: "+name)
	}
	return s, nil
}

func (m *mockStoreRepo) List(_ context.Context) ([]*domain.Store, error) {
	result := make([]*domain.Store, 0, len(m.stores))
	for _, s := range m.stores {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockStoreRepo) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.stores[name]
	return ok, nil
}

// helpers

func makeClaims(subject, name string, role domain.InstanceRole) apimw.JWTClaims {
	return apimw.JWTClaims{
		Name: name,
		Type: domain.PrincipalTypeHuman,
		Role: role,
	}
}

// setSubject is needed because JWTClaims embeds RegisteredClaims.
func claimsFor(subject, name string, role domain.InstanceRole) apimw.JWTClaims {
	c := makeClaims(subject, name, role)
	c.Subject = subject
	return c
}

// --- GetOrCreate ---

func TestGetOrCreate_NewPrincipal_CreatesAndSavesPersonalStore(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	claims := claimsFor("alice-id", "Alice", domain.InstanceRoleUser)

	p, err := svc.GetOrCreate(context.Background(), claims)
	require.NoError(t, err)

	assert.Equal(t, "alice-id", p.ID)
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, domain.InstanceRoleUser, p.Role)

	// Personal store should be created.
	_, exists := storeRepo.stores["alice-id"]
	assert.True(t, exists, "personal store should be created")
}

func TestGetOrCreate_ExistingPrincipal_UpdatesLastSeenAt(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	earlier := time.Now().UTC().Add(-1 * time.Hour)
	existing := &domain.Principal{
		ID:         "bob-id",
		Name:       "Bob",
		Role:       domain.InstanceRoleUser,
		LastSeenAt: earlier,
	}
	_ = repo.Save(context.Background(), existing)

	claims := claimsFor("bob-id", "Bob", domain.InstanceRoleUser)

	p, err := svc.GetOrCreate(context.Background(), claims)
	require.NoError(t, err)

	assert.Equal(t, "bob-id", p.ID)
	assert.True(t, p.LastSeenAt.After(earlier), "LastSeenAt should be updated")
	// No additional personal store should be created.
	assert.Len(t, storeRepo.stores, 0)
}

func TestGetOrCreate_ExistingPersonalStore_DoesNotDuplicate(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	// Pre-existing personal store.
	_ = storeRepo.Save(context.Background(), &domain.Store{Name: "carol-id", OwnerID: "carol-id"})

	claims := claimsFor("carol-id", "Carol", domain.InstanceRoleUser)

	_, err := svc.GetOrCreate(context.Background(), claims)
	require.NoError(t, err)

	// Should still be exactly one store.
	assert.Len(t, storeRepo.stores, 1)
}

// --- WhoAmI ---

func TestWhoAmI_ReturnsMembershipsAndCapabilities(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	p := &domain.Principal{ID: "alice-id", Role: domain.InstanceRoleUser}
	_ = storeRepo.Save(context.Background(), &domain.Store{
		Name:    "alice-id",
		OwnerID: "alice-id",
		Members: []domain.StoreMember{{PrincipalID: "alice-id", Role: domain.StoreRoleOwner}},
	})
	_ = storeRepo.Save(context.Background(), &domain.Store{
		Name:    "shared",
		OwnerID: "bob-id",
		Members: []domain.StoreMember{{PrincipalID: "alice-id", Role: domain.StoreRoleReader}},
	})

	resp, err := svc.WhoAmI(context.Background(), p)
	require.NoError(t, err)

	assert.Equal(t, p, resp.Principal)
	assert.Len(t, resp.StoreMemberships, 2)

	capSet := make(map[string]bool)
	for _, c := range resp.Capabilities {
		capSet[c] = true
	}
	assert.True(t, capSet["memory:read"])
	assert.True(t, capSet["memory:write"])
}

func TestWhoAmI_NoMemberships_ReturnsEmptySlices(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	p := &domain.Principal{ID: "orphan-id", Role: domain.InstanceRoleUser}

	resp, err := svc.WhoAmI(context.Background(), p)
	require.NoError(t, err)

	assert.Len(t, resp.StoreMemberships, 0)
	assert.Len(t, resp.Capabilities, 0)
}

// --- List ---

func TestList_Admin_ReturnsAllPrincipals(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	_ = repo.Save(context.Background(), &domain.Principal{ID: "alice-id"})
	_ = repo.Save(context.Background(), &domain.Principal{ID: "bob-id"})

	caller := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}

	principals, err := svc.List(context.Background(), caller)
	require.NoError(t, err)
	assert.Len(t, principals, 2)
}

func TestList_NonAdmin_ReturnsForbidden(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	caller := &domain.Principal{ID: "alice-id", Role: domain.InstanceRoleUser}

	_, err := svc.List(context.Background(), caller)
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

// --- Create ---

func TestCreate_Admin_CreatesPrincipalAndReturnsAPIKey(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	req := CreateRequest{
		ID:   "new-user",
		Name: "New User",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleUser,
	}

	resp, err := svc.Create(context.Background(), admin, req)
	require.NoError(t, err)

	assert.Equal(t, "new-user", resp.Principal.ID)
	assert.NotEmpty(t, resp.APIKey, "plaintext API key should be returned")
	assert.True(t, len(resp.APIKey) > 10, "API key should be reasonably long")
	assert.NotEmpty(t, resp.Principal.APIKeyHash, "hash should be stored")
	assert.NotEqual(t, resp.APIKey, resp.Principal.APIKeyHash, "plaintext must differ from hash")

	// Personal store auto-created.
	_, exists := storeRepo.stores["new-user"]
	assert.True(t, exists)
}

func TestCreate_NonAdmin_ReturnsForbidden(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	caller := &domain.Principal{ID: "alice-id", Role: domain.InstanceRoleUser}

	_, err := svc.Create(context.Background(), caller, CreateRequest{Name: "Bob"})
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

func TestCreate_GeneratesIDWhenEmpty(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	resp, err := svc.Create(context.Background(), admin, CreateRequest{
		Name: "Auto ID User",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleUser,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Principal.ID)
}

// --- ReissueAPIKey ---

func TestReissueAPIKey_Admin_ReturnsNewKey(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	// Create principal first.
	resp, err := svc.Create(context.Background(), admin, CreateRequest{
		ID: "bob", Name: "Bob", Type: domain.PrincipalTypeHuman, Role: domain.InstanceRoleUser,
	})
	require.NoError(t, err)
	oldKey := resp.APIKey
	oldHash := resp.Principal.APIKeyHash

	newKey, err := svc.ReissueAPIKey(context.Background(), admin, "bob")
	require.NoError(t, err)

	assert.NotEmpty(t, newKey)
	assert.NotEqual(t, oldKey, newKey, "re-issued key must differ")
	assert.NotEqual(t, oldHash, repo.principals["bob"].APIKeyHash, "hash must be updated")
}

func TestReissueAPIKey_NonAdmin_ReturnsForbidden(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	caller := &domain.Principal{ID: "alice-id", Role: domain.InstanceRoleUser}
	_, err := svc.ReissueAPIKey(context.Background(), caller, "bob")
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

// --- RevokeAPIKey ---

func TestRevokeAPIKey_Admin_ClearsHash(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	_, err := svc.Create(context.Background(), admin, CreateRequest{
		ID: "bob", Name: "Bob", Type: domain.PrincipalTypeHuman, Role: domain.InstanceRoleUser,
	})
	require.NoError(t, err)
	require.NotEmpty(t, repo.principals["bob"].APIKeyHash)

	require.NoError(t, svc.RevokeAPIKey(context.Background(), admin, "bob"))
	assert.Empty(t, repo.principals["bob"].APIKeyHash, "hash should be cleared after revoke")
}

func TestRevokeAPIKey_NonAdmin_ReturnsForbidden(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	caller := &domain.Principal{ID: "alice-id", Role: domain.InstanceRoleUser}
	err := svc.RevokeAPIKey(context.Background(), caller, "bob")
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

// --- AuthenticateByAPIKey ---

func TestAuthenticateByAPIKey_ValidKey_ReturnsPrincipal(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	resp, err := svc.Create(context.Background(), admin, CreateRequest{
		ID: "alice", Name: "Alice", Type: domain.PrincipalTypeHuman, Role: domain.InstanceRoleUser,
	})
	require.NoError(t, err)

	p, err := svc.AuthenticateByAPIKey(context.Background(), resp.APIKey)
	require.NoError(t, err)
	assert.Equal(t, "alice", p.ID)
}

func TestAuthenticateByAPIKey_InvalidKey_ReturnsUnauthorized(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	_, err := svc.AuthenticateByAPIKey(context.Background(), "igt_notavalidkey")
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeUnauthorized, appErr.Code)
}

func TestAuthenticateByAPIKey_RevokedKey_ReturnsUnauthorized(t *testing.T) {
	repo := newMockPrincipalRepo()
	storeRepo := newMockStoreRepo()
	svc := NewService(repo, storeRepo)

	admin := &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
	resp, err := svc.Create(context.Background(), admin, CreateRequest{
		ID: "alice", Name: "Alice", Type: domain.PrincipalTypeHuman, Role: domain.InstanceRoleUser,
	})
	require.NoError(t, err)
	savedKey := resp.APIKey

	require.NoError(t, svc.RevokeAPIKey(context.Background(), admin, "alice"))

	_, err = svc.AuthenticateByAPIKey(context.Background(), savedKey)
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeUnauthorized, appErr.Code)
}
