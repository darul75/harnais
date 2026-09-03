package workflows

import (
	"context"
	"testing"
	"time"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

func TestPlanGateApprove(t *testing.T) {

	hub := graph.NewQuestionHub()

	s := NewShared(
		tools.NewWorkspace(t.TempDir()),
		config.NewStore(""),
		hub,
	)

	ctx := graph.WithExecutionContext(
		context.Background(),
		graph.ExecutionContext{
			RunID: "run-1",
		},
	)

	gate := s.PlanGate()

	resultCh := make(chan graph.WorkerResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := gate.Run(
			ctx,
			graph.WorkerInput{
				State: graph.State{
					"plan": "the plan",
				},
			},
		)
		resultCh <- result
		errCh <- err
	}()

	replyTo(t, hub, "run-1", "plan_gate_approve_1", [][]string{{"Approve"}})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("PlanGate error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("PlanGate did not return")
	}

	result := <-resultCh

	if approved, _ := result.State["plan_approved"].(bool); !approved {
		t.Fatalf("expected approved=true, got %v", result.State)
	}

	if attempts, _ := result.State["plan_attempts"].(int); attempts != 1 {
		t.Fatalf("expected attempts=1, got %v", result.State)
	}
}

func TestPlanGateRequestChanges(t *testing.T) {

	hub := graph.NewQuestionHub()

	s := NewShared(
		tools.NewWorkspace(t.TempDir()),
		config.NewStore(""),
		hub,
	)

	ctx := graph.WithExecutionContext(
		context.Background(),
		graph.ExecutionContext{
			RunID: "run-1",
		},
	)

	gate := s.PlanGate()

	resultCh := make(chan graph.WorkerResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := gate.Run(
			ctx,
			graph.WorkerInput{
				State: graph.State{
					"plan": "the plan",
				},
			},
		)
		resultCh <- result
		errCh <- err
	}()

	replyTo(t, hub, "run-1", "plan_gate_approve_1", [][]string{{"Request changes"}})
	replyTo(t, hub, "run-1", "plan_gate_changes_1", [][]string{{"Add memoization"}})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("PlanGate error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("PlanGate did not return")
	}

	result := <-resultCh

	if approved, _ := result.State["plan_approved"].(bool); approved {
		t.Fatalf("expected approved=false, got %v", result.State)
	}

	if feedback, _ := result.State["plan_feedback"].(string); feedback != "Add memoization" {
		t.Fatalf("expected feedback %q, got %q", "Add memoization", feedback)
	}
}

// replyTo delivers an answer to a registered question, waiting briefly
// for the worker to register it.
func replyTo(
	t *testing.T,
	hub *graph.QuestionHub,
	runID string,
	requestID string,
	answers [][]string,
) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {

		if hub.Reply(runID, requestID, answers) {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("question %s was never registered", requestID)
}
