package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// mockRepo is an in-memory implementation of Repository for testing.
type mockRepo struct {
	stores map[string]*domain.Store
}

func newMockRepo() *mockRepo {
	return &mockRepo{stores: make(map[string]*domain.Store)}
}

func (m *mockRepo) Save(_ context.Context, s *domain.Store) error {
	m.stores[s.Name] = s
	return nil
}

func (m *mockRepo) Get(_ context.Context, name string) (*domain.Store, error) {
	s, ok := m.stores[name]
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "store not found: "+name)
	}
	return s, nil
}

func (m *mockRepo) Delete(_ context.Context, name string) error {
	if _, ok := m.stores[name]; !ok {
		return domain.NewAppError(domain.ErrCodeNotFound, "store not found: "+name)
	}
	delete(m.stores, name)
	return nil
}

func (m *mockRepo) List(_ context.Context) ([]*domain.Store, error) {
	result := make([]*domain.Store, 0, len(m.stores))
	for _, s := range m.stores {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockRepo) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.stores[name]
	return ok, nil
}

// helpers

func adminPrincipal() *domain.Principal {
	return &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
}

func userPrincipal(id string) *domain.Principal {
	return &domain.Principal{ID: id, Role: domain.InstanceRoleUser}
}

func storeWithOwner(name, ownerID string) *domain.Store {
	return &domain.Store{
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now().UTC(),
		Members:   []domain.StoreMember{{PrincipalID: ownerID, Role: domain.StoreRoleOwner}},
	}
}

// --- Create ---

func TestCreate_ValidName_CreatesStore(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	p := userPrincipal("alice")
	req := CreateRequest{Name: "my-store", Description: "desc", Principal: p}

	st, err := svc.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "my-store", st.Name)
	assert.Equal(t, "alice", st.OwnerID)
	assert.Len(t, st.Members, 1)
	assert.Equal(t, domain.StoreRoleOwner, st.Members[0].Role)
}

func TestCreate_InvalidName_ReturnsInvalidRequest(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	p := userPrincipal("alice")
	req := CreateRequest{Name: "My_Store!", Principal: p}

	_, err := svc.Create(context.Background(), req)
	require.Error(t, err)

	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestCreate_EmptyName_ReturnsInvalidRequest(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	req := CreateRequest{Name: "", Principal: userPrincipal("alice")}

	_, err := svc.Create(context.Background(), req)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestCreate_DuplicateName_ReturnsConflict(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	p := userPrincipal("alice")
	req := CreateRequest{Name: "my-store", Principal: p}

	_, err := svc.Create(context.Background(), req)
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), req)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeConflict, appErr.Code)
}

// --- Get ---

func TestGet_Found_ReturnStore(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	owner := userPrincipal("alice")
	_ = repo.Save(context.Background(), storeWithOwner("test-store", "alice"))

	st, err := svc.Get(context.Background(), "test-store", owner)
	require.NoError(t, err)
	assert.Equal(t, "test-store", st.Name)
}

func TestGet_NotFound_ReturnsNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_, err := svc.Get(context.Background(), "nonexistent", userPrincipal("alice"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

func TestGet_NonMember_ReturnsForbidden(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("test-store", "alice"))

	_, err := svc.Get(context.Background(), "test-store", userPrincipal("bob"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

func TestGet_Admin_CanSeeAnyStore(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("alice-store", "alice"))

	st, err := svc.Get(context.Background(), "alice-store", adminPrincipal())
	require.NoError(t, err)
	assert.Equal(t, "alice-store", st.Name)
}

// --- List ---

func TestList_Admin_ReturnsAllStores(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("alice-store", "alice"))
	_ = repo.Save(context.Background(), storeWithOwner("bob-store", "bob"))

	stores, err := svc.List(context.Background(), adminPrincipal())
	require.NoError(t, err)
	assert.Len(t, stores, 2)
}

func TestList_User_ReturnsOnlyOwnStores(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("alice-store", "alice"))
	_ = repo.Save(context.Background(), storeWithOwner("bob-store", "bob"))

	stores, err := svc.List(context.Background(), userPrincipal("alice"))
	require.NoError(t, err)
	assert.Len(t, stores, 1)
	assert.Equal(t, "alice-store", stores[0].Name)
}

// --- Delete ---

func TestDelete_Owner_DeletesStore(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("my-store", "alice"))

	err := svc.Delete(context.Background(), "my-store", "my-store", userPrincipal("alice"))
	require.NoError(t, err)

	_, err = repo.Get(context.Background(), "my-store")
	assert.Error(t, err)
}

func TestDelete_WrongConfirm_ReturnsInvalidRequest(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("my-store", "alice"))

	err := svc.Delete(context.Background(), "my-store", "wrong-name", userPrincipal("alice"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestDelete_PersonalStore_ReturnsForbidden(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	// Personal store: Name == OwnerID
	personal := &domain.Store{
		Name:    "alice",
		OwnerID: "alice",
		Members: []domain.StoreMember{{PrincipalID: "alice", Role: domain.StoreRoleOwner}},
	}
	_ = repo.Save(context.Background(), personal)

	err := svc.Delete(context.Background(), "alice", "alice", userPrincipal("alice"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeStoreDeleteForbidden, appErr.Code)
}

func TestDelete_NonOwner_ReturnsForbidden(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("shared-store", "alice"))

	err := svc.Delete(context.Background(), "shared-store", "shared-store", userPrincipal("bob"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

func TestDelete_Admin_CanDeleteAnyStore(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_ = repo.Save(context.Background(), storeWithOwner("alice-store", "alice"))

	err := svc.Delete(context.Background(), "alice-store", "alice-store", adminPrincipal())
	require.NoError(t, err)
}

func TestDelete_NotFound_ReturnsNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	err := svc.Delete(context.Background(), "no-such-store", "no-such-store", userPrincipal("alice"))
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}
