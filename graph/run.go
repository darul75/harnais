package graph

import (
	"sync"
	"time"
)

type Run struct {
	ID string

	Graph *Graph

	Status Status

	StartedAt   time.Time
	CompletedAt *time.Time

	// Every actual node execution.
	Executions []*NodeExecution

	// A node can be activated and then consumed
	// by the scheduler.
	ActivatedNodes map[string]bool

	// An edge is activated when its condition is satisfied.
	ActivatedEdges map[string]bool

	State State

	mu sync.RWMutex
}

func NewRun(
	id string,
	graph *Graph,
) *Run {

	return &Run{
		ID: id,

		Graph: graph,

		Status: StatusRunning,

		StartedAt: time.Now(),

		Executions: make(
			[]*NodeExecution,
			0,
		),

		ActivatedNodes: make(
			map[string]bool,
		),

		ActivatedEdges: make(
			map[string]bool,
		),

		State: make(
			map[string]any,
		),
	}
}

// ------------------------------------------------------------
// Node activation
// ------------------------------------------------------------

func (r *Run) ActivateNode(
	nodeID string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ActivatedNodes[nodeID] = true
}

func (r *Run) IsNodeActivated(
	nodeID string,
) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ActivatedNodes[nodeID]
}

func (r *Run) ConsumeNodeActivation(
	nodeID string,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.ActivatedNodes[nodeID] {
		return false
	}

	delete(
		r.ActivatedNodes,
		nodeID,
	)

	return true
}

// ------------------------------------------------------------
// Edge activation
// ------------------------------------------------------------

func (r *Run) ActivateEdge(
	edgeID string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ActivatedEdges[edgeID] = true
}

func (r *Run) IsEdgeActivated(
	edgeID string,
) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ActivatedEdges[edgeID]
}

// ------------------------------------------------------------
// Execution history
// ------------------------------------------------------------

func (r *Run) AddExecution(
	execution *NodeExecution,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Executions = append(
		r.Executions,
		execution,
	)
}

func (r *Run) NextAttempt(
	nodeID string,
) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	attempt := 0

	for _, execution := range r.Executions {

		if execution.NodeID == nodeID {
			attempt++
		}
	}

	return attempt + 1
}

func (r *Run) SetState(state State) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.State = state.Clone()
}

func (r *Run) StateSnapshot() State {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.State.Clone()
}

// ------------------------------------------------------------
// Snapshot
//
// Server/UI never accesses Run's internal mutex directly.
// ------------------------------------------------------------

type RunSnapshot struct {
	ID string `json:"id"`

	Status Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Executions []*NodeExecution `json:"executions"`

	ActivatedNodes map[string]bool `json:"activatedNodes"`

	ActivatedEdges map[string]bool `json:"activatedEdges"`

	State State `json:"state"`
}

func (r *Run) Snapshot() RunSnapshot {

	r.mu.RLock()
	defer r.mu.RUnlock()

	executions := make(
		[]*NodeExecution,
		len(r.Executions),
	)

	copy(
		executions,
		r.Executions,
	)

	activatedNodes := make(
		map[string]bool,
		len(r.ActivatedNodes),
	)

	for key, value := range r.ActivatedNodes {
		activatedNodes[key] = value
	}

	activatedEdges := make(
		map[string]bool,
		len(r.ActivatedEdges),
	)

	for key, value := range r.ActivatedEdges {
		activatedEdges[key] = value
	}

	return RunSnapshot{
		ID: r.ID,

		Status: r.Status,

		StartedAt: r.StartedAt,

		CompletedAt: r.CompletedAt,

		Executions: executions,

		ActivatedNodes: activatedNodes,

		ActivatedEdges: activatedEdges,
	}
}
