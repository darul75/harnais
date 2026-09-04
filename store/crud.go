package store

import (
	"fmt"
	"time"

	"harnais/graph"
)

func (s *RunStore) CreateRun(id, workflowID, task string, startedAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO runs (id, workflow_id, task, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		id, workflowID, task, formatTime(startedAt),
	)
	return err
}

func (s *RunStore) UpdateRunStatus(id string, status graph.Status, completedAt *time.Time) error {
	_, err := s.db.Exec(
		`UPDATE runs SET status = ?, completed_at = ? WHERE id = ?`,
		status, optionalTime(completedAt), id,
	)
	return err
}

func (s *RunStore) GetRun(id string) (*RunRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, workflow_id, task, status, started_at, completed_at FROM runs WHERE id = ?`,
		id,
	)
	return scanRun(row)
}

func (s *RunStore) ListRuns() ([]RunRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, workflow_id, task, status, started_at, completed_at FROM runs ORDER BY started_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRuns(rows)
}

func (s *RunStore) DeleteRun(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete child records explicitly (SQLite FK cascade requires PRAGMA).
	tables := []string{
		"events",
		"edge_activations",
		"tool_calls",
		"llm_calls",
		"agent_activities",
		"agent_executions",
		"node_executions",
	}
	for _, t := range tables {
		if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE run_id = ?", t), id); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM runs WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) AddNodeExecution(runID string, e *graph.NodeExecution) error {
	_, err := s.db.Exec(
		`INSERT INTO node_executions (
			id, run_id, node_id, worker_id, attempt, status,
			input, output, error, started_at, completed_at, triggered_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, runID, 		e.NodeID, e.WorkerID, e.Attempt, e.Status,
		toJSON(e.Input),
		toJSON(e.Output),
		e.Error,
		formatTime(e.StartedAt),
		optionalTime(e.CompletedAt),
		jsonMust(e.TriggeredBy),
	)
	return err
}

func (s *RunStore) UpdateNodeExecution(e *graph.NodeExecution) error {
	_, err := s.db.Exec(
		`UPDATE node_executions SET
			status = ?, output = ?, error = ?, completed_at = ?
		WHERE id = ?`,
		e.Status,
		toJSON(e.Output),
		e.Error,
		optionalTime(e.CompletedAt),
		e.ID,
	)
	return err
}

