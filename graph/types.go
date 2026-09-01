package graph

import (
	"context"
	"time"
)

type State map[string]any

func (s State) Clone() State {
	result := make(State, len(s))

	for key, value := range s {
		result[key] = value
	}

	return result
}

func (s State) Merge(other State) {
	for key, value := range other {
		s[key] = value
	}
}

// ------------------------------------------------------------
// Worker abstraction
// ------------------------------------------------------------

type WorkerInput struct {
	State State
}

type WorkerResult struct {
	State State
}

type Worker interface {
	ID() string

	Run(
		ctx context.Context,
		input WorkerInput,
	) (WorkerResult, error)
}

// ------------------------------------------------------------
// Graph node
// ------------------------------------------------------------

type Node struct {
	ID string

	Worker Worker
}

// ------------------------------------------------------------
// Edge
// ------------------------------------------------------------

type Edge struct {
	ID string

	From string
	To   string

	// nil = unconditional
	Condition func(State) bool
}

// ------------------------------------------------------------
// Status
// ------------------------------------------------------------

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// ------------------------------------------------------------
// Node execution
// ------------------------------------------------------------

type NodeExecution struct {
	ID string `json:"id"`

	NodeID string `json:"nodeId"`

	WorkerID string `json:"workerId"`

	Attempt int `json:"attempt"`

	Status Status `json:"status"`

	Input State `json:"input"`

	Output State `json:"output"`

	Error string `json:"error,omitempty"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`
}
