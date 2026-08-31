package graph

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type EventType string

const (
	EventRunStarted EventType = "run.started"

	EventNodeStarted   EventType = "node.started"
	EventNodeCompleted EventType = "node.completed"
	EventNodeFailed    EventType = "node.failed"

	EventEdgeActivated EventType = "edge.activated"

	EventRunCompleted EventType = "run.completed"
	EventRunFailed    EventType = "run.failed"
)

type Event struct {
	ID uint64 `json:"id"`

	Time time.Time `json:"time"`

	RunID string `json:"runID"`

	Type EventType `json:"type"`

	NodeID string `json:"nodeID,omitempty"`

	ExecutionID string `json:"executionID,omitempty"`

	Message string `json:"message,omitempty"`

	Data map[string]any `json:"data,omitempty"`
}

type EventHandler func(Event)

type Executor struct {
	OnEvent EventHandler

	// Called immediately after the Run is created.
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

func (e *Executor) emit(
	event Event,
) {
	if e.OnEvent != nil {
		e.OnEvent(event)
	}
}

// ------------------------------------------------------------
// Run
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
	// Activate roots.
	// --------------------------------------------------

	for nodeID := range run.Graph.Nodes {

		if len(
			run.Graph.Incoming(nodeID),
		) == 0 {

			run.ActivateNode(
				nodeID,
			)
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

		ready :=
			e.findReadyNodes(
				run.Graph,
				run,
			)

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

			err := fmt.Errorf(
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

			run.SetState(
				state,
			)

			e.emit(Event{
				Time:        time.Now(),
				RunID:       run.ID,
				Type:        EventNodeCompleted,
				NodeID:      execution.NodeID,
				ExecutionID: execution.ID,

				Data: map[string]any{
					"attempt": execution.Attempt,
				},
			})

			e.activateOutgoingEdges(
				run.Graph,
				run,
				execution.NodeID,
				state,
			)
		}
	}
}

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
	)

	if e.OnRun != nil {
		e.OnRun(run)
	}

	go func() {

		_, _, err :=
			e.run(
				ctx,
				run,
				initial,
			)

		if err != nil {
			return
		}
	}()

	return run
}

// ------------------------------------------------------------
// Find nodes ready to execute.
// ------------------------------------------------------------

func (e *Executor) findReadyNodes(
	g *Graph,
	run *Run,
) []*Node {

	var ready []*Node

	for nodeID, node := range g.Nodes {

		if !run.IsNodeActivated(nodeID) {
			continue
		}

		if e.isNodeRunning(
			run,
			nodeID,
		) {
			continue
		}

		if !e.dependenciesCompleted(
			g,
			run,
			nodeID,
		) {
			continue
		}

		// Consume the activation.
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

// ------------------------------------------------------------
// Dependency handling.
// ------------------------------------------------------------

func (e *Executor) dependenciesCompleted(
	g *Graph,
	run *Run,
	nodeID string,
) bool {

	for _, edge := range g.Incoming(nodeID) {

		// Ignore edges which were never selected.
		if !run.IsEdgeActivated(edge.ID) {
			continue
		}

		if !e.hasCompletedExecution(
			run,
			edge.From,
		) {
			return false
		}
	}

	return true
}

func (e *Executor) hasCompletedExecution(
	run *Run,
	nodeID string,
) bool {

	run.mu.RLock()
	defer run.mu.RUnlock()

	for _, execution := range run.Executions {

		if execution.NodeID == nodeID &&
			execution.Status == StatusCompleted {

			return true
		}
	}

	return false
}

func (e *Executor) isNodeRunning(
	run *Run,
	nodeID string,
) bool {

	run.mu.RLock()
	defer run.mu.RUnlock()

	for _, execution := range run.Executions {

		if execution.NodeID == nodeID &&
			execution.Status == StatusRunning {

			return true
		}
	}

	return false
}

// ------------------------------------------------------------
// Evaluate outgoing edges.
// ------------------------------------------------------------

func (e *Executor) activateOutgoingEdges(
	g *Graph,
	run *Run,
	nodeID string,
	state State,
) {

	for _, edge := range g.Outgoing(nodeID) {

		activate := true

		if edge.Condition != nil {
			activate = edge.Condition(state)
		}

		if !activate {
			continue
		}

		run.ActivateEdge(
			edge.ID,
		)

		run.ActivateNode(
			edge.To,
		)

		e.emit(Event{
			Time:   time.Now(),
			RunID:  run.ID,
			Type:   EventEdgeActivated,
			NodeID: edge.To,

			Message: fmt.Sprintf(
				"%s -> %s",
				edge.From,
				edge.To,
			),

			Data: map[string]any{
				"from": edge.From,
				"to":   edge.To,
			},
		})
	}
}

// ------------------------------------------------------------
// Execute one wave concurrently.
// ------------------------------------------------------------

func (e *Executor) executeWave(
	ctx context.Context,
	run *Run,
	nodes []*Node,
	state State,
) ([]*NodeExecution, error) {

	var wg sync.WaitGroup

	var mu sync.Mutex

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

			execution := &NodeExecution{
				ID: fmt.Sprintf(
					"%s-%d",
					node.ID,
					time.Now().UnixNano(),
				),

				NodeID: node.ID,

				Attempt: run.NextAttempt(
					node.ID,
				),

				Status: StatusRunning,

				Input: state.Clone(),

				StartedAt: time.Now(),
			}

			run.AddExecution(
				execution,
			)

			e.emit(Event{
				Time:        time.Now(),
				RunID:       run.ID,
				Type:        EventNodeStarted,
				NodeID:      node.ID,
				ExecutionID: execution.ID,

				Data: map[string]any{
					"attempt": execution.Attempt,
				},
			})

			output, err := node.Execute(
				ctx,
				execution.Input.Clone(),
			)

			if err != nil {

				execution.Status = StatusFailed
				execution.Error = err.Error()

				now := time.Now()

				execution.CompletedAt = &now

				e.emit(Event{
					Time:        now,
					RunID:       run.ID,
					Type:        EventNodeFailed,
					NodeID:      node.ID,
					ExecutionID: execution.ID,
					Message:     err.Error(),

					Data: map[string]any{
						"attempt": execution.Attempt,
					},
				})

				errs <- fmt.Errorf(
					"node %s: %w",
					node.ID,
					err,
				)

				return
			}

			execution.Output = output
			execution.Status = StatusCompleted

			now := time.Now()

			execution.CompletedAt = &now

			mu.Lock()

			results = append(
				results,
				execution,
			)

			mu.Unlock()

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
// Finished?
// ------------------------------------------------------------

func (e *Executor) isFinished(
	run *Run,
) bool {

	run.mu.RLock()
	defer run.mu.RUnlock()

	// Something is still active/running.
	for _, execution := range run.Executions {

		if execution.Status == StatusRunning {
			return false
		}
	}

	// Something has been activated but not executed.
	if len(run.ActivatedNodes) > 0 {
		return false
	}

	return true
}

// ------------------------------------------------------------
// Fail the run.
// ------------------------------------------------------------

func (e *Executor) failRun(
	run *Run,
	err error,
) {

	now := time.Now()

	run.mu.Lock()

	run.Status = StatusFailed
	run.CompletedAt = &now

	run.mu.Unlock()

	e.emit(Event{
		Time:    now,
		RunID:   run.ID,
		Type:    EventRunFailed,
		Message: err.Error(),
	})
}
