package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"harnais/graph"
)

type RunRecord struct {
	RunID      string
	WorkflowID string
	Task       string
	Status     graph.Status
	StartedAt  time.Time
	CompletedAt *time.Time
}

func scanRun(row *sql.Row) (*RunRecord, error) {
	var r RunRecord
	var status string
	var completedAt sql.NullTime

	err := row.Scan(
		&r.RunID,
		&r.WorkflowID,
		&r.Task,
		&status,
		&r.StartedAt,
		&completedAt,
	)
	if err != nil {
		return nil, err
	}

	r.Status = graph.Status(status)
	if completedAt.Valid {
		r.CompletedAt = &completedAt.Time
	}

	return &r, nil
}

func scanRuns(rows *sql.Rows) ([]RunRecord, error) {
	var records []RunRecord
	for rows.Next() {
		var r RunRecord
		var status string
		var completedAt sql.NullTime

		if err := rows.Scan(
			&r.RunID,
			&r.WorkflowID,
			&r.Task,
			&status,
			&r.StartedAt,
			&completedAt,
		); err != nil {
			return nil, err
		}

		r.Status = graph.Status(status)
		if completedAt.Valid {
			r.CompletedAt = &completedAt.Time
		}

		records = append(records, r)
	}
	return records, rows.Err()
}

func scanNodeExecution(row *sql.Row) (*graph.NodeExecution, error) {
	var e graph.NodeExecution
	var triggeredBy string

	err := row.Scan(
		&e.ID,
		&e.NodeID,
		&e.WorkerID,
		&e.Attempt,
		&e.Status,
		&e.Input,
		&e.Output,
		&e.Error,
		&e.StartedAt,
		&e.CompletedAt,
		&triggeredBy,
	)
	if err != nil {
		return nil, err
	}

	if triggeredBy != "" {
		if err := json.Unmarshal([]byte(triggeredBy), &e.TriggeredBy); err != nil {
			e.TriggeredBy = []string{}
		}
	}

	return &e, nil
}

func scanNodeExecutions(rows *sql.Rows) ([]*graph.NodeExecution, error) {
	var executions []*graph.NodeExecution
	for rows.Next() {
		e, err := scanNodeExecRow(rows)
		if err != nil {
			return nil, err
		}
		executions = append(executions, e)
	}
	return executions, rows.Err()
}

func scanNodeExecRow(rows *sql.Rows) (*graph.NodeExecution, error) {
	var e graph.NodeExecution
	var input, output, triggeredBy string

	err := rows.Scan(
		&e.ID,
		&e.NodeID,
		&e.WorkerID,
		&e.Attempt,
		&e.Status,
		&input,
		&output,
		&e.Error,
		&e.StartedAt,
		&e.CompletedAt,
		&triggeredBy,
	)
	if err != nil {
		return nil, err
	}

	if input != "" {
		if err := json.Unmarshal([]byte(input), &e.Input); err != nil {
			e.Input = make(graph.State)
		}
	} else {
		e.Input = make(graph.State)
	}

	if output != "" {
		if err := json.Unmarshal([]byte(output), &e.Output); err != nil {
			e.Output = make(graph.State)
		}
	} else {
		e.Output = make(graph.State)
	}

	if triggeredBy != "" {
		if err := json.Unmarshal([]byte(triggeredBy), &e.TriggeredBy); err != nil {
			e.TriggeredBy = []string{}
		}
	}

	return &e, nil
}

func scanAgentExecution(row *sql.Row) (*graph.AgentExecution, error) {
	var e graph.AgentExecution
	err := row.Scan(
		&e.ID,
		&e.NodeExecutionID,
		&e.AgentID,
		&e.Status,
		&e.StartedAt,
		&e.CompletedAt,
		&e.Error,
	)
	return &e, err
}

func scanAgentExecutions(rows *sql.Rows) ([]*graph.AgentExecution, error) {
	var executions []*graph.AgentExecution
	for rows.Next() {
		var e graph.AgentExecution
		if err := rows.Scan(
			&e.ID,
			&e.NodeExecutionID,
			&e.AgentID,
			&e.Status,
			&e.StartedAt,
			&e.CompletedAt,
			&e.Error,
		); err != nil {
			return nil, err
		}
		executions = append(executions, &e)
	}
	return executions, rows.Err()
}

func scanAgentActivity(row *sql.Row) (*graph.AgentActivity, error) {
	var a graph.AgentActivity
	var llmCallID, toolCallID string
	var completedAt sql.NullTime

	err := row.Scan(
		&a.ID,
		&a.AgentExecutionID,
		&a.Sequence,
		&a.Kind,
		&llmCallID,
		&toolCallID,
		&a.StartedAt,
		&completedAt,
		&a.Status,
	)
	if err != nil {
		return nil, err
	}

	if llmCallID != "" {
		a.LLMCallID = &llmCallID
	}
	if toolCallID != "" {
		a.ToolCallID = &toolCallID
	}
	if completedAt.Valid {
		a.CompletedAt = &completedAt.Time
	}

	return &a, nil
}

