package integration_test

import (
	"net/http"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversation_StartAndGet(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/conversations", tok, map[string]any{
		"title": "Test Conversation",
		"store": "user-1",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	decodeJSON(t, resp, &created)
	convID := firstString(created, "ConversationID", "conversation_id")
	require.NotEmpty(t, convID, "conversation ID should be present")
	assert.Equal(t, "Test Conversation", firstString(created, "Title", "title"))

	// Get it back.
	resp2 := ts.do(t, http.MethodGet, "/api/v1/conversations/"+convID, tok, nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var got map[string]any
	decodeJSON(t, resp2, &got)
	conv, _ := got["conversation"].(map[string]any)
	assert.Equal(t, convID, firstString(conv, "ConversationID", "conversation_id"))
}

func TestConversation_AddMessage(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Start a conversation.
	resp := ts.do(t, http.MethodPost, "/api/v1/conversations", tok, map[string]any{
		"title": "Message Test",
		"store": "user-1",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	decodeJSON(t, resp, &created)
	convID := firstString(created, "ConversationID", "conversation_id")

	// Add a user message.
	resp2 := ts.do(t, http.MethodPost, "/api/v1/conversations/"+convID+"/messages", tok, map[string]any{
		"role":    "user",
		"content": "Hello, this is a test message.",
	})
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	var msg map[string]any
	decodeJSON(t, resp2, &msg)
	assert.Equal(t, "user", firstString(msg, "Role", "role"))
	assert.Equal(t, "Hello, this is a test message.", firstString(msg, "Content", "content"))

	// Verify the message appears in the conversation.
	resp3 := ts.do(t, http.MethodGet, "/api/v1/conversations/"+convID, tok, nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)

	var got map[string]any
	decodeJSON(t, resp3, &got)
	messages, _ := got["messages"].([]any)
	assert.Len(t, messages, 1)
}

func TestConversation_List(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Start two conversations.
	for i := 0; i < 2; i++ {
		resp := ts.do(t, http.MethodPost, "/api/v1/conversations", tok, map[string]any{
			"title": "Conversation",
			"store": "user-1",
		})
		require.Equal(t, http.StatusCreated, resp.StatusCode)

	}

	resp := ts.do(t, http.MethodGet, "/api/v1/conversations", tok, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list map[string]any
	decodeJSON(t, resp, &list)
	conversations, _ := list["conversations"].([]any)
	assert.Len(t, conversations, 2)
}

func TestConversation_Delete(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/conversations", tok, map[string]any{
		"title": "To Delete",
		"store": "user-1",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	decodeJSON(t, resp, &created)
	convID := firstString(created, "ConversationID", "conversation_id")

	// Delete with confirm.
	delResp := ts.do(t, http.MethodDelete, "/api/v1/conversations/"+convID, tok, map[string]any{
		"confirm": convID,
	})
	require.Equal(t, http.StatusOK, delResp.StatusCode)

	// List should be empty.
	listResp := ts.do(t, http.MethodGet, "/api/v1/conversations", tok, nil)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var list map[string]any
	decodeJSON(t, listResp, &list)
	conversations, _ := list["conversations"].([]any)
	assert.Empty(t, conversations)
}
