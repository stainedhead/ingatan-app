package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// mockAnthropicServer returns a test HTTP server that responds with a valid Anthropic message response.
func mockAnthropicServer(t *testing.T, textContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":    "msg_test",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-haiku-latest",
			"content": []map[string]any{
				{"type": "text", "text": textContent},
			},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 20,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("mock server: encode response: %v", err)
		}
	}))
}

// mockAnthropicErrorServer returns a test HTTP server that responds with an Anthropic API error.
func mockAnthropicErrorServer(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		resp := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("mock server: encode error response: %v", err)
		}
	}))
}

// TestAnthropicProvider_New verifies that NewAnthropicProvider does not panic.
func TestAnthropicProvider_New(t *testing.T) {
	p := NewAnthropicProvider("test-key", "claude-3-5-haiku-latest", "")
	require.NotNil(t, p)
	assert.Equal(t, "claude-3-5-haiku-latest", p.model)
}

// TestAnthropicProvider_Summarize_Success verifies that Summarize returns the text from the API response.
func TestAnthropicProvider_Summarize_Success(t *testing.T) {
	srv := mockAnthropicServer(t, "This is a summary.")
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-3-5-haiku-latest", srv.URL)

	messages := []*domain.Message{
		{
			MessageID:      "m1",
			ConversationID: "conv-1",
			Role:           domain.MessageRoleUser,
			Content:        "Hello",
			CreatedAt:      time.Now(),
		},
		{
			MessageID:      "m2",
			ConversationID: "conv-1",
			Role:           domain.MessageRoleAssistant,
			Content:        "Hi there!",
			CreatedAt:      time.Now(),
		},
	}

	summary, err := p.Summarize(context.Background(), messages, "Summarize this conversation.")
	require.NoError(t, err)
	assert.Equal(t, "This is a summary.", summary)
}

// TestAnthropicProvider_Summarize_EmptyMessages verifies behaviour with no messages.
func TestAnthropicProvider_Summarize_EmptyMessages(t *testing.T) {
	srv := mockAnthropicServer(t, "No content to summarize.")
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-3-5-haiku-latest", srv.URL)

	summary, err := p.Summarize(context.Background(), nil, "Summarize.")
	require.NoError(t, err)
	assert.Equal(t, "No content to summarize.", summary)
}

// TestAnthropicProvider_Summarize_SkipsSystemAndToolMessages verifies that system and tool messages are filtered.
func TestAnthropicProvider_Summarize_SkipsSystemAndToolMessages(t *testing.T) {
	srv := mockAnthropicServer(t, "Filtered summary.")
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-3-5-haiku-latest", srv.URL)

	messages := []*domain.Message{
		{MessageID: "m1", Role: domain.MessageRoleSystem, Content: "You are helpful.", CreatedAt: time.Now()},
		{MessageID: "m2", Role: domain.MessageRoleUser, Content: "Hello", CreatedAt: time.Now()},
		{MessageID: "m3", Role: domain.MessageRoleTool, Content: "{}", CreatedAt: time.Now()},
	}

	summary, err := p.Summarize(context.Background(), messages, "")
	require.NoError(t, err)
	assert.Equal(t, "Filtered summary.", summary)
}

// TestAnthropicProvider_Summarize_APIError verifies that an API error is wrapped in a domain AppError.
func TestAnthropicProvider_Summarize_APIError(t *testing.T) {
	srv := mockAnthropicErrorServer(t, http.StatusUnauthorized)
	defer srv.Close()

	p := NewAnthropicProvider("bad-key", "claude-3-5-haiku-latest", srv.URL)

	_, err := p.Summarize(context.Background(), nil, "")
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeLLMError, appErr.Code)
}

// TestConvertToAnthropicMessages verifies that message conversion filters correctly.
func TestConvertToAnthropicMessages(t *testing.T) {
	messages := []*domain.Message{
		{Role: domain.MessageRoleUser, Content: "hello"},
		{Role: domain.MessageRoleAssistant, Content: "hi"},
		{Role: domain.MessageRoleSystem, Content: "system"},
		{Role: domain.MessageRoleTool, Content: "{}"},
		{Role: domain.MessageRoleUser, Content: "goodbye"},
	}

	result := convertToAnthropicMessages(messages)

	// Should only include user and assistant messages.
	require.Len(t, result, 3)
}
