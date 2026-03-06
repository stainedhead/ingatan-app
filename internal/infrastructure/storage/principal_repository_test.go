package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// newTestPrincipal returns a Principal with deterministic values for testing.
func newTestPrincipal(id, name string) *domain.Principal {
	return &domain.Principal{
		ID:         id,
		Name:       name,
		Type:       domain.PrincipalTypeHuman,
		Role:       domain.InstanceRoleUser,
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastSeenAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// TestPrincipalRepo_SaveAndGet verifies that a saved principal can be read back.
func TestPrincipalRepo_SaveAndGet(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	p := newTestPrincipal("alice-id", "Alice")

	require.NoError(t, repo.Save(ctx, p))

	got, err := repo.Get(ctx, "alice-id")
	require.NoError(t, err)

	assert.Equal(t, p.ID, got.ID)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.Type, got.Type)
	assert.Equal(t, p.Role, got.Role)
	assert.Equal(t, p.CreatedAt.UTC(), got.CreatedAt.UTC())
}

// TestPrincipalRepo_Get_NotFound verifies that Get returns a NOT_FOUND AppError for missing principals.
func TestPrincipalRepo_Get_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nobody")
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestPrincipalRepo_List verifies that all saved principals are returned.
func TestPrincipalRepo_List(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, newTestPrincipal("p1", "Alice")))
	require.NoError(t, repo.Save(ctx, newTestPrincipal("p2", "Bob")))
	require.NoError(t, repo.Save(ctx, newTestPrincipal("p3", "Carol")))

	principals, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, principals, 3)
}

// TestPrincipalRepo_List_Empty verifies that List returns an empty slice (not an error) when none exist.
func TestPrincipalRepo_List_Empty(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	principals, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, principals)
}

// TestPrincipalRepo_Update verifies that updating a principal overwrites the stored record.
func TestPrincipalRepo_Update(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	p := newTestPrincipal("alice-id", "Alice")
	require.NoError(t, repo.Save(ctx, p))

	p.Name = "Alice Updated"
	require.NoError(t, repo.Update(ctx, p))

	got, err := repo.Get(ctx, "alice-id")
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", got.Name)
}

// TestPrincipalRepo_Update_NotFound verifies that Update returns NOT_FOUND for a missing principal.
func TestPrincipalRepo_Update_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	p := newTestPrincipal("ghost-id", "Ghost")
	err := repo.Update(ctx, p)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestPrincipalRepo_Save_DuplicateID verifies that saving a duplicate principal returns CONFLICT.
func TestPrincipalRepo_Save_DuplicateID(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewPrincipalRepo(fs)
	ctx := context.Background()

	p := newTestPrincipal("dup-id", "Duplicate")
	require.NoError(t, repo.Save(ctx, p))

	err := repo.Save(ctx, p)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeConflict, appErr.Code)
}
