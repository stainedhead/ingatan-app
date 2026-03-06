package integration_test

import (
	"net/http"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemory_SaveAndGet(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Save a memory.
	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Test Memory",
		"content": "This is a test memory with some content.",
		"tags":    []string{"test", "integration"},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	decodeJSON(t, resp, &created)
	memID, ok := created["ID"].(string)
	if !ok {
		memID, _ = created["id"].(string)
	}
	require.NotEmpty(t, memID, "memory ID should be present in response")
	assert.Equal(t, "Test Memory", firstString(created, "Title", "title"))
	assert.Equal(t, "This is a test memory with some content.", firstString(created, "Content", "content"))

	// Get it back.
	resp2 := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories/"+memID, tok, nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var got map[string]any
	decodeJSON(t, resp2, &got)
	assert.Equal(t, memID, firstString(got, "ID", "id"))
	assert.Equal(t, "Test Memory", firstString(got, "Title", "title"))
}

func TestMemory_List(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	for i := 0; i < 3; i++ {
		resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
			"title":   "Memory",
			"content": "Content for memory number test list",
		})
		require.Equal(t, http.StatusCreated, resp.StatusCode)

	}

	resp := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories", tok, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list map[string]any
	decodeJSON(t, resp, &list)
	memories, _ := list["memories"].([]any)
	assert.Len(t, memories, 3)
}

func TestMemory_Update(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Original Title",
		"content": "Original content.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	decodeJSON(t, resp, &created)
	memID := firstString(created, "ID", "id")

	resp2 := ts.do(t, http.MethodPut, "/api/v1/stores/user-1/memories/"+memID, tok, map[string]any{
		"title":   "Updated Title",
		"content": "Updated content.",
	})
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var updated map[string]any
	decodeJSON(t, resp2, &updated)
	assert.Equal(t, "Updated Title", firstString(updated, "Title", "title"))
	assert.Equal(t, "Updated content.", firstString(updated, "Content", "content"))
}

func TestMemory_Delete(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "To Delete",
		"content": "This memory will be deleted.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	decodeJSON(t, resp, &created)
	memID := firstString(created, "ID", "id")

	delResp := ts.do(t, http.MethodDelete, "/api/v1/stores/user-1/memories/"+memID, tok, nil)
	require.Equal(t, http.StatusOK, delResp.StatusCode)

	getResp := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories/"+memID, tok, nil)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)

}

func TestMemory_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories", "", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

}

func TestMemory_ForbiddenStore(t *testing.T) {
	ts := newTestServer(t)

	// user-1 authenticates and gets their personal store auto-created.
	tok1 := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)
	resp := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories", tok1, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// user-2 tries to access user-1's personal store.
	tok2 := ts.token(t, "user-2", "User Two", domain.InstanceRoleUser)
	resp2 := ts.do(t, http.MethodGet, "/api/v1/stores/user-1/memories", tok2, nil)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)

}

func TestMemory_NoContent(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Empty",
		"content": "",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

}

// firstString returns the value of the first key found in the map as a string.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
