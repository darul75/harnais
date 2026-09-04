package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"harnais/graph"
)

func newTestStore(t *testing.T) *RunStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewRunStore: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func TestCreateAndGetRun(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	if err := store.CreateRun("run-1", "pdf-summary", "Summarize this PDF", now); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	record, err := store.GetRun("run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	if record.RunID != "run-1" {
		t.Errorf("expected run-1, got %s", record.RunID)
	}
	if record.WorkflowID != "pdf-summary" {
		t.Errorf("expected pdf-summary, got %s", record.WorkflowID)
	}
	if record.Status != graph.StatusRunning {
		t.Errorf("expected running, got %s", record.Status)
	}
}

func TestUpdateRunStatus(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	if err := store.CreateRun("run-1", "", "test", now); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	completedAt := time.Now()
	if err := store.UpdateRunStatus("run-1", graph.StatusCompleted, &completedAt); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}

	record, err := store.GetRun("run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	if record.Status != graph.StatusCompleted {
		t.Errorf("expected completed, got %s", record.Status)
	}
	if record.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestListRuns(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "task 1", now.Add(-2*time.Hour))
	store.CreateRun("run-2", "wf-2", "task 2", now.Add(-1*time.Hour))
	store.CreateRun("run-3", "wf-1", "task 3", now)

	records, err := store.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(records))
	}

	if records[0].RunID != "run-3" {
		t.Errorf("expected first run to be run-3 (most recent), got %s", records[0].RunID)
	}
}

func TestAddNodeExecution(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	exec := &graph.NodeExecution{
		ID:        "exec-1",
		NodeID:    "node-reviewer",
		WorkerID:  "worker-opencode",
		Attempt:   1,
		Status:    graph.StatusRunning,
		Input:     graph.State{"key": "value"},
		Output:    graph.State{},
		StartedAt: now,
		TriggeredBy: []string{"start"},
	}

	if err := store.AddNodeExecution("run-1", exec); err != nil {
		t.Fatalf("AddNodeExecution: %v", err)
	}

	execs, err := store.GetNodeExecutions("run-1")
	if err != nil {
		t.Fatalf("GetNodeExecutions: %v", err)
	}

	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}

	if execs[0].NodeID != "node-reviewer" {
		t.Errorf("expected node-reviewer, got %s", execs[0].NodeID)
	}
}

func TestAddAgentExecution(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	agent := &graph.AgentExecution{
		ID:              "agent-1",
		NodeExecutionID: "exec-1",
		AgentID:         "agent-reviewer",
		Status:          graph.StatusRunning,
		StartedAt:       now,
	}

	if err := store.AddAgentExecution("run-1", agent); err != nil {
		t.Fatalf("AddAgentExecution: %v", err)
	}

	agents, err := store.GetAgentExecutions("run-1")
	if err != nil {
		t.Fatalf("GetAgentExecutions: %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent execution, got %d", len(agents))
	}
}

func TestAddLLMCall(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	llm := &graph.LLMCall{
		ID:                "llm-1",
		AgentExecutionID:  "agent-1",
		Sequence:          0,
		Status:            graph.StatusRunning,
		Messages:          []graph.MessageRecord{{Role: "user", Content: "hello"}},
		Reasoning:         "thinking...",
		Response:          "hi there",
		StartedAt:         now,
	}

	if err := store.AddLLMCall("run-1", llm); err != nil {
		t.Fatalf("AddLLMCall: %v", err)
	}

	calls, err := store.GetLLMCalls("run-1")
	if err != nil {
		t.Fatalf("GetLLMCalls: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(calls))
	}

	if calls[0].Reasoning != "thinking..." {
		t.Errorf("expected reasoning, got %s", calls[0].Reasoning)
	}
}

func TestAddToolCall(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	tool := &graph.ToolCall{
		ID:               "tool-1",
		AgentExecutionID: "agent-1",
		Sequence:         0,
		ToolID:           "bash",
		Status:           graph.StatusRunning,
		Input:            map[string]any{"command": "echo hello"},
		StartedAt:        now,
	}

	if err := store.AddToolCall("run-1", tool); err != nil {
		t.Fatalf("AddToolCall: %v", err)
	}

	calls, err := store.GetToolCalls("run-1")
	if err != nil {
		t.Fatalf("GetToolCalls: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	if calls[0].ToolID != "bash" {
		t.Errorf("expected bash, got %s", calls[0].ToolID)
	}
}

func TestAddEdgeActivation(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	edge := &graph.EdgeActivation{
		ID:              "edge-1",
		EdgeID:          "edge-reviewer-to-writer",
		FromExecutionID: "exec-1",
		FromNodeID:      "node-reviewer",
		ToNodeID:        "node-writer",
		CreatedAt:       now,
	}

	if err := store.AddEdgeActivation("run-1", edge); err != nil {
		t.Fatalf("AddEdgeActivation: %v", err)
	}

	edges, err := store.GetEdgeActivations("run-1")
	if err != nil {
		t.Fatalf("GetEdgeActivations: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge activation, got %d", len(edges))
	}
}

func TestReconstructRun(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	exec := &graph.NodeExecution{
		ID:        "exec-1",
		NodeID:    "node-reviewer",
		WorkerID:  "worker-opencode",
		Attempt:   1,
		Status:    graph.StatusCompleted,
		Output:    graph.State{"review": "good"},
		StartedAt: now,
		CompletedAt: &now,
	}
	store.AddNodeExecution("run-1", exec)

	run, err := store.ReconstructRun("run-1")
	if err != nil {
		t.Fatalf("ReconstructRun: %v", err)
	}

	if run.ID != "run-1" {
		t.Errorf("expected run-1, got %s", run.ID)
	}
	if len(run.Executions) != 1 {
		t.Errorf("expected 1 execution, got %d", len(run.Executions))
	}
	if run.State["review"] != "good" {
		t.Errorf("expected state review=good, got %v", run.State)
	}
}

func TestDeleteRun(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	store.CreateRun("run-1", "wf-1", "test", now)

	if err := store.DeleteRun("run-1"); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}

	_, err := store.GetRun("run-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStorePersistsAcrossOpenClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	now := time.Now()

	func() {
		store, err := NewRunStore(dbPath)
		if err != nil {
			t.Fatalf("NewRunStore: %v", err)
		}
		defer store.Close()

		store.CreateRun("run-1", "wf-1", "persist test", now)
	}()

	func() {
		store, err := NewRunStore(dbPath)
		if err != nil {
			t.Fatalf("NewRunStore (reopen): %v", err)
		}
		defer store.Close()

		record, err := store.GetRun("run-1")
		if err != nil {
			t.Fatalf("GetRun after reopen: %v", err)
		}

		if record.Task != "persist test" {
			t.Errorf("expected 'persist test', got %s", record.Task)
		}
	}()
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
