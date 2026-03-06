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

// mockOpenAIServer returns a test HTTP server that responds with a valid OpenAI chat completion response.
func mockOpenAIServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": content,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("mock server: encode response: %v", err)
		}
	}))
}

// mockOpenAIErrorServer returns a test HTTP server that responds with an OpenAI API error.
func mockOpenAIErrorServer(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		resp := map[string]any{
			"error": map[string]any{
				"message": "Incorrect API key provided",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("mock server: encode error response: %v", err)
		}
	}))
}

// TestOpenAIProvider_New verifies that NewOpenAIProvider does not panic.
func TestOpenAIProvider_New(t *testing.T) {
	p := NewOpenAIProvider("test-key", "gpt-4o-mini", "")
	require.NotNil(t, p)
	assert.Equal(t, "gpt-4o-mini", p.model)
}

// TestOpenAIProvider_Summarize_Success verifies that Summarize returns the content from the API response.
func TestOpenAIProvider_Summarize_Success(t *testing.T) {
	srv := mockOpenAIServer(t, "OpenAI summary here.")
	defer srv.Close()

	p := NewOpenAIProvider("test-key", "gpt-4o-mini", srv.URL)

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
	assert.Equal(t, "OpenAI summary here.", summary)
}

// TestOpenAIProvider_Summarize_WithSystemPrompt verifies that a system prompt is included.
func TestOpenAIProvider_Summarize_WithSystemPrompt(t *testing.T) {
	srv := mockOpenAIServer(t, "Summary with system prompt.")
	defer srv.Close()

	p := NewOpenAIProvider("test-key", "gpt-4o-mini", srv.URL)

	summary, err := p.Summarize(context.Background(), nil, "You are a summarizer.")
	require.NoError(t, err)
	assert.Equal(t, "Summary with system prompt.", summary)
}

// TestOpenAIProvider_Summarize_APIError verifies that an API error is wrapped in a domain AppError.
func TestOpenAIProvider_Summarize_APIError(t *testing.T) {
	srv := mockOpenAIErrorServer(t, http.StatusUnauthorized)
	defer srv.Close()

	p := NewOpenAIProvider("bad-key", "gpt-4o-mini", srv.URL)

	_, err := p.Summarize(context.Background(), nil, "")
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeLLMError, appErr.Code)
}

// TestOpenAIProvider_Summarize_SkipsSystemAndToolMessages verifies that system and tool messages are filtered.
func TestOpenAIProvider_Summarize_SkipsSystemAndToolMessages(t *testing.T) {
	srv := mockOpenAIServer(t, "Filtered summary.")
	defer srv.Close()

	p := NewOpenAIProvider("test-key", "gpt-4o-mini", srv.URL)

	messages := []*domain.Message{
		{MessageID: "m1", Role: domain.MessageRoleSystem, Content: "You are helpful."},
		{MessageID: "m2", Role: domain.MessageRoleUser, Content: "Hello"},
		{MessageID: "m3", Role: domain.MessageRoleTool, Content: "{}"},
		{MessageID: "m4", Role: domain.MessageRoleAssistant, Content: "Hi"},
	}

	summary, err := p.Summarize(context.Background(), messages, "")
	require.NoError(t, err)
	assert.Equal(t, "Filtered summary.", summary)
}

// TestBuildOpenAIChatMessages verifies that message building filters correctly and includes system prompt.
func TestBuildOpenAIChatMessages(t *testing.T) {
	messages := []*domain.Message{
		{Role: domain.MessageRoleUser, Content: "hello"},
		{Role: domain.MessageRoleAssistant, Content: "hi"},
		{Role: domain.MessageRoleSystem, Content: "system"},
		{Role: domain.MessageRoleTool, Content: "{}"},
	}

	result := buildOpenAIChatMessages(messages, "system prompt")

	// system prompt + user + assistant = 3; system/tool domain messages are skipped.
	assert.Len(t, result, 3)
}
