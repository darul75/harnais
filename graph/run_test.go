package graph

import (
	"testing"
	"time"
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

func TestIsFinishedIgnoresStaleActivations(t *testing.T) {

	executor :=
		NewExecutor(nil, nil)

	now := time.Now()

	run :=
		NewRun(
			"run-stale",
			nil,
			State{},
		)

	run.AddExecution(
		&NodeExecution{
			ID:          "exec-reviewer",
			NodeID:      "reviewer",
			Status:      StatusCompleted,
			StartedAt:   now,
			CompletedAt: &now,
		},
	)

	// A stale activation left when a source node fired more times than
	// the JoinAll target consumed must not block completion.
	run.AddEdgeActivation(
		&EdgeActivation{
			ID:         "activation-stale",
			EdgeID:     "security->reviewer",
			FromNodeID: "security",
			ToNodeID:   "reviewer",
			CreatedAt:  now,
		},
	)

	if !executor.isFinished(run) {
		t.Fatal(
			"run with only a stale activation should be finished",
		)
	}

	// Pending work for an un-run node must still block completion.
	run.AddEdgeActivation(
		&EdgeActivation{
			ID:         "activation-pending",
			EdgeID:     "x->y",
			FromNodeID: "x",
			ToNodeID:   "y",
			CreatedAt:  now,
		},
	)

	if executor.isFinished(run) {
		t.Fatal(
			"run with pending work for an un-run node should not be finished",
		)
	}
}
