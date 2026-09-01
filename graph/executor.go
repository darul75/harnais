package graph

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type EventType string

const (
	EventRunStarted   EventType = "run.started"
	EventRunCompleted EventType = "run.completed"
	EventRunFailed    EventType = "run.failed"

	EventNodeStarted   EventType = "node.started"
	EventNodeCompleted EventType = "node.completed"
	EventNodeFailed    EventType = "node.failed"

	EventEdgeActivated EventType = "edge.activated"

	EventWorkerStarted   EventType = "worker.started"
	EventWorkerCompleted EventType = "worker.completed"
	EventWorkerFailed    EventType = "worker.failed"

	EventAgentStarted   EventType = "agent.started"
	EventAgentCompleted EventType = "agent.completed"

	EventLLMStarted   EventType = "llm.started"
	EventLLMCompleted EventType = "llm.completed"

	EventToolStarted   EventType = "tool.started"
	EventToolCompleted EventType = "tool.completed"
	EventToolFailed    EventType = "tool.failed"
)

type Event struct {
	ID uint64 `json:"id"`

	Time time.Time `json:"time"`

	RunID string `json:"runID"`

	Type EventType `json:"type"`

	NodeID string `json:"nodeID,omitempty"`

	ExecutionID string `json:"executionID,omitempty"`

	WorkerID string `json:"workerID,omitempty"`

	AgentID string `json:"agentID,omitempty"`

	ToolID string `json:"toolID,omitempty"`

	ActivationID string `json:"activationID,omitempty"`

	Message string `json:"message,omitempty"`

	Data map[string]any `json:"data,omitempty"`
}

type EventHandler func(Event)

type Executor struct {
	OnEvent EventHandler

	OnRun func(*Run)
}

func NewExecutor(
	handler EventHandler,
	onRun func(*Run),
) *Executor {
	return &Executor{
		OnEvent: handler,
		OnRun:   onRun,
	}
}

func (e *Executor) emit(event Event) {
	if e.OnEvent != nil {
		e.OnEvent(event)
	}
}

// ------------------------------------------------------------
// Start asynchronously
// ------------------------------------------------------------

func (e *Executor) Start(
	ctx context.Context,
	g *Graph,
	initial State,
) *Run {
	runID := fmt.Sprintf(
		"run-%d",
		time.Now().UnixNano(),
	)

	run := NewRun(
		runID,
		g,
		initial,
	)

	if e.OnRun != nil {
		e.OnRun(run)
	}

	go func() {
		_, _, _ = e.run(
			ctx,
			run,
			initial,
		)
	}()

	return run
}

// ------------------------------------------------------------
// Synchronous Run
// ------------------------------------------------------------

func (e *Executor) Run(
	ctx context.Context,
	g *Graph,
	initial State,
) (State, *Run, error) {
	runID := fmt.Sprintf(
		"run-%d",
		time.Now().UnixNano(),
	)

	run := NewRun(
		runID,
		g,
		initial,
	)

	if e.OnRun != nil {
		e.OnRun(run)
	}

	return e.run(
		ctx,
		run,
		initial,
	)
}

// ------------------------------------------------------------
// Internal execution
// ------------------------------------------------------------

func (e *Executor) run(
	ctx context.Context,
	run *Run,
	initial State,
) (State, *Run, error) {
	state := initial.Clone()

	run.SetState(state)

	e.emit(Event{
		Time:  time.Now(),
		RunID: run.ID,
		Type:  EventRunStarted,
	})

	// --------------------------------------------------
	// Activate root nodes.
	// --------------------------------------------------

	for nodeID := range run.Graph.Nodes {
		if len(run.Graph.Incoming(nodeID)) == 0 {
			run.ActivateNode(nodeID)
		}
	}

	// --------------------------------------------------
	// Scheduler loop.
	// --------------------------------------------------

	for {
		select {
		case <-ctx.Done():
			e.failRun(run, ctx.Err())
			return state, run, ctx.Err()

		default:
		}

		ready := e.findReadyNodes(run)

		if len(ready) == 0 {
			if e.isFinished(run) {
				now := time.Now()

				run.mu.Lock()
				run.Status = StatusCompleted
				run.CompletedAt = &now
				run.mu.Unlock()

				e.emit(Event{
					Time:  now,
					RunID: run.ID,
					Type:  EventRunCompleted,
				})

				return state, run, nil
			}

			err := fmt.Errorf("execution is stuck")

			e.failRun(run, err)

			return state, run, err
		}

		results, err := e.executeWave(
			ctx,
			run,
			ready,
			state,
		)

		if err != nil {
			e.failRun(run, err)
			return state, run, err
		}

		// --------------------------------------------------
		// Merge completed wave results.
		// --------------------------------------------------

		for _, execution := range results {
			state.Merge(execution.Output)

			run.SetState(state)

			e.activateOutgoingEdges(
				run,
				execution,
				state,
			)
		}
	}
}

