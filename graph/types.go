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
// Node
// ------------------------------------------------------------

type Node struct {
	ID string

	Execute func(
		ctx context.Context,
		state State,
	) (State, error)
}

// ------------------------------------------------------------
// Edge
// ------------------------------------------------------------

type Edge struct {
	ID string

	From string
	To   string

	// nil = unconditional edge
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
//
// A node may execute multiple times:
//
// coder attempt #1
// coder attempt #2
// coder attempt #3
// ------------------------------------------------------------

type NodeExecution struct {
	ID string `json:"id"`

	NodeID string `json:"nodeId"`

	Attempt int `json:"attempt"`

	Status Status `json:"status"`

	Input State `json:"input"`

	Output State `json:"output"`

	Error string `json:"error,omitempty"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`
}