func (s *RunStore) GetNodeExecutions(runID string) ([]*graph.NodeExecution, error) {
	rows, err := s.db.Query(
		`SELECT id, node_id, worker_id, attempt, status,
			input, output, error, started_at, completed_at, triggered_by
		FROM node_executions WHERE run_id = ? ORDER BY started_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNodeExecutions(rows)
}

func (s *RunStore) AddAgentExecution(runID string, e *graph.AgentExecution) error {
	_, err := s.db.Exec(
		`INSERT INTO agent_executions (
			id, run_id, node_execution_id, agent_id, status,
			started_at, completed_at, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, runID, e.NodeExecutionID, e.AgentID, e.Status,
		formatTime(e.StartedAt),
		optionalTime(e.CompletedAt),
		e.Error,
	)
	return err
}

func (s *RunStore) UpdateAgentExecution(e *graph.AgentExecution) error {
	_, err := s.db.Exec(
		`UPDATE agent_executions SET status = ?, completed_at = ?, error = ? WHERE id = ?`,
		e.Status,
		optionalTime(e.CompletedAt),
		e.Error,
		e.ID,
	)
	return err
}

func (s *RunStore) GetAgentExecutions(runID string) ([]*graph.AgentExecution, error) {
	rows, err := s.db.Query(
		`SELECT id, node_execution_id, agent_id, status,
			started_at, completed_at, error
		FROM agent_executions WHERE run_id = ? ORDER BY started_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAgentExecutions(rows)
}

func (s *RunStore) AddAgentActivity(runID string, a *graph.AgentActivity) error {
	llmCallID := ""
	if a.LLMCallID != nil {
		llmCallID = *a.LLMCallID
	}
	toolCallID := ""
	if a.ToolCallID != nil {
		toolCallID = *a.ToolCallID
	}

	_, err := s.db.Exec(
		`INSERT INTO agent_activities (
			id, agent_execution_id, run_id, sequence, kind,
			llm_call_id, tool_call_id, started_at, completed_at, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.AgentExecutionID, runID, a.Sequence, a.Kind,
		llmCallID, toolCallID, formatTime(a.StartedAt), optionalTime(a.CompletedAt), a.Status,
	)
	return err
}

func (s *RunStore) GetAgentActivities(runID string) ([]*graph.AgentActivity, error) {
	rows, err := s.db.Query(
		`SELECT id, agent_execution_id, sequence, kind,
			llm_call_id, tool_call_id, started_at, completed_at, status
		FROM agent_activities WHERE run_id = ? ORDER BY sequence ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAgentActivities(rows)
}

func (s *RunStore) AddLLMCall(runID string, c *graph.LLMCall) error {
	_, err := s.db.Exec(
		`INSERT INTO llm_calls (
			id, run_id, agent_execution_id, sequence, status, messages,
			reasoning, response, requested_tool, started_at, completed_at, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, runID, c.AgentExecutionID, c.Sequence, c.Status,
		jsonMust(c.Messages),
		c.Reasoning,
		c.Response,
		c.RequestedTool,
		formatTime(c.StartedAt),
		optionalTime(c.CompletedAt),
		c.Error,
	)
	return err
}

func (s *RunStore) UpdateLLMCall(c *graph.LLMCall) error {
	_, err := s.db.Exec(
		`UPDATE llm_calls SET status = ?, messages = ?, reasoning = ?, response = ?,
			requested_tool = ?, completed_at = ?, error = ?
		WHERE id = ?`,
		c.Status,
		jsonMust(c.Messages),
		c.Reasoning,
		c.Response,
		c.RequestedTool,
		optionalTime(c.CompletedAt),
		c.Error,
		c.ID,
	)
	return err
}

func (s *RunStore) AddToolCall(runID string, c *graph.ToolCall) error {
	_, err := s.db.Exec(
		`INSERT INTO tool_calls (
			id, run_id, agent_execution_id, sequence, tool_id, status,
			input, output, started_at, completed_at, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, runID, c.AgentExecutionID, c.Sequence, c.ToolID, c.Status,
		toJSON(c.Input),
		toJSON(c.Output),
		formatTime(c.StartedAt),
		optionalTime(c.CompletedAt),
		c.Error,
	)
	return err
}

func (s *RunStore) UpdateToolCall(c *graph.ToolCall) error {
	_, err := s.db.Exec(
		`UPDATE tool_calls SET status = ?, output = ?, completed_at = ?, error = ? WHERE id = ?`,
		c.Status,
		toJSON(c.Output),
		optionalTime(c.CompletedAt),
		c.Error,
		c.ID,
	)
	return err
}

func (s *RunStore) AddEdgeActivation(runID string, a *graph.EdgeActivation) error {
	_, err := s.db.Exec(
		`INSERT INTO edge_activations (
			id, run_id, edge_id, from_execution_id, from_node_id,
			to_node_id, created_at, consumed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, runID, a.EdgeID, a.FromExecutionID, a.FromNodeID,
		a.ToNodeID,
		formatTime(a.CreatedAt),
		optionalTime(a.ConsumedAt),
	)
	return err
}

func (s *RunStore) GetEdgeActivations(runID string) ([]*graph.EdgeActivation, error) {
	rows, err := s.db.Query(
		`SELECT id, edge_id, from_execution_id, from_node_id,
			to_node_id, created_at, consumed_at
		FROM edge_activations WHERE run_id = ? ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEdgeActivations(rows)
}

func (s *RunStore) ReconstructRun(runID string) (*graph.Run, error) {
	detail, err := s.GetRunDetail(runID)
	if err != nil {
		return nil, fmt.Errorf("get run detail: %w", err)
	}

	r := &graph.Run{
		ID: detail.RunID,
		Status: detail.Status,
		StartedAt: detail.StartedAt,
		CompletedAt: detail.CompletedAt,
		Executions: detail.NodeExecutions,
		EdgeActivations: detail.EdgeActivations,
		AgentExecutions: detail.AgentExecutions,
		LLMCalls: detail.LLMCalls,
		ToolCalls: detail.ToolCalls,
		ActivatedNodes: make(map[string]bool),
		State: make(graph.State),
	}

	for _, e := range detail.NodeExecutions {
		r.ActivatedNodes[e.NodeID] = true
		if e.Output != nil {
			r.State.Merge(e.Output)
		}
	}

	return r, nil
}

func (s *RunStore) AddEvent(e *graph.Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events (
			run_id, event_type, node_id, execution_id, worker_id,
			agent_id, tool_id, message, data, time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.RunID, e.Type, e.NodeID, e.ExecutionID, e.WorkerID,
		e.AgentID, e.ToolID, e.Message, jsonMust(e.Data), formatTime(e.Time),
	)
	return err
}

func (s *RunStore) GetEvents(runID string) ([]graph.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, event_type, node_id, execution_id,
			worker_id, agent_id, tool_id, message, data, time
		FROM events WHERE run_id = ? ORDER BY id ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