// ------------------------------------------------------------
// Find ready nodes
// ------------------------------------------------------------

func (e *Executor) findReadyNodes(
	run *Run,
) []*Node {
	var ready []*Node

	for nodeID, node := range run.Graph.Nodes {
		if !run.IsNodeActivated(nodeID) {
			continue
		}

		if run.HasRunningExecution(nodeID) {
			continue
		}

		activations :=
			run.PendingActivationsForNode(nodeID)

		// --------------------------------------------------
		// Root node.
		// --------------------------------------------------

		if len(run.Graph.Incoming(nodeID)) == 0 {
			if !run.ConsumeNodeActivation(nodeID) {
				continue
			}

			ready = append(
				ready,
				node,
			)

			continue
		}

		// --------------------------------------------------
		// Non-root node.
		// --------------------------------------------------

		if len(activations) == 0 {
			continue
		}

		// JoinAll means every incoming graph edge
		// must have produced a runtime activation.
		if node.JoinAll {
			if !e.hasActivationForEveryIncomingEdge(
				run,
				nodeID,
				activations,
			) {
				continue
			}
		}

		if !run.ConsumeNodeActivation(nodeID) {
			continue
		}

		ready = append(
			ready,
			node,
		)
	}

	return ready
}

func (e *Executor) hasActivationForEveryIncomingEdge(
	run *Run,
	nodeID string,
	activations []*EdgeActivation,
) bool {
	incoming := run.Graph.Incoming(nodeID)

	for _, edge := range incoming {
		found := false

		for _, activation := range activations {
			if activation.EdgeID == edge.ID {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// ------------------------------------------------------------
// Activate outgoing edges
// ------------------------------------------------------------

func (e *Executor) activateOutgoingEdges(
	run *Run,
	execution *NodeExecution,
	state State,
) {
	for _, edge := range run.Graph.Outgoing(
		execution.NodeID,
	) {
		activate := true

		if edge.Condition != nil {
			activate = edge.Condition(state)
		}

		if !activate {
			continue
		}

		activation := &EdgeActivation{
			ID: fmt.Sprintf(
				"activation-%d",
				time.Now().UnixNano(),
			),

			EdgeID: edge.ID,

			FromExecutionID: execution.ID,

			FromNodeID: edge.From,

			ToNodeID: edge.To,

			CreatedAt: time.Now(),
		}

		run.AddEdgeActivation(
			activation,
		)

		run.ActivateNode(
			edge.To,
		)

		e.emit(Event{
			Time: time.Now(),

			RunID: run.ID,

			Type: EventEdgeActivated,

			NodeID: edge.To,

			ExecutionID: execution.ID,

			ActivationID: activation.ID,

			Message: fmt.Sprintf(
				"%s -> %s",
				edge.From,
				edge.To,
			),

			Data: map[string]any{
				"from": edge.From,

				"to": edge.To,

				"fromExecutionId": execution.ID,

				"activationId": activation.ID,
			},
		})
	}
}

// ------------------------------------------------------------
// Execute a wave in parallel
// ------------------------------------------------------------

func (e *Executor) executeWave(
	ctx context.Context,
	run *Run,
	nodes []*Node,
	state State,
) ([]*NodeExecution, error) {
	var wg sync.WaitGroup

	var resultsMu sync.Mutex

	results := make(
		[]*NodeExecution,
		0,
		len(nodes),
	)

	errs := make(
		chan error,
		len(nodes),
	)

	for _, node := range nodes {
		node := node

		wg.Add(1)

		go func() {
			defer wg.Done()

			activations :=
				run.PendingActivationsForNode(
					node.ID,
				)

			execution := &NodeExecution{
				ID: fmt.Sprintf(
					"%s-%d",
					node.ID,
					time.Now().UnixNano(),
				),

				NodeID: node.ID,

				WorkerID: node.Worker.ID(),

				Attempt: run.NextAttempt(
					node.ID,
				),

				Status: StatusRunning,

				Input: state.Clone(),

				StartedAt: time.Now(),

				TriggeredBy: make(
					[]string,
					0,
					len(activations),
				),
			}

			// --------------------------------------------------
			// Attach triggering activations.
			// --------------------------------------------------

			for _, activation := range activations {

				if node.JoinAll {
					if !run.ConsumeEdgeActivation(
						activation.ID,
						execution.ID,
					) {
						continue
					}

					execution.TriggeredBy =
						append(
							execution.TriggeredBy,
							activation.ID,
						)

					continue
				}

				// Normal nodes consume exactly one
				// activation for this execution.

				if len(
					execution.TriggeredBy,
				) > 0 {
					break
				}

				if !run.ConsumeEdgeActivation(
					activation.ID,
					execution.ID,
				) {
					continue
				}

				execution.TriggeredBy =
					append(
						execution.TriggeredBy,
						activation.ID,
					)
			}

			run.AddExecution(
				execution,
			)

			// --------------------------------------------------
			// Context passed to everything below the node.
			// --------------------------------------------------

			execCtx :=
				WithExecutionContext(
					ctx,
					ExecutionContext{
						RunID: run.ID,

						ExecutionID: execution.ID,

						NodeID: node.ID,

						WorkerID: node.Worker.ID(),

						Run: run,

						EventSink: e.emit,
					},
				)

			// --------------------------------------------------
			// Node started.
			// --------------------------------------------------

			e.emit(Event{
				Time: time.Now(),

				RunID: run.ID,

				Type: EventNodeStarted,

				NodeID: node.ID,

				ExecutionID: execution.ID,

				WorkerID: node.Worker.ID(),

				Data: map[string]any{
					"attempt": execution.Attempt,

					"triggeredBy": execution.TriggeredBy,
				},
			})

			// --------------------------------------------------
			// Worker started.
			// --------------------------------------------------

			e.emit(Event{
				Time: time.Now(),

				RunID: run.ID,

				Type: EventWorkerStarted,

				NodeID: node.ID,

				ExecutionID: execution.ID,

				WorkerID: node.Worker.ID(),
			})

			// --------------------------------------------------
			// Execute worker.
			// --------------------------------------------------

			result, err :=
				node.Worker.Run(
					execCtx,
					WorkerInput{
						State: execution.Input.Clone(),
					},
				)

			if err != nil {
				execution.Status =
					StatusFailed

				execution.Error =
					err.Error()

				now :=
					time.Now()

				execution.CompletedAt =
					&now

				e.emit(Event{
					Time: now,

					RunID: run.ID,

					Type: EventWorkerFailed,

					NodeID: node.ID,

					ExecutionID: execution.ID,

					WorkerID: node.Worker.ID(),

					Message: err.Error(),
				})

				e.emit(Event{
					Time: now,

					RunID: run.ID,

					Type: EventNodeFailed,

					NodeID: node.ID,

					ExecutionID: execution.ID,

					WorkerID: node.Worker.ID(),

					Message: err.Error(),
				})

				errs <- fmt.Errorf(
					"node %s: %w",
					node.ID,
					err,
				)

				return
			}

			// --------------------------------------------------
			// Worker completed.
			// --------------------------------------------------

			execution.Output =
				result.State

			execution.Status =
				StatusCompleted

			now :=
				time.Now()

			execution.CompletedAt =
				&now

			e.emit(Event{
				Time: now,

				RunID: run.ID,

				Type: EventWorkerCompleted,

				NodeID: node.ID,

				ExecutionID: execution.ID,

				WorkerID: node.Worker.ID(),
			})

			e.emit(Event{
				Time: now,

				RunID: run.ID,

				Type: EventNodeCompleted,

				NodeID: node.ID,

				ExecutionID: execution.ID,

				WorkerID: node.Worker.ID(),

				Data: map[string]any{
					"attempt": execution.Attempt,
				},
			})

			resultsMu.Lock()

			results =
				append(
					results,
					execution,
				)

			resultsMu.Unlock()

		}()
	}

	wg.Wait()

	select {
	case err := <-errs:
		return nil, err

	default:
		return results, nil
	}
}

// ------------------------------------------------------------
// Finished
// ------------------------------------------------------------

func (e *Executor) isFinished(
	run *Run,
) bool {
	run.mu.RLock()
	defer run.mu.RUnlock()

	for _, execution := range run.Executions {

		if execution.Status ==
			StatusRunning {

			return false
		}
	}

	if len(run.ActivatedNodes) > 0 {
		return false
	}

	for _, activation := range run.EdgeActivations {

		if activation.ToExecutionID == nil {
			return false
		}
	}

	return true
}

// ------------------------------------------------------------
// Fail run
// ------------------------------------------------------------

func (e *Executor) failRun(
	run *Run,
	err error,
) {
	now :=
		time.Now()

	run.mu.Lock()

	run.Status =
		StatusFailed

	run.CompletedAt =
		&now

	run.mu.Unlock()

	e.emit(Event{
		Time: now,

		RunID: run.ID,

		Type: EventRunFailed,

		Message: err.Error(),
	})
}
