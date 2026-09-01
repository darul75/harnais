package graph

import (
	"fmt"
	"sync"
	"time"
)

type Run struct {
	ID string

	Graph *Graph

	Status Status

	StartedAt   time.Time
	CompletedAt *time.Time

	Executions []*NodeExecution

	EdgeActivations []*EdgeActivation

	AgentExecutions []*AgentExecution

	LLMCalls []*LLMCall

	ToolCalls []*ToolCall

	ActivatedNodes map[string]bool

	State State

	mu sync.RWMutex
}

func NewRun(
	id string,
	graph *Graph,
	initial State,
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

		EdgeActivations: make(
			[]*EdgeActivation,
			0,
		),

		AgentExecutions: make(
			[]*AgentExecution,
			0,
		),

		LLMCalls: make(
			[]*LLMCall,
			0,
		),

		ToolCalls: make(
			[]*ToolCall,
			0,
		),

		ActivatedNodes: make(
			map[string]bool,
		),

		State: initial.Clone(),
	}
}

// ------------------------------------------------------------
// IDs
// ------------------------------------------------------------

func runtimeID(prefix string) string {
	return fmt.Sprintf(
		"%s-%d",
		prefix,
		time.Now().UnixNano(),
	)
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
// Node executions
// ------------------------------------------------------------

func (r *Run) AddExecution(
	execution *NodeExecution,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Executions =
		append(
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

func (r *Run) HasRunningExecution(
	nodeID string,
) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, execution := range r.Executions {

		if execution.NodeID == nodeID &&
			execution.Status == StatusRunning {

			return true
		}
	}

	return false
}

// ------------------------------------------------------------
// Edge activations
// ------------------------------------------------------------

func (r *Run) AddEdgeActivation(
	activation *EdgeActivation,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.EdgeActivations =
		append(
			r.EdgeActivations,
			activation,
		)
}

func (r *Run) PendingActivationsForNode(
	nodeID string,
) []*EdgeActivation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*EdgeActivation

	for _, activation := range r.EdgeActivations {

		if activation.ToNodeID != nodeID {
			continue
		}

		if activation.ToExecutionID != nil {
			continue
		}

		result =
			append(
				result,
				activation,
			)
	}

	return result
}

func (r *Run) ConsumeEdgeActivation(
	activationID string,
	executionID string,
) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, activation := range r.EdgeActivations {

		if activation.ID != activationID {
			continue
		}

		if activation.ToExecutionID != nil {
			return false
		}

		now := time.Now()

		activation.ToExecutionID =
			&executionID

		activation.ConsumedAt =
			&now

		return true
	}

	return false
}

// ------------------------------------------------------------
// State
// ------------------------------------------------------------

