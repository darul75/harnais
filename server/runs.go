package server

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"harnais/graph"
	"harnais/store"
)

type RunMeta struct {
	Task string

	WorkflowID string
}

type RunRecord struct {
	Run *graph.Run

	Meta RunMeta
}

type RunManager struct {
	mu sync.RWMutex

	runs  map[string]*RunRecord
	store *store.RunStore
}

func NewRunManager(db *store.RunStore) *RunManager {
	return &RunManager{
		runs:  make(map[string]*RunRecord),
		store: db,
	}
}

func (m *RunManager) Add(
	run *graph.Run,
	meta RunMeta,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.runs[run.ID] = &RunRecord{
		Run: run,

		Meta: meta,
	}
}

func (m *RunManager) CreateRun(id string, meta RunMeta, startedAt time.Time) {
	if m.store == nil {
		return
	}
	m.store.CreateRun(id, meta.WorkflowID, meta.Task, startedAt)
}

func (m *RunManager) Get(
	runID string,
) (*graph.Run, bool) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok :=
		m.runs[runID]

	if !ok {
		return nil, false
	}

	return record.Run, true
}

func (m *RunManager) Meta(
	runID string,
) (RunMeta, bool) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok :=
		m.runs[runID]

	if !ok {
		return RunMeta{}, false
	}

	return record.Meta, true
}

func (m *RunManager) List() []RunRecord {

	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make(
		[]RunRecord,
		0,
		len(m.runs),
	)

	for _, record := range m.runs {

		records = append(
			records,
			*record,
		)
	}

	sort.Slice(
		records,
		func(i, j int) bool {

			return records[i].Run.StartedAt.After(
				records[j].Run.StartedAt,
			)
		},
	)

	return records
}

func (m *RunManager) UpdateStatus(runID string, status graph.Status, completedAt *time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.runs[runID]
	if !ok {
		return
	}

	record.Run.Status = status
	record.Run.CompletedAt = completedAt

	if m.store != nil {
		m.store.UpdateRunStatus(runID, status, completedAt)
	}
}

func (m *RunManager) DeleteRun(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.runs, runID)

	if m.store != nil {
		return m.store.DeleteRun(runID)
	}
	return nil
}

func (m *RunManager) PersistRunSnapshot(runID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.runs[runID]
	if !ok {
		return nil
	}

	if m.store == nil {
		return nil
	}

	snapshot := record.Run.Snapshot()

	for _, e := range snapshot.Executions {
		if err := m.store.AddNodeExecution(runID, e); err != nil {
			return err
		}
	}

	for _, e := range snapshot.AgentExecutions {
		if err := m.store.AddAgentExecution(runID, e); err != nil {
			return err
		}
		for _, a := range e.Activities {
			if err := m.store.AddAgentActivity(runID, a); err != nil {
				return err
			}
		}
	}

	for _, c := range snapshot.LLMCalls {
		if err := m.store.AddLLMCall(runID, c); err != nil {
			return err
		}
	}

	for _, c := range snapshot.ToolCalls {
		if err := m.store.AddToolCall(runID, c); err != nil {
			return err
		}
	}

	for _, a := range snapshot.EdgeActivations {
		if err := m.store.AddEdgeActivation(runID, a); err != nil {
			return err
		}
	}

	return nil
}

func (m *RunManager) Store() *store.RunStore {
	return m.store
}

func (m *RunManager) ListFromStore() ([]store.RunRecord, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.ListRuns()
}

func (m *RunManager) GetReconstructed(runID string) (*graph.Run, error) {
	if m.store == nil {
		return nil, fmt.Errorf("no store")
	}
	return m.store.ReconstructRun(runID)
}

func (m *RunManager) GetNodeExecutions(runID string) ([]*graph.NodeExecution, error) {
	if m.store == nil {
		return nil, fmt.Errorf("no store")
	}
	return m.store.GetNodeExecutions(runID)
}

func (m *RunManager) GetAgentExecutions(runID string) ([]*graph.AgentExecution, error) {
	if m.store == nil {
		return nil, fmt.Errorf("no store")
	}
	return m.store.GetAgentExecutions(runID)
}
