package agent

import "context"

type LLMResponse struct {
	Text string

	ToolCall *ToolCall

	// Provider-specific continuation identifier.
	// For OpenAI this is the Responses API response ID.
	ResponseID string
}

type ToolCall struct {
	Name string

	Input map[string]any

	CallID string
}

type Message struct {
	Role string

	Content string

	// Set when this message represents the output
	// of a tool/function call.
	CallID string
}

type ToolDefinition struct {
	Name string

	Description string

	Parameters map[string]any
}

type LLM interface {
	Generate(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
	) (LLMResponse, error)
}
