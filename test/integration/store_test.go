package integration_test

import (
	"net/http"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGet(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/stores", tok, map[string]any{
		"name":        "team-alpha",
		"description": "Alpha team shared store",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	decodeJSON(t, resp, &created)
	name := firstString(created, "Name", "name")
	assert.Equal(t, "team-alpha", name)

	// Get it back.
	resp2 := ts.do(t, http.MethodGet, "/api/v1/stores/team-alpha", tok, nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var got map[string]any
	decodeJSON(t, resp2, &got)
	assert.Equal(t, "team-alpha", firstString(got, "Name", "name"))
	assert.Equal(t, "Alpha team shared store", firstString(got, "Description", "description"))
}

func TestStore_List(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// The first authenticated request auto-creates the personal store "user-1".
	resp := ts.do(t, http.MethodGet, "/api/v1/stores", tok, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list map[string]any
	decodeJSON(t, resp, &list)
	stores, _ := list["stores"].([]any)
	require.NotEmpty(t, stores, "should have at least the personal store")

	// Check that the personal store is in the list.
	found := false
	for _, s := range stores {
		sm, _ := s.(map[string]any)
		if firstString(sm, "Name", "name") == "user-1" {
			found = true
			break
		}
	}
	assert.True(t, found, "personal store user-1 should be in the list")
}

func TestStore_Delete(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Create a non-personal store.
	resp := ts.do(t, http.MethodPost, "/api/v1/stores", tok, map[string]any{
		"name":        "temp-store",
		"description": "Temporary store for deletion test",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Delete it with confirm.
	delResp := ts.do(t, http.MethodDelete, "/api/v1/stores/temp-store", tok, map[string]any{
		"confirm": "temp-store",
	})
	require.Equal(t, http.StatusOK, delResp.StatusCode)

	// Verify it's gone.
	getResp := ts.do(t, http.MethodGet, "/api/v1/stores/temp-store", tok, nil)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)

}

func TestStore_DeletePersonalForbidden(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Ensure personal store exists via any authenticated call.
	resp := ts.do(t, http.MethodGet, "/api/v1/stores", tok, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Try to delete the personal store.
	delResp := ts.do(t, http.MethodDelete, "/api/v1/stores/user-1", tok, map[string]any{
		"confirm": "user-1",
	})
	assert.Equal(t, http.StatusForbidden, delResp.StatusCode)

}

func TestStore_NameValidation(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Invalid name: uppercase characters.
	resp := ts.do(t, http.MethodPost, "/api/v1/stores", tok, map[string]any{
		"name":        "InvalidName",
		"description": "Should fail validation",
	})
	// Expect 422 (INVALID_REQUEST) since store name must match ^[a-z0-9-]+$.
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

}
