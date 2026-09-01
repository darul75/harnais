package llm

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"harnais/agent"
)

// TestParallelToolCallsLoop drives the provider through a loop that
// forces two parallel tool calls in one response and verifies the
// continuation no longer 400s. Skipped unless HARNAIS_REPRO=1.
func TestParallelToolCallsLoop(t *testing.T) {

	if os.Getenv("HARNAIS_REPRO") == "" {
		t.Skip("set HARNAIS_REPRO=1")
	}

	provider := NewOpenAI("", "")

	tools := []agent.ToolDefinition{}

	for _, name := range []string{"echo_one", "echo_two"} {

		tools = append(tools, agent.ToolDefinition{
			Name: name,

			Description: "Echo back the given text.",

			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{"text": map[string]any{"type": "string"}},
				"required":             []string{"text"},
				"additionalProperties": false,
			},
		})
	}

	messages := []agent.Message{
		{
			Role:    "user",
			Content: "In this SINGLE response, call BOTH the echo_one and echo_two tools, then reply with the word DONE.",
		},
	}

	for round := 0; round < 6; round++ {

		response, err :=
			provider.Generate(
				context.Background(),
				messages,
				tools,
			)

		if err != nil {
			t.Fatalf(
				"round %d Generate: %v",
				round,
				err,
			)
		}

		if len(response.ToolCalls) == 0 {
			t.Logf(
				"round %d: final answer %q",
				round,
				response.Text,
			)
			return
		}

		t.Logf(
			"round %d: %d tool calls",
			round,
			len(response.ToolCalls),
		)

		for _, call := range response.ToolCalls {

			output, _ :=
				json.Marshal(call.Input)

			t.Logf(
				"  %s input=%s call_id=%s",
				call.Name,
				output,
				call.CallID,
			)

			messages =
				append(
					messages,
					agent.Message{
						Role:    "tool",
						Content: "echoed",
						CallID:  call.CallID,
					},
				)
		}
	}

	t.Fatal("loop did not finish")
}