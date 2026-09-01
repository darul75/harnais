package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"harnais/agent"
)

func TestGenerateReturnsAllToolCalls(t *testing.T) {

	var lastBody responseRequest

	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {

				_ = json.NewDecoder(
					r.Body,
				).Decode(
					&lastBody,
				)

				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				isContinuation :=
					len(lastBody.Input) > 0 &&
						lastBody.Input[0].Type ==
							"function_call_output"

				if isContinuation {

					_ = json.NewEncoder(
						w,
					).Encode(
						responseResponse{
							ID: "resp_2",

							Output: []responseOutput{
								{
									Type: "message",

									Content: []responseContent{
										{
											Type: "output_text",
											Text: "DONE",
										},
									},
								},
							},
						},
					)

					return
				}

				_ = json.NewEncoder(
					w,
				).Encode(
					responseResponse{
						ID: "resp_1",

						Output: []responseOutput{
							{
								Type:      "function_call",
								Name:      "echo_one",
								Arguments: `{"text":"hi"}`,
								CallID:    "call_a",
							},
							{
								Type:      "function_call",
								Name:      "echo_two",
								Arguments: `{"text":"yo"}`,
								CallID:    "call_b",
							},
						},
					},
				)
			},
		),
	)

	defer server.Close()

	provider := &OpenAI{
		APIKey:  "sk-test",
		Model:   "gpt-test",
		Client:  server.Client(),
		BaseURL: server.URL,
	}

	tools := []agent.ToolDefinition{
		{Name: "echo_one", Description: "echo", Parameters: map[string]any{}},
		{Name: "echo_two", Description: "echo", Parameters: map[string]any{}},
	}

	// First call: response with two parallel function calls.
	first, err :=
		provider.Generate(
			context.Background(),
			[]agent.Message{
				{
					Role:    "user",
					Content: "call both",
				},
			},
			tools,
		)

	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}

	if len(first.ToolCalls) != 2 {
		t.Fatalf(
			"expected 2 tool calls, got %d",
			len(first.ToolCalls),
		)
	}

	if first.ToolCalls[0].CallID != "call_a" ||
		first.ToolCalls[1].CallID != "call_b" {
		t.Errorf(
			"unexpected call ids: %q %q",
			first.ToolCalls[0].CallID,
			first.ToolCalls[1].CallID,
		)
	}

	// Continuation: provide output for BOTH pending calls.
	second, err :=
		provider.Generate(
			context.Background(),
			[]agent.Message{
				{Role: "tool", Content: "one", CallID: "call_a"},
				{Role: "tool", Content: "two", CallID: "call_b"},
			},
			tools,
		)

	if err != nil {
		t.Fatalf("continuation Generate: %v", err)
	}

	if second.Text != "DONE" {
		t.Errorf(
			"expected DONE, got %q",
			second.Text,
		)
	}

	if len(lastBody.Input) != 2 {
		t.Fatalf(
			"expected 2 function_call_output items, got %d",
			len(lastBody.Input),
		)
	}

	// Both outputs must be present in the continuation.
	found := map[string]bool{}

	for _, item := range lastBody.Input {
		if item.Type != "function_call_output" {
			continue
		}
		found[item.CallID] = true
	}

	if !found["call_a"] || !found["call_b"] {
		t.Errorf(
			"continuation missing outputs, got %v",
			lastBody.Input,
		)
	}
}

func TestContinuationDoesNotResendAnsweredCalls(t *testing.T) {

	var lastBody responseRequest

	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {

				_ = json.NewDecoder(
					r.Body,
				).Decode(
					&lastBody,
				)

				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				_ = json.NewEncoder(
					w,
				).Encode(
					responseResponse{
						ID: "resp_x",

						Output: []responseOutput{
							{
								Type: "message",

								Content: []responseContent{
									{
										Type: "output_text",
										Text: "ok",
									},
								},
							},
						},
					},
				)
			},
		),
	)

	defer server.Close()

	provider := &OpenAI{
		APIKey:  "sk-test",
		Model:   "gpt-test",
		Client:  server.Client(),
		BaseURL: server.URL,

		// Force continuation mode so the first call answers calls.
		PreviousResponseID: "resp_prev",
	}

	// The first continuation answers call_a and call_b.
	if _, err :=
		provider.Generate(
			context.Background(),
			[]agent.Message{
				{Role: "tool", Content: "one", CallID: "call_a"},
				{Role: "tool", Content: "two", CallID: "call_b"},
			},
			nil,
		); err != nil {

		t.Fatalf("first continuation: %v", err)
	}

	// A later continuation for a new call must NOT resend call_a/call_b.
	if _, err :=
		provider.Generate(
			context.Background(),
			[]agent.Message{
				{Role: "tool", Content: "one", CallID: "call_a"},
				{Role: "tool", Content: "two", CallID: "call_b"},
				{Role: "tool", Content: "three", CallID: "call_c"},
			},
			nil,
		); err != nil {

		t.Fatalf("second continuation: %v", err)
	}

	if len(lastBody.Input) != 1 {
		t.Fatalf(
			"expected only 1 new output, got %d",
			len(lastBody.Input),
		)
	}

	if lastBody.Input[0].CallID != "call_c" {
		t.Errorf(
			"expected only call_c to be resent, got %q",
			lastBody.Input[0].CallID,
		)
	}
}
