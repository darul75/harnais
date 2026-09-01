package server

import (
	"sort"
	"sync"

	"harnais/graph"
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

	runs map[string]*RunRecord
}

func NewRunManager() *RunManager {
	return &RunManager{
		runs: make(
			map[string]*RunRecord,
		),
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