func (r *Run) SetState(
	state State,
) {
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
// Agent execution
// ------------------------------------------------------------

func (r *Run) StartAgentExecution(
	nodeExecutionID string,
	agentID string,
) *AgentExecution {

	execution := &AgentExecution{
		ID: runtimeID("agent"),

		NodeExecutionID: nodeExecutionID,

		AgentID: agentID,

		Status: StatusRunning,

		StartedAt: time.Now(),

		Activities: make(
			[]*AgentActivity,
			0,
		),
	}

	r.mu.Lock()

	r.AgentExecutions =
		append(
			r.AgentExecutions,
			execution,
		)

	r.mu.Unlock()

	return execution
}

func (r *Run) CompleteAgentExecution(
	agentExecutionID string,
	err error,
) {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, execution := range r.AgentExecutions {

		if execution.ID !=
			agentExecutionID {
			continue
		}

		execution.CompletedAt =
			&now

		if err != nil {

			execution.Status =
				StatusFailed

			execution.Error =
				err.Error()

		} else {

			execution.Status =
				StatusCompleted
		}

		return
	}
}

// ------------------------------------------------------------
// Agent activities
// ------------------------------------------------------------

func (r *Run) StartAgentActivity(
	agentExecutionID string,
	sequence int,
	kind AgentActivityKind,
) *AgentActivity {

	activity := &AgentActivity{
		ID: runtimeID("activity"),

		AgentExecutionID: agentExecutionID,

		Sequence: sequence,

		Kind: kind,

		Status: StatusRunning,

		StartedAt: time.Now(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.attachActivityLocked(
		activity,
	)

	return activity
}

func (r *Run) attachActivityLocked(
	activity *AgentActivity,
) {
	for _, execution := range r.AgentExecutions {

		if execution.ID !=
			activity.AgentExecutionID {
			continue
		}

		execution.Activities =
			append(
				execution.Activities,
				activity,
			)

		return
	}
}

func (r *Run) CompleteAgentActivity(
	activityID string,
	err error,
) {

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, execution := range r.AgentExecutions {

		for _, activity := range execution.Activities {

			if activity.ID !=
				activityID {
				continue
			}

			activity.CompletedAt =
				&now

			if err != nil {
				activity.Status =
					StatusFailed
			} else {
				activity.Status =
					StatusCompleted
			}

			return
		}
	}
}

// ------------------------------------------------------------
// LLM calls
// ------------------------------------------------------------

func (r *Run) StartLLMCall(
	agentExecutionID string,
	activityID string,
	sequence int,
	messages []MessageRecord,
) *LLMCall {

	call := &LLMCall{
		ID: runtimeID("llm"),

		AgentExecutionID: agentExecutionID,

		ActivityID: activityID,

		Sequence: sequence,

		Status: StatusRunning,

		Messages: append(
			[]MessageRecord(nil),
			messages...,
		),

		StartedAt: time.Now(),
	}

	r.mu.Lock()

	r.LLMCalls =
		append(
			r.LLMCalls,
			call,
		)

	r.mu.Unlock()

	return call
}

func (r *Run) CompleteLLMCall(
	llmCallID string,
	response string,
	requestedTool string,
	err error,
) {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, call := range r.LLMCalls {

		if call.ID != llmCallID {
			continue
		}

		call.CompletedAt =
			&now

		call.Response =
			response

		call.RequestedTool =
			requestedTool

		if err != nil {
			call.Status =
				StatusFailed

			call.Error =
				err.Error()
		} else {
			call.Status =
				StatusCompleted
		}

		return
	}
}

// ------------------------------------------------------------
// Tool calls
// ------------------------------------------------------------

func (r *Run) StartToolCall(
	agentExecutionID string,
	activityID string,
	sequence int,
	toolID string,
	input map[string]any,
) *ToolCall {

	call := &ToolCall{
		ID: runtimeID("tool"),

		AgentExecutionID: agentExecutionID,

		ActivityID: activityID,

		Sequence: sequence,

		ToolID: toolID,

		Status: StatusRunning,

		Input: input,

		StartedAt: time.Now(),
	}

	r.mu.Lock()

	r.ToolCalls =
		append(
			r.ToolCalls,
			call,
		)

	r.mu.Unlock()

	return call
}

func (r *Run) CompleteToolCall(
	toolCallID string,
	output map[string]any,
	err error,
) {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, call := range r.ToolCalls {

		if call.ID != toolCallID {
			continue
		}

		call.CompletedAt =
			&now

		call.Output =
			output

		if err != nil {
			call.Status =
				StatusFailed

			call.Error =
				err.Error()
		} else {
			call.Status =
				StatusCompleted
		}

		return
	}
}

// ------------------------------------------------------------
// Snapshot
// ------------------------------------------------------------

type RunSnapshot struct {
	ID string `json:"id"`

	Status Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Executions []*NodeExecution `json:"executions"`

	EdgeActivations []*EdgeActivation `json:"edgeActivations"`

	AgentExecutions []*AgentExecution `json:"agentExecutions"`

	LLMCalls []*LLMCall `json:"llmCalls"`

	ToolCalls []*ToolCall `json:"toolCalls"`

	ActivatedNodes map[string]bool `json:"activatedNodes"`

	State State `json:"state"`
}

func (r *Run) Snapshot() RunSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	executions :=
		make(
			[]*NodeExecution,
			len(r.Executions),
		)

	copy(
		executions,
		r.Executions,
	)

	edgeActivations :=
		make(
			[]*EdgeActivation,
			len(r.EdgeActivations),
		)

	copy(
		edgeActivations,
		r.EdgeActivations,
	)

	agentExecutions :=
		make(
			[]*AgentExecution,
			len(r.AgentExecutions),
		)

	copy(
		agentExecutions,
		r.AgentExecutions,
	)

	llmCalls :=
		make(
			[]*LLMCall,
			len(r.LLMCalls),
		)

	copy(
		llmCalls,
		r.LLMCalls,
	)

	toolCalls :=
		make(
			[]*ToolCall,
			len(r.ToolCalls),
		)

	copy(
		toolCalls,
		r.ToolCalls,
	)

	activatedNodes :=
		make(
			map[string]bool,
			len(r.ActivatedNodes),
		)

	for key, value := range r.ActivatedNodes {

		activatedNodes[key] =
			value
	}

	return RunSnapshot{
		ID: r.ID,

		Status: r.Status,

		StartedAt: r.StartedAt,

		CompletedAt: r.CompletedAt,

		Executions: executions,

		EdgeActivations: edgeActivations,

		AgentExecutions: agentExecutions,

		LLMCalls: llmCalls,

		ToolCalls: toolCalls,

		ActivatedNodes: activatedNodes,

		State: r.State.Clone(),
	}
}
