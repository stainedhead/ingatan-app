package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// newTestStore returns a Store with deterministic values for testing.
func newTestStore(name, ownerID string) *domain.Store {
	return &domain.Store{
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Members: []domain.StoreMember{
			{PrincipalID: ownerID, Role: domain.StoreRoleOwner},
		},
	}
}

// TestStoreRepo_SaveAndGet verifies that a saved store can be read back with all fields intact.
func TestStoreRepo_SaveAndGet(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	s := newTestStore("my-store", "alice")

	require.NoError(t, repo.Save(ctx, s))

	got, err := repo.Get(ctx, "my-store")
	require.NoError(t, err)

	assert.Equal(t, s.Name, got.Name)
	assert.Equal(t, s.OwnerID, got.OwnerID)
	assert.Equal(t, s.CreatedAt.UTC(), got.CreatedAt.UTC())
	require.Len(t, got.Members, 1)
	assert.Equal(t, "alice", got.Members[0].PrincipalID)
	assert.Equal(t, domain.StoreRoleOwner, got.Members[0].Role)
}

// TestStoreRepo_Get_NotFound verifies that Get returns a NOT_FOUND AppError for missing stores.
func TestStoreRepo_Get_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestStoreRepo_Delete verifies that a deleted store returns NOT_FOUND on subsequent Get.
func TestStoreRepo_Delete(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	s := newTestStore("to-delete", "alice")
	require.NoError(t, repo.Save(ctx, s))

	require.NoError(t, repo.Delete(ctx, "to-delete"))

	_, err := repo.Get(ctx, "to-delete")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestStoreRepo_List verifies that all saved stores are returned.
func TestStoreRepo_List(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	for _, name := range []string{"store-a", "store-b", "store-c"} {
		require.NoError(t, repo.Save(ctx, newTestStore(name, "alice")))
	}

	stores, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, stores, 3)
}

// TestStoreRepo_List_Empty verifies that List returns nil (not an error) when no stores exist.
func TestStoreRepo_List_Empty(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	stores, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Nil(t, stores)
}

// TestStoreRepo_Exists_True verifies that Exists returns true for a saved store.
func TestStoreRepo_Exists_True(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, newTestStore("present", "alice")))

	exists, err := repo.Exists(ctx, "present")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestStoreRepo_Exists_False verifies that Exists returns false for a missing store.
func TestStoreRepo_Exists_False(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewStoreRepo(fs)
	ctx := context.Background()

	exists, err := repo.Exists(ctx, "absent")
	require.NoError(t, err)
	assert.False(t, exists)
}