func scanAgentActivities(rows *sql.Rows) ([]*graph.AgentActivity, error) {
	var activities []*graph.AgentActivity
	for rows.Next() {
		a, err := scanAgentActivityRow(rows)
		if err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

func scanAgentActivityRow(rows *sql.Rows) (*graph.AgentActivity, error) {
	var a graph.AgentActivity
	var llmCallID, toolCallID string
	var completedAt sql.NullTime

	err := rows.Scan(
		&a.ID,
		&a.AgentExecutionID,
		&a.Sequence,
		&a.Kind,
		&llmCallID,
		&toolCallID,
		&a.StartedAt,
		&completedAt,
		&a.Status,
	)
	if err != nil {
		return nil, err
	}

	if llmCallID != "" {
		a.LLMCallID = &llmCallID
	}
	if toolCallID != "" {
		a.ToolCallID = &toolCallID
	}
	if completedAt.Valid {
		a.CompletedAt = &completedAt.Time
	}

	return &a, nil
}

func scanLLMCall(row *sql.Row) (*graph.LLMCall, error) {
	var c graph.LLMCall
	var messages string

	err := row.Scan(
		&c.ID,
		&c.AgentExecutionID,
		&c.Sequence,
		&c.Status,
		&messages,
		&c.Reasoning,
		&c.Response,
		&c.RequestedTool,
		&c.StartedAt,
		&c.CompletedAt,
		&c.Error,
	)
	if err != nil {
		return nil, err
	}

	if messages != "" {
		if err := json.Unmarshal([]byte(messages), &c.Messages); err != nil {
			c.Messages = []graph.MessageRecord{}
		}
	}

	return &c, nil
}

func scanLLMCalls(rows *sql.Rows) ([]*graph.LLMCall, error) {
	var calls []*graph.LLMCall
	for rows.Next() {
		var c graph.LLMCall
		var messages string

		if err := rows.Scan(
			&c.ID,
			&c.AgentExecutionID,
			&c.Sequence,
			&c.Status,
			&messages,
			&c.Reasoning,
			&c.Response,
			&c.RequestedTool,
			&c.StartedAt,
			&c.CompletedAt,
			&c.Error,
		); err != nil {
			return nil, err
		}

		if messages != "" {
			if err := json.Unmarshal([]byte(messages), &c.Messages); err != nil {
				c.Messages = []graph.MessageRecord{}
			}
		}

		calls = append(calls, &c)
	}
	return calls, rows.Err()
}

func scanToolCall(row *sql.Row) (*graph.ToolCall, error) {
	var c graph.ToolCall
	var input, output string

	err := row.Scan(
		&c.ID,
		&c.AgentExecutionID,
		&c.Sequence,
		&c.ToolID,
		&c.Status,
		&input,
		&output,
		&c.StartedAt,
		&c.CompletedAt,
		&c.Error,
	)
	if err != nil {
		return nil, err
	}

	if input != "" {
		if err := json.Unmarshal([]byte(input), &c.Input); err != nil {
			c.Input = make(map[string]any)
		}
	} else {
		c.Input = make(map[string]any)
	}

	if output != "" {
		if err := json.Unmarshal([]byte(output), &c.Output); err != nil {
			c.Output = make(map[string]any)
		}
	} else {
		c.Output = make(map[string]any)
	}

	return &c, nil
}

func scanToolCalls(rows *sql.Rows) ([]*graph.ToolCall, error) {
	var calls []*graph.ToolCall
	for rows.Next() {
		var c graph.ToolCall
		var input, output string

		if err := rows.Scan(
			&c.ID,
			&c.AgentExecutionID,
			&c.Sequence,
			&c.ToolID,
			&c.Status,
			&input,
			&output,
			&c.StartedAt,
			&c.CompletedAt,
			&c.Error,
		); err != nil {
			return nil, err
		}

		if input != "" {
			if err := json.Unmarshal([]byte(input), &c.Input); err != nil {
				c.Input = make(map[string]any)
			}
		} else {
			c.Input = make(map[string]any)
		}

		if output != "" {
			if err := json.Unmarshal([]byte(output), &c.Output); err != nil {
				c.Output = make(map[string]any)
			}
		} else {
			c.Output = make(map[string]any)
		}

		calls = append(calls, &c)
	}
	return calls, rows.Err()
}

func scanEdgeActivation(row *sql.Row) (*graph.EdgeActivation, error) {
	var a graph.EdgeActivation
	var consumedAt sql.NullTime

	err := row.Scan(
		&a.ID,
		&a.EdgeID,
		&a.FromExecutionID,
		&a.FromNodeID,
		&a.ToNodeID,
		&a.CreatedAt,
		&consumedAt,
	)
	if err != nil {
		return nil, err
	}

	if consumedAt.Valid {
		a.ConsumedAt = &consumedAt.Time
	}

	return &a, nil
}

func scanEdgeActivations(rows *sql.Rows) ([]*graph.EdgeActivation, error) {
	var activations []*graph.EdgeActivation
	for rows.Next() {
		var a graph.EdgeActivation
		var consumedAt sql.NullTime

		if err := rows.Scan(
			&a.ID,
			&a.EdgeID,
			&a.FromExecutionID,
			&a.FromNodeID,
			&a.ToNodeID,
			&a.CreatedAt,
			&consumedAt,
		); err != nil {
			return nil, err
		}

		if consumedAt.Valid {
			a.ConsumedAt = &consumedAt.Time
		}

		activations = append(activations, &a)
	}
	return activations, rows.Err()
}

func scanEvents(rows *sql.Rows) ([]graph.Event, error) {
	var events []graph.Event
	for rows.Next() {
		var e graph.Event
		var dataStr string
		var id int64

		if err := rows.Scan(
			&id,
			&e.RunID,
			&e.Type,
			&e.NodeID,
			&e.ExecutionID,
			&e.WorkerID,
			&e.AgentID,
			&e.ToolID,
			&e.Message,
			&dataStr,
			&e.Time,
		); err != nil {
			return nil, err
		}

		e.ID = uint64(id)
		if dataStr != "" {
			if err := json.Unmarshal([]byte(dataStr), &e.Data); err != nil {
				e.Data = make(map[string]any)
			}
		}

		events = append(events, e)
	}
	return events, rows.Err()
}

func toJSON(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func optionalTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t
}

func jsonMust(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

type RunDetail struct {
	RunRecord
	NodeExecutions   []*graph.NodeExecution
	AgentExecutions  []*graph.AgentExecution
	LLMCalls         []*graph.LLMCall
	ToolCalls        []*graph.ToolCall
	EdgeActivations  []*graph.EdgeActivation
}

func (s *RunStore) GetRunDetail(runID string) (*RunDetail, error) {
	record, err := s.GetRun(runID)
	if err != nil {
		return nil, err
	}

	nodeExecs, err := s.GetNodeExecutions(runID)
	if err != nil {
		return nil, fmt.Errorf("get node executions: %w", err)
	}

	agentExecs, err := s.GetAgentExecutions(runID)
	if err != nil {
		return nil, fmt.Errorf("get agent executions: %w", err)
	}

	llmCalls, err := s.GetLLMCalls(runID)
	if err != nil {
		return nil, fmt.Errorf("get llm calls: %w", err)
	}

	toolCalls, err := s.GetToolCalls(runID)
	if err != nil {
		return nil, fmt.Errorf("get tool calls: %w", err)
	}

	edgeActs, err := s.GetEdgeActivations(runID)
	if err != nil {
		return nil, fmt.Errorf("get edge activations: %w", err)
	}

	activities, err := s.GetAgentActivities(runID)
	if err != nil {
		return nil, fmt.Errorf("get agent activities: %w", err)
	}

	activityByAgent := make(map[string][]*graph.AgentActivity)
	for _, a := range activities {
		activityByAgent[a.AgentExecutionID] = append(activityByAgent[a.AgentExecutionID], a)
	}

	for _, ae := range agentExecs {
		if acts, ok := activityByAgent[ae.ID]; ok {
			ae.Activities = acts
		}
	}

	return &RunDetail{
		RunRecord:        *record,
		NodeExecutions:   nodeExecs,
		AgentExecutions:  agentExecs,
		LLMCalls:         llmCalls,
		ToolCalls:        toolCalls,
		EdgeActivations:  edgeActs,
	}, nil
}

func (s *RunStore) GetLLMCalls(runID string) ([]*graph.LLMCall, error) {
	query := `
		SELECT id, agent_execution_id, sequence, status, messages,
		       reasoning, response, requested_tool, started_at,
		       completed_at, error
		FROM llm_calls
		WHERE run_id = ?
		ORDER BY sequence ASC
	`
	rows, err := s.db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLLMCalls(rows)
}

func (s *RunStore) GetToolCalls(runID string) ([]*graph.ToolCall, error) {
	query := `
		SELECT id, agent_execution_id, sequence, tool_id, status,
		       input, output, started_at, completed_at, error
		FROM tool_calls
		WHERE run_id = ?
		ORDER BY sequence ASC
	`
	rows, err := s.db.Query(query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanToolCalls(rows)
}
