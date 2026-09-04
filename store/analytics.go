package store

import (
	"database/sql"
	"time"
)

type AnalyticsOverview struct {
	TotalRuns       int     `json:"totalRuns"`
	CompletedRuns   int     `json:"completedRuns"`
	FailedRuns      int     `json:"failedRuns"`
	RunningRuns     int     `json:"runningRuns"`
	TotalLLMCalls   int     `json:"totalLLMCalls"`
	CompletedLLM    int     `json:"completedLLM"`
	FailedLLM       int     `json:"failedLLM"`
	TotalToolCalls  int     `json:"totalToolCalls"`
	CompletedTools  int     `json:"completedTools"`
	FailedTools     int     `json:"failedTools"`
	TotalAgents     int     `json:"totalAgents"`
	CompletedAgents int     `json:"completedAgents"`
	TotalEvents     int     `json:"totalEvents"`
	AvgRunDuration  float64 `json:"avgRunDuration"` // seconds
	AvgLLMDuration  float64 `json:"avgLLMDuration"` // seconds
	AvgToolDuration float64 `json:"avgToolDuration"` // seconds
}

type LLMCallByRun struct {
	RunID   string  `json:"runId"`
	Task    string  `json:"task"`
	Count   int     `json:"count"`
	Failed  int     `json:"failed"`
	AvgSec  float64 `json:"avgSec"`
	TotalSec float64 `json:"totalSec"`
}

type ToolCallStats struct {
	ToolID   string  `json:"toolId"`
	Count    int     `json:"count"`
	Completed int    `json:"completed"`
	Failed   int     `json:"failed"`
	AvgSec   float64 `json:"avgSec"`
}

type ToolFailure struct {
	RunID     string    `json:"runId"`
	Task      string    `json:"task"`
	ToolID    string    `json:"toolId"`
	Error     string    `json:"error"`
	StartedAt time.Time `json:"startedAt"`
}

type LLMFailure struct {
	RunID     string    `json:"runId"`
	Task      string    `json:"task"`
	Error     string    `json:"error"`
	StartedAt time.Time `json:"startedAt"`
}

type EdgeActivationStats struct {
	EdgeID    string `json:"edgeId"`
	FromNode  string `json:"fromNode"`
	ToNode    string `json:"toNode"`
	Activated int    `json:"activated"`
	Consumed  int    `json:"consumed"`
}

type RunDuration struct {
	RunID      string    `json:"runId"`
	Task       string    `json:"task"`
	Status     string    `json:"status"`
	DurationSec float64  `json:"durationSec"`
	StartedAt  time.Time `json:"startedAt"`
}

func (s *RunStore) GetAnalyticsOverview() (*AnalyticsOverview, error) {
	var o AnalyticsOverview

	row := s.db.QueryRow(`SELECT COUNT(*) FROM runs`)
	row.Scan(&o.TotalRuns)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'completed'`)
	row.Scan(&o.CompletedRuns)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'failed'`)
	row.Scan(&o.FailedRuns)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'running'`)
	row.Scan(&o.RunningRuns)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM llm_calls`)
	row.Scan(&o.TotalLLMCalls)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE status = 'completed'`)
	row.Scan(&o.CompletedLLM)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE status = 'error' OR status = 'failed'`)
	row.Scan(&o.FailedLLM)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM tool_calls`)
	row.Scan(&o.TotalToolCalls)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM tool_calls WHERE status = 'completed'`)
	row.Scan(&o.CompletedTools)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM tool_calls WHERE status = 'error' OR status = 'failed'`)
	row.Scan(&o.FailedTools)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM agent_executions`)
	row.Scan(&o.TotalAgents)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM agent_executions WHERE status = 'completed'`)
	row.Scan(&o.CompletedAgents)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM events`)
	row.Scan(&o.TotalEvents)

	var avgRun sql.NullFloat64
	row = s.db.QueryRow(`
		SELECT AVG(
			(strftime('%s', completed_at) - strftime('%s', started_at))
		) FROM runs WHERE completed_at IS NOT NULL
	`)
	row.Scan(&avgRun)
	if avgRun.Valid {
		o.AvgRunDuration = avgRun.Float64
	}

	var avgLLM sql.NullFloat64
	row = s.db.QueryRow(`
		SELECT AVG(
			(strftime('%s', completed_at) - strftime('%s', started_at))
		) FROM llm_calls WHERE completed_at IS NOT NULL
	`)
	row.Scan(&avgLLM)
	if avgLLM.Valid {
		o.AvgLLMDuration = avgLLM.Float64
	}

	var avgTool sql.NullFloat64
	row = s.db.QueryRow(`
		SELECT AVG(
			(strftime('%s', completed_at) - strftime('%s', started_at))
		) FROM tool_calls WHERE completed_at IS NOT NULL
	`)
	row.Scan(&avgTool)
	if avgTool.Valid {
		o.AvgToolDuration = avgTool.Float64
	}

	return &o, nil
}

