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
// Worker
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
// Node
// ------------------------------------------------------------

type Node struct {
	ID string

	Worker Worker

	// If true, all incoming runtime activations must be
	// available before this node executes.
	JoinAll bool
}

// ------------------------------------------------------------
// Edge
// ------------------------------------------------------------

type Edge struct {
	ID string

	From string
	To   string

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

	TriggeredBy []string `json:"triggeredBy"`
}

// ------------------------------------------------------------
// Runtime edge activation
// ------------------------------------------------------------

type EdgeActivation struct {
	ID string `json:"id"`

	EdgeID string `json:"edgeId"`

	FromExecutionID string `json:"fromExecutionId"`

	FromNodeID string `json:"fromNodeId"`

	ToNodeID string `json:"toNodeId"`

	CreatedAt time.Time `json:"createdAt"`

	ToExecutionID *string `json:"toExecutionId,omitempty"`

	ConsumedAt *time.Time `json:"consumedAt,omitempty"`
}

// ------------------------------------------------------------
// Agent execution
// ------------------------------------------------------------

type AgentExecution struct {
	ID string `json:"id"`

	NodeExecutionID string `json:"nodeExecutionId"`

	AgentID string `json:"agentId"`

	Status Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Error string `json:"error,omitempty"`
}

// ------------------------------------------------------------
// LLM call
// ------------------------------------------------------------

type MessageRecord struct {
	Role string `json:"role"`

	Content string `json:"content"`
}

type LLMCall struct {
	ID string `json:"id"`

	AgentExecutionID string `json:"agentExecutionId"`

	Sequence int `json:"sequence"`

	Status Status `json:"status"`

	Messages []MessageRecord `json:"messages,omitempty"`

	Response string `json:"response,omitempty"`

	RequestedTool string `json:"requestedTool,omitempty"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Error string `json:"error,omitempty"`
}

// ------------------------------------------------------------
// Tool call
// ------------------------------------------------------------

type ToolCall struct {
	ID string `json:"id"`

	AgentExecutionID string `json:"agentExecutionId"`

	Sequence int `json:"sequence"`

	ToolID string `json:"toolId"`

	Status Status `json:"status"`

	Input map[string]any `json:"input,omitempty"`

	Output map[string]any `json:"output,omitempty"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Error string `json:"error,omitempty"`
}

// ------------------------------------------------------------
// Event context
// ------------------------------------------------------------

type EventSink func(Event)

type executionContextKey string

const executionContextKeyName executionContextKey = "graph-execution"

type ExecutionContext struct {
	RunID string

	ExecutionID string

	NodeID string

	WorkerID string

	Run *Run

	EventSink EventSink
}

func WithExecutionContext(
	ctx context.Context,
	execution ExecutionContext,
) context.Context {
	return context.WithValue(
		ctx,
		executionContextKeyName,
		execution,
	)
}

func GetExecutionContext(
	ctx context.Context,
) (ExecutionContext, bool) {
	value := ctx.Value(
		executionContextKeyName,
	)

	if value == nil {
		return ExecutionContext{}, false
	}

	execution, ok :=
		value.(ExecutionContext)

	return execution, ok
}

func EmitEvent(
	ctx context.Context,
	event Event,
) {
	execution, ok :=
		GetExecutionContext(ctx)

	if !ok {
		return
	}

	event.RunID =
		execution.RunID

	event.NodeID =
		execution.NodeID

	event.ExecutionID =
		execution.ExecutionID

	event.WorkerID =
		execution.WorkerID

	if execution.EventSink != nil {
		execution.EventSink(event)
	}
}
