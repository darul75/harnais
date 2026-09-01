package agent

import "context"

type LLMResponse struct {
	Text string

	// ToolCalls are the tool calls the model requested. A single
	// response may contain several (parallel tool calling); the
	// caller must execute all of them before continuing.
	ToolCalls []*ToolCall

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
