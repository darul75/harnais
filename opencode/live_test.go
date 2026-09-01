package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"harnais/graph"
)

// TestLiveOpenCodeRun is a manual live check of the OpenCode
// subprocess integration. It is skipped unless HARNAIS_OPENCODE_LIVE=1
// because it makes a real model call and writes a session.
func TestLiveOpenCodeRun(t *testing.T) {

	if os.Getenv("HARNAIS_OPENCODE_LIVE") == "" {
		t.Skip("live test skipped (set HARNAIS_OPENCODE_LIVE=1)")
	}

	dir := filepath.Join(
		t.TempDir(),
		"oc-workspace",
	)

	if err :=
		os.MkdirAll(
			dir,
			0o755,
		); err != nil {

		t.Fatal(err)
	}

	run :=
		graph.NewRun(
			"run-live",
			nil,
			graph.State{},
		)

	ctx :=
		graph.WithExecutionContext(
			context.Background(),
			graph.ExecutionContext{
				RunID:       run.ID,
				ExecutionID: "exec-live",
				NodeID:      "coder",
				Run:         run,
			},
		)

	worker := &Worker{
		AgentID: "opencode-coder",

		Prompt: "Work in the current directory. " +
			"Create a file named hello.txt containing the text 'hi'." +
			"Then reply with a one-line summary.",

		Dir: dir,

		Timeout: 3 * time.Minute,
	}

	result, err :=
		worker.Run(
			ctx,
			graph.WorkerInput{
				State: graph.State{
					"task": "create hello.txt",
				},
			},
		)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.State["output"] == "" {
		t.Error("expected non-empty output")
	} else {
		t.Logf("output: %s", result.State["output"])
	}

	content, readErr :=
		os.ReadFile(
			filepath.Join(
				dir,
				"hello.txt",
			),
		)

	if readErr != nil {
		t.Fatalf("hello.txt not created: %v", readErr)
	}

	if string(content) != "hi" {
		t.Errorf(
			"expected hello.txt content 'hi', got %q",
			string(content),
		)
	}

	agents := run.AgentExecutions

	if len(agents) != 1 {
		t.Fatalf(
			"expected 1 agent execution, got %d",
			len(agents),
		)
	}

	if agents[0].Status != graph.StatusCompleted {
		t.Errorf(
			"expected completed agent, got %s",
			agents[0].Status,
		)
	}

	if len(run.LLMCalls) == 0 {
		t.Error(
			"expected at least one llm call recorded",
		)
	}

	if len(run.ToolCalls) == 0 {
		t.Error(
			"expected at least one tool call recorded",
		)
	}

	if len(agents[0].Activities) == 0 {
		t.Error(
			"expected recorded activities",
		)
	}
}
