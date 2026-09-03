package opencode

import (
	"context"
	"encoding/json"
	"testing"

	"harnais/graph"
)

func testRecorder(
	t *testing.T,
) (*activityRecorder, *graph.Run) {
	t.Helper()

	run :=
		graph.NewRun(
			"run-test",
			nil,
			graph.State{},
		)

	execution :=
		run.StartAgentExecution(
			"exec-test",
			"opencode-coder",
		)

	recorder :=
		newActivityRecorder(
			context.Background(),
			run,
			"opencode-coder",
			execution.ID,
			"implement it",
		)

	return recorder, run
}

func TestRecorderTextOnly(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"text","sessionID":"ses_1","part":{"id":"p1","type":"text","text":"Hello world"}}`),
	)

	sessionID, text :=
		recorder.output()

	if sessionID != "ses_1" {
		t.Errorf(
			"expected session ses_1, got %q",
			sessionID,
		)
	}

	if text != "Hello world" {
		t.Errorf(
			"expected text, got %q",
			text,
		)
	}
}

func TestRecorderDeltaAppend(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"Hel"}}`),
	)
	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"lo"}}`),
	)
	recorder.process(
		[]byte(`{"type":"step_finish","sessionID":"s","part":{"id":"p2","type":"step-finish"}}`),
	)

	_, text :=
		recorder.output()

	if text != "Hello" {
		t.Errorf(
			"expected Hello, got %q",
			text,
		)
	}
}

func TestRecorderSnapshotGrowth(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"Hel"}}`),
	)
	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"Hello"}}`),
	)

	_, text :=
		recorder.output()

	if text != "Hello" {
		t.Errorf(
			"expected Hello, got %q",
			text,
		)
	}
}

func TestRecorderMultipleParts(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"One. "}}`),
	)
	recorder.process(
		[]byte(`{"type":"text","sessionID":"s","part":{"id":"p2","type":"text","text":"Two."}}`),
	)

	_, text :=
		recorder.output()

	if text != "One. Two." {
		t.Errorf(
			"expected 'One. Two.', got %q",
			text,
		)
	}
}

func TestRecorderRecordsLLMAndTool(t *testing.T) {

	recorder, run :=
		testRecorder(t)

	lines := []string{
		`{"type":"step_start","sessionID":"s","part":{"id":"a1","type":"step-start"}}`,
		`{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"write","callID":"call_1","state":{"status":"completed","input":{"filePath":"a.txt"},"output":"Wrote file successfully."}}}`,
		`{"type":"text","sessionID":"s","part":{"id":"p1","type":"text","text":"Created the file."}}`,
		`{"type":"step_finish","sessionID":"s","part":{"id":"a2","type":"step-finish"}}`,
	}

	for _, line := range lines {
		recorder.process([]byte(line))
	}

	if len(run.AgentExecutions) != 1 {
		t.Fatalf(
			"expected 1 agent execution, got %d",
			len(run.AgentExecutions),
		)
	}

	execution := run.AgentExecutions[0]

	if len(execution.Activities) != 2 {
		t.Fatalf(
			"expected 2 activities, got %d",
			len(execution.Activities),
		)
	}

	if execution.Activities[0].Kind != graph.ActivityLLM {
		t.Errorf(
			"expected first activity llm, got %q",
			execution.Activities[0].Kind,
		)
	}

	if execution.Activities[1].Kind != graph.ActivityTool {
		t.Errorf(
			"expected second activity tool, got %q",
			execution.Activities[1].Kind,
		)
	}

	if len(run.LLMCalls) != 1 {
		t.Fatalf(
			"expected 1 llm call, got %d",
			len(run.LLMCalls),
		)
	}

	llmCall := run.LLMCalls[0]

	if llmCall.Response != "Created the file." {
		t.Errorf(
			"expected llm response, got %q",
			llmCall.Response,
		)
	}

	if llmCall.RequestedTool != "write" {
		t.Errorf(
			"expected requested tool write, got %q",
			llmCall.RequestedTool,
		)
	}

	if len(llmCall.Messages) != 1 ||
		llmCall.Messages[0].Content != "implement it" {
		t.Errorf(
			"expected first llm call to carry the prompt",
		)
	}

	if len(run.ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			len(run.ToolCalls),
		)
	}

	toolCall := run.ToolCalls[0]

	if toolCall.ToolID != "write" {
		t.Errorf(
			"expected tool write, got %q",
			toolCall.ToolID,
		)
	}

	if toolCall.Status != graph.StatusCompleted {
		t.Errorf(
			"expected tool completed, got %q",
			toolCall.Status,
		)
	}
}

