package server

import (
	"sync"

	"harnais/graph"
)

type RunManager struct {
	mu sync.RWMutex

	runs map[string]*graph.Run
}

func NewRunManager() *RunManager {
	return &RunManager{
		runs: make(
			map[string]*graph.Run,
		),
	}
}

func (m *RunManager) Add(
	run *graph.Run,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.runs[run.ID] = run
}

func (m *RunManager) Get(
	runID string,
) (*graph.Run, bool) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	run, ok :=
		m.runs[runID]

	return run, ok
}
