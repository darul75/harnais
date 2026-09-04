package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS runs (
    id           TEXT PRIMARY KEY,
    workflow_id  TEXT NOT NULL DEFAULT '',
    task         TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'running',
    started_at   DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS node_executions (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    node_id      TEXT NOT NULL,
    worker_id    TEXT NOT NULL,
    attempt      INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL,
    input        TEXT NOT NULL DEFAULT '{}',
    output       TEXT NOT NULL DEFAULT '{}',
    error        TEXT NOT NULL DEFAULT '',
    started_at   DATETIME NOT NULL,
    completed_at DATETIME,
    triggered_by TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS agent_executions (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    node_execution_id TEXT NOT NULL,
    agent_id          TEXT NOT NULL,
    status            TEXT NOT NULL,
    started_at        DATETIME NOT NULL,
    completed_at      DATETIME,
    error             TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS agent_activities (
    id                 TEXT PRIMARY KEY,
    agent_execution_id TEXT NOT NULL,
    run_id             TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence           INTEGER NOT NULL,
    kind               TEXT NOT NULL,
    llm_call_id        TEXT NOT NULL DEFAULT '',
    tool_call_id       TEXT NOT NULL DEFAULT '',
    started_at         DATETIME NOT NULL,
    completed_at       DATETIME,
    status             TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_calls (
    id                 TEXT PRIMARY KEY,
    run_id             TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    agent_execution_id TEXT NOT NULL,
    sequence           INTEGER NOT NULL,
    status             TEXT NOT NULL,
    messages           TEXT NOT NULL DEFAULT '[]',
    reasoning          TEXT NOT NULL DEFAULT '',
    response           TEXT NOT NULL DEFAULT '',
    requested_tool     TEXT NOT NULL DEFAULT '',
    started_at         DATETIME NOT NULL,
    completed_at       DATETIME,
    error              TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tool_calls (
    id                 TEXT PRIMARY KEY,
    run_id             TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    agent_execution_id TEXT NOT NULL,
    sequence           INTEGER NOT NULL,
    tool_id            TEXT NOT NULL,
    status             TEXT NOT NULL,
    input              TEXT NOT NULL DEFAULT '{}',
    output             TEXT NOT NULL DEFAULT '{}',
    started_at         DATETIME NOT NULL,
    completed_at       DATETIME,
    error              TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS edge_activations (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    edge_id           TEXT NOT NULL,
    from_execution_id TEXT NOT NULL DEFAULT '',
    from_node_id      TEXT NOT NULL DEFAULT '',
    to_node_id        TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL,
    consumed_at       DATETIME
);

CREATE INDEX IF NOT EXISTS idx_node_executions_run_id ON node_executions(run_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_run_id ON agent_executions(run_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_node_exec_id ON agent_executions(node_execution_id);
CREATE INDEX IF NOT EXISTS idx_agent_activities_agent_exec_id ON agent_activities(agent_execution_id);
CREATE INDEX IF NOT EXISTS idx_agent_activities_run_id ON agent_activities(run_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_run_id ON llm_calls(run_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_agent_exec_id ON llm_calls(agent_execution_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_run_id ON tool_calls(run_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_agent_exec_id ON tool_calls(agent_execution_id);
CREATE INDEX IF NOT EXISTS idx_edge_activations_run_id ON edge_activations(run_id);
`

func (s *RunStore) initSchema() error {
	_, err := s.db.Exec(schemaSQL)
	return err
}