func TestRecorderToolError(t *testing.T) {

	recorder, run :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"bash","callID":"call_2","state":{"status":"error","output":"boom"}}}`),
	)

	if len(run.ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			len(run.ToolCalls),
		)
	}

	if run.ToolCalls[0].Status != graph.StatusFailed {
		t.Errorf(
			"expected tool failed, got %q",
			run.ToolCalls[0].Status,
		)
	}
}

func TestRecorderCapturesErrorMessage(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"error","sessionID":"s","error":{"name":"APIError","data":{"message":"Your workspace has reached its monthly spending limit","statusCode":401}}}`),
	)

	if recorder.errorMessage !=
		"Your workspace has reached its monthly spending limit" {

		t.Errorf(
			"expected nested data message, got %q",
			recorder.errorMessage,
		)
	}
}

func TestRecorderCapturesTopLevelErrorMessage(t *testing.T) {

	recorder, _ :=
		testRecorder(t)

	recorder.process(
		[]byte(`{"type":"error","error":{"message":"top level boom"}}`),
	)

	if recorder.errorMessage !=
		"top level boom" {

		t.Errorf(
			"expected top level message, got %q",
			recorder.errorMessage,
		)
	}
}

func TestWorkerIDDefault(t *testing.T) {
	if got := (&Worker{}).ID(); got != "opencode" {
		t.Errorf(
			"expected default id, got %q",
			got,
		)
	}

	if got := (&Worker{AgentID: "coder"}).ID(); got != "coder" {
		t.Errorf(
			"expected coder id, got %q",
			got,
		)
	}
}

func TestRecorderRecordParts(t *testing.T) {

	recorder, run :=
		testRecorder(t)

	parts := []byte(`[
		{"type":"text","text":"Done."},
		{"type":"tool","tool":"write","callID":"call_1","state":{"status":"completed","input":{"filePath":"a.txt"},"output":"Wrote"}},
		{"type":"tool","tool":"bash","callID":"call_2","state":{"status":"error","input":{"command":"go test"},"error":"boom"}}
	]`)

	recorder.recordParts(parts)

	if !recorder.hasRecorded() {
		t.Fatal("expected activities to be recorded from parts")
	}

	if len(run.LLMCalls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(run.LLMCalls))
	}

	if len(run.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(run.ToolCalls))
	}

	if run.LLMCalls[0].Response != "Done." {
		t.Fatalf("unexpected response %q", run.LLMCalls[0].Response)
	}

	if run.ToolCalls[1].Error == "" {
		t.Fatal("expected tool error to be recorded")
	}
}

func TestRecorderRecordPartsSkippedWhenLive(t *testing.T) {

	recorder, run :=
		testRecorder(t)

	recorder.processServerEvent(
		ServerEvent{
			Type:       "session.next.step.started",
			Properties: map[string]any{},
		},
	)

	recorder.recordParts(
		[]byte(`[{"type":"text","text":"X"}]`),
	)

	if len(run.LLMCalls) != 1 {
		t.Fatalf(
			"expected only the live LLM call, got %d",
			len(run.LLMCalls),
		)
	}
}

func TestRecorderSyncMessages(t *testing.T) {

	recorder, run :=
		testRecorder(t)

	messages := []SessionMessage{
		{
			Info: SessionMessageInfo{
				ID:   "msg_1",
				Role: "assistant",
			},

			Parts: []json.RawMessage{
				json.RawMessage(`{"id":"p1","type":"reasoning","text":"think"}`),
				json.RawMessage(`{"id":"p2","type":"text","text":"Hello"}`),
				json.RawMessage(`{"id":"p3","type":"tool","tool":"bash","callID":"c1","state":{"status":"completed","input":{"command":"go test"},"output":"ok"}}`),
			},
		},
	}

	recorder.syncMessages(messages)

	if len(run.LLMCalls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(run.LLMCalls))
	}

	if run.LLMCalls[0].Response != "Hello" {
		t.Fatalf("unexpected response %q", run.LLMCalls[0].Response)
	}

	if run.LLMCalls[0].Reasoning != "think" {
		t.Fatalf("unexpected reasoning %q", run.LLMCalls[0].Reasoning)
	}

	if len(run.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(run.ToolCalls))
	}

	if got :=
		run.ToolCalls[0].Output["output"]; got != "ok" {
		t.Fatalf("unexpected tool output %v", got)
	}

	recorder.finalize()

	if run.LLMCalls[0].Status != graph.StatusCompleted {
		t.Fatalf(
			"expected completed LLM call, got %v",
			run.LLMCalls[0].Status,
		)
	}
}