func (s *RunStore) GetLLMCallsByRun() ([]LLMCallByRun, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.task, COUNT(*) as cnt,
			SUM(CASE WHEN l.status IN ('error', 'failed') THEN 1 ELSE 0 END) as failed,
			COALESCE(AVG(CASE WHEN l.completed_at IS NOT NULL
				THEN (strftime('%s', l.completed_at) - strftime('%s', l.started_at))
				ELSE NULL END), 0) as avg_sec,
			COALESCE(SUM(CASE WHEN l.completed_at IS NOT NULL
				THEN (strftime('%s', l.completed_at) - strftime('%s', l.started_at))
				ELSE 0 END), 0) as total_sec
		FROM llm_calls l JOIN runs r ON l.run_id = r.id
		GROUP BY l.run_id
		ORDER BY total_sec DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LLMCallByRun
	for rows.Next() {
		var r LLMCallByRun
		if err := rows.Scan(&r.RunID, &r.Task, &r.Count, &r.Failed, &r.AvgSec, &r.TotalSec); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *RunStore) GetToolCallStats() ([]ToolCallStats, error) {
	rows, err := s.db.Query(`
		SELECT tool_id, COUNT(*) as cnt,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status IN ('error', 'failed') THEN 1 ELSE 0 END) as failed,
			COALESCE(AVG(CASE WHEN completed_at IS NOT NULL
				THEN (strftime('%s', completed_at) - strftime('%s', started_at))
				ELSE NULL END), 0) as avg_sec
		FROM tool_calls
		GROUP BY tool_id
		ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ToolCallStats
	for rows.Next() {
		var s ToolCallStats
		if err := rows.Scan(&s.ToolID, &s.Count, &s.Completed, &s.Failed, &s.AvgSec); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (s *RunStore) GetToolFailures() ([]ToolFailure, error) {
	rows, err := s.db.Query(`
		SELECT t.run_id, r.task, t.tool_id, t.error, t.started_at
		FROM tool_calls t JOIN runs r ON t.run_id = r.id
		WHERE t.status IN ('error', 'failed') AND t.error != ''
		ORDER BY t.started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ToolFailure
	for rows.Next() {
		var f ToolFailure
		if err := rows.Scan(&f.RunID, &f.Task, &f.ToolID, &f.Error, &f.StartedAt); err != nil {
			return nil, err
		}
		results = append(results, f)
	}
	return results, rows.Err()
}

func (s *RunStore) GetLLMFailures() ([]LLMFailure, error) {
	rows, err := s.db.Query(`
		SELECT l.run_id, r.task, l.error, l.started_at
		FROM llm_calls l JOIN runs r ON l.run_id = r.id
		WHERE l.status IN ('error', 'failed') AND l.error != ''
		ORDER BY l.started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LLMFailure
	for rows.Next() {
		var f LLMFailure
		if err := rows.Scan(&f.RunID, &f.Task, &f.Error, &f.StartedAt); err != nil {
			return nil, err
		}
		results = append(results, f)
	}
	return results, rows.Err()
}

func (s *RunStore) GetEdgeActivationStats() ([]EdgeActivationStats, error) {
	rows, err := s.db.Query(`
		SELECT edge_id, from_node_id, to_node_id,
			COUNT(*) as activated,
			SUM(CASE WHEN consumed_at IS NOT NULL THEN 1 ELSE 0 END) as consumed
		FROM edge_activations
		GROUP BY edge_id, from_node_id, to_node_id
		ORDER BY activated DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []EdgeActivationStats
	for rows.Next() {
		var e EdgeActivationStats
		if err := rows.Scan(&e.EdgeID, &e.FromNode, &e.ToNode, &e.Activated, &e.Consumed); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (s *RunStore) GetRunDurations() ([]RunDuration, error) {
	rows, err := s.db.Query(`
		SELECT id, task, status,
			COALESCE(CASE WHEN completed_at IS NOT NULL
				THEN (strftime('%s', completed_at) - strftime('%s', started_at))
				ELSE 0 END, 0) as duration_sec,
			started_at
		FROM runs
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RunDuration
	for rows.Next() {
		var r RunDuration
		if err := rows.Scan(&r.RunID, &r.Task, &r.Status, &r.DurationSec, &r.StartedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
