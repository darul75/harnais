package graph

import (
	"testing"
)

func TestActivityCallLinking(t *testing.T) {

	run := NewRun(
		"run-test",
		nil,
		State{},
	)

	execution := run.StartAgentExecution(
		"node-exec",
		"agent-test",
	)

	llmActivity := run.StartAgentActivity(
		execution.ID,
		1,
		ActivityLLM,
	)

	llmCall := run.StartLLMCall(
		execution.ID,
		llmActivity.ID,
		1,
		[]MessageRecord{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	)

	toolActivity := run.StartAgentActivity(
		execution.ID,
		2,
		ActivityTool,
	)

	toolCall := run.StartToolCall(
		execution.ID,
		toolActivity.ID,
		1,
		"echo",
		map[string]any{
			"text": "hi",
		},
	)

	if llmActivity.LLMCallID == nil ||
		*llmActivity.LLMCallID !=
			llmCall.ID {
		t.Fatalf(
			"expected LLMCallID %q, got %v",
			llmCall.ID,
			llmActivity.LLMCallID,
		)
	}

	if toolActivity.ToolCallID == nil ||
		*toolActivity.ToolCallID !=
			toolCall.ID {
		t.Fatalf(
			"expected ToolCallID %q, got %v",
			toolCall.ID,
			toolActivity.ToolCallID,
		)
	}

	if llmActivity.Kind !=
		ActivityLLM {
		t.Fatalf(
			"expected llm kind, got %q",
			llmActivity.Kind,
		)
	}

	if toolActivity.Kind !=
		ActivityTool {
		t.Fatalf(
			"expected tool kind, got %q",
			toolActivity.Kind,
		)
	}
}