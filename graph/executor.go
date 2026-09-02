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
// Start
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

	// Make the run context available to workers so they can write
	// run-scoped artifacts (e.g. reports/<runID>/).
	state["run_id"] =
		run.ID

	state["reports_dir"] =
		fmt.Sprintf(
			"reports/%s",
			run.ID,
		)

	run.SetState(state)

	e.emit(Event{
		Time:  time.Now(),
		RunID: run.ID,
		Type:  EventRunStarted,
	})

	// --------------------------------------------------
	// Activate roots.
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

			e.failRun(
				run,
				ctx.Err(),
			)

			return state, run, ctx.Err()

		default:
		}

		ready := e.findReadyNodes(run)

		if len(ready) == 0 {

			if e.isFinished(run) {

				now := time.Now()

				run.mu.Lock()

				run.Status =
					StatusCompleted

				run.CompletedAt =
					&now

				run.mu.Unlock()

				e.emit(Event{
					Time:  now,
					RunID: run.ID,
					Type:  EventRunCompleted,
				})

				return state, run, nil
			}

			err :=
				fmt.Errorf(
					"execution is stuck",
				)

			e.failRun(
				run,
				err,
			)

			return state, run, err
		}

		results, err :=
			e.executeWave(
				ctx,
				run,
				ready,
				state,
			)

		if err != nil {

			e.failRun(
				run,
				err,
			)

			return state, run, err
		}

		for _, execution := range results {

			state.Merge(
				execution.Output,
			)

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

		// --------------------------------------------------
		// Root node.
		// --------------------------------------------------

		if len(
			run.Graph.Incoming(nodeID),
		) == 0 {

			if !run.ConsumeNodeActivation(
				nodeID,
			) {
				continue
			}

			ready = append(
				ready,
				node,
			)

			continue
		}

		// --------------------------------------------------
		// Runtime activations.
		// --------------------------------------------------

		activations :=
			run.PendingActivationsForNode(
				nodeID,
			)

		if len(activations) == 0 {
			continue
		}

		// --------------------------------------------------
		// JoinAll.
		//
		// We require at least one activation from
		// every incoming graph edge.
		// --------------------------------------------------

		if node.JoinAll {

			if !hasActivationForEveryIncomingEdge(
				run,
				nodeID,
				activations,
			) {
				continue
			}
		}

		if !run.ConsumeNodeActivation(
			nodeID,
		) {
			continue
		}

		ready = append(
			ready,
			node,
		)
	}

	return ready
}

// ------------------------------------------------------------
// Verify JoinAll
// ------------------------------------------------------------

func hasActivationForEveryIncomingEdge(
	run *Run,
	nodeID string,
	activations []*EdgeActivation,
) bool {

	incoming :=
		run.Graph.Incoming(nodeID)

	for _, edge := range incoming {

		found := false

		for _, activation := range activations {

			if activation.EdgeID ==
				edge.ID {

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
// Select runtime activations for a node execution
// ------------------------------------------------------------

func selectTriggeringActivations(
	run *Run,
	node *Node,
) []*EdgeActivation {

	activations :=
		run.PendingActivationsForNode(
			node.ID,
		)

	if len(activations) == 0 {
		return nil
	}

	// --------------------------------------------------
	// JoinAll:
	//
	// Select exactly ONE activation per incoming
	// graph edge.
	//
	// Important:
	// If three activations exist for the same incoming
	// edge, we do NOT consume all three.
	// --------------------------------------------------

	if node.JoinAll {

		incoming :=
			run.Graph.Incoming(node.ID)

		result :=
			make(
				[]*EdgeActivation,
				0,
				len(incoming),
			)

		for _, edge := range incoming {

			for _, activation := range activations {

				if activation.EdgeID ==
					edge.ID {

					result =
						append(
							result,
							activation,
						)

					break
				}
			}
		}

		return result
	}

	// --------------------------------------------------
	// Normal node:
	//
	// One runtime activation = one execution.
	// --------------------------------------------------

	return []*EdgeActivation{
		activations[0],
	}
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
			activate =
				edge.Condition(state)
		}

		if !activate {
			continue
		}

		activation :=
			&EdgeActivation{
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
// Execute wave
// ------------------------------------------------------------

func (e *Executor) executeWave(
	ctx context.Context,
	run *Run,
	nodes []*Node,
	state State,
) ([]*NodeExecution, error) {

	var wg sync.WaitGroup

	var resultsMu sync.Mutex

	results :=
		make(
			[]*NodeExecution,
			0,
			len(nodes),
		)

	errs :=
		make(
			chan error,
			len(nodes),
		)

	for _, node := range nodes {

		wg.Add(1)

		go func() {

			defer wg.Done()

			// --------------------------------------------------
			// Determine exact runtime parents.
			// --------------------------------------------------

			triggeringActivations :=
				selectTriggeringActivations(
					run,
					node,
				)

			execution :=
				&NodeExecution{
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
						len(
							triggeringActivations,
						),
					),
				}

			// --------------------------------------------------
			// Consume exactly the activations selected above.
			// --------------------------------------------------

			for _, activation := range triggeringActivations {

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

			// --------------------------------------------------
			// If there are still pending activations for this
			// node, make sure another execution will happen.
			// --------------------------------------------------

			if len(
				run.PendingActivationsForNode(
					node.ID,
				),
			) > 0 {

				run.ActivateNode(
					node.ID,
				)
			}

			run.AddExecution(
				execution,
			)

			// --------------------------------------------------
			// Execution context.
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
			// Worker.
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
			// Completed.
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
// Failure
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
