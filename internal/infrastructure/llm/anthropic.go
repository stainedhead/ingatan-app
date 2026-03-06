// Package llm provides LLM provider adapters for ingatan.
package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
)

// Compile-time interface check.
var _ conversationuc.LLMProvider = (*AnthropicProvider)(nil)

// AnthropicProvider implements LLMProvider using the Anthropic Messages API.
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicProvider creates an Anthropic LLM provider.
// model should be e.g. "claude-3-5-haiku-latest".
// If baseURL is non-empty it overrides the default Anthropic API endpoint (useful for testing).
func NewAnthropicProvider(apiKey, model, baseURL string) *AnthropicProvider {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	c := anthropic.NewClient(opts...)
	return &AnthropicProvider{client: &c, model: model}
}

// Summarize calls the Anthropic Messages API to produce a summary of conversation messages.
// systemPrompt is sent as the system instruction. User and assistant messages are forwarded;
// system and tool messages are skipped as the Anthropic Messages API does not accept them
// in the messages array.
func (p *AnthropicProvider) Summarize(ctx context.Context, messages []*domain.Message, systemPrompt string) (string, error) {
	apiMessages := convertToAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		Messages:  apiMessages,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeLLMError, fmt.Sprintf("anthropic summarize failed: %v", err))
	}

	// Extract first text block from the response.
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			return tb.Text, nil
		}
	}

	return "", domain.NewAppError(domain.ErrCodeLLMError, "anthropic returned no text content")
}

// convertToAnthropicMessages converts domain messages to Anthropic API message params.
// System and tool messages are skipped as they are not valid in the messages array.
func convertToAnthropicMessages(messages []*domain.Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range messages {
		switch m.Role {
		case domain.MessageRoleUser:
			out = append(out, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(m.Content)},
			})
		case domain.MessageRoleAssistant:
			out = append(out, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(m.Content)},
			})
		default:
			// Skip system and tool messages.
		}
	}
	return out
}
