package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
)

// Compile-time interface check.
var _ conversationuc.LLMProvider = (*OpenAIProvider)(nil)

// OpenAIProvider implements LLMProvider using the OpenAI Chat Completions API.
// It can be used with OpenAI directly or any OpenAI-compatible endpoint such as Ollama.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates an OpenAI LLM provider.
// apiKey is the API key for authentication.
// model is the chat model to use (e.g. "gpt-4o-mini").
// baseURL can be empty to use the default OpenAI endpoint, or set to a custom URL
// for OpenAI-compatible services (e.g. Ollama at "http://localhost:11434/v1").
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &OpenAIProvider{client: client, model: model}
}

// Summarize uses the Chat Completions API to summarize conversation messages.
// systemPrompt is prepended as a system message. User and assistant messages are
// forwarded; system and tool messages from domain are skipped.
func (p *OpenAIProvider) Summarize(ctx context.Context, messages []*domain.Message, systemPrompt string) (string, error) {
	chatMessages := buildOpenAIChatMessages(messages, systemPrompt)

	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.F(openai.ChatModel(p.model)),
		Messages: openai.F(chatMessages),
	})
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeLLMError, fmt.Sprintf("openai summarize failed: %v", err))
	}

	if len(resp.Choices) == 0 {
		return "", domain.NewAppError(domain.ErrCodeLLMError, "openai returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// buildOpenAIChatMessages constructs the chat message slice from domain messages and a system prompt.
func buildOpenAIChatMessages(messages []*domain.Message, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	var out []openai.ChatCompletionMessageParamUnion

	if systemPrompt != "" {
		out = append(out, openai.SystemMessage(systemPrompt))
	}

	for _, m := range messages {
		switch m.Role {
		case domain.MessageRoleUser:
			out = append(out, openai.UserMessage(m.Content))
		case domain.MessageRoleAssistant:
			out = append(out, openai.AssistantMessage(m.Content))
		default:
			// Skip system and tool messages from domain.
		}
	}

	return out
}
