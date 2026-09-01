package agent

import (
	"context"
	"fmt"
	"time"

	"harnais/graph"
)

type LLMResponse struct {
	Text string

	ToolCall *ToolCall
}

type ToolCall struct {
	Name string

	Input map[string]any
}

type Message struct {
	Role string

	Content string
}

type LLM interface {
	Generate(
		ctx context.Context,
		messages []Message,
	) (LLMResponse, error)
}

// ------------------------------------------------------------
// LoopAgent
// ------------------------------------------------------------

type LoopAgent struct {
	AgentID string

	Prompt string

	LLM LLM

	Tools map[string]Tool

	Emit func(graph.Event)
}

func (a *LoopAgent) ID() string {
	return a.AgentID
}

// ------------------------------------------------------------
// Worker interface
// ------------------------------------------------------------

func (a *LoopAgent) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	result, err := a.runAgent(
		ctx,
		Input{
			Message: a.Prompt,
			State:   input.State,
		},
	)

	if err != nil {
		return graph.WorkerResult{}, err
	}

	output := graph.State{
		"agent_output": result.Output,
	}

	output.Merge(result.State)

	return graph.WorkerResult{
		State: output,
	}, nil
}

// ------------------------------------------------------------
// Agent loop
// ------------------------------------------------------------

func (a *LoopAgent) runAgent(
	ctx context.Context,
	input Input,
) (Result, error) {

	execution, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return Result{}, fmt.Errorf(
			"agent %q: missing execution context",
			a.AgentID,
		)
	}

	a.emit(graph.Event{
		Time:        time.Now(),
		RunID:       execution.RunID,
		Type:        graph.EventAgentStarted,
		NodeID:      execution.NodeID,
		ExecutionID: execution.ExecutionID,
		AgentID:     a.AgentID,
		Data: map[string]any{
			"message": input.Message,
		},
	})

	messages := []Message{
		{
			Role:    "user",
			Content: input.Message,
		},
	}

	for {
		// --------------------------------------------------
		// Ask LLM
		// --------------------------------------------------

		a.emit(graph.Event{
			Time:        time.Now(),
			RunID:       execution.RunID,
			Type:        graph.EventLLMStarted,
			NodeID:      execution.NodeID,
			ExecutionID: execution.ExecutionID,
			AgentID:     a.AgentID,
		})

		response, err :=
			a.LLM.Generate(
				ctx,
				messages,
			)

		if err != nil {
			return Result{}, err
		}

		a.emit(graph.Event{
			Time:        time.Now(),
			RunID:       execution.RunID,
			Type:        graph.EventLLMCompleted,
			NodeID:      execution.NodeID,
			ExecutionID: execution.ExecutionID,
			AgentID:     a.AgentID,
			Data: map[string]any{
				"hasToolCall": response.ToolCall != nil,
			},
		})

		// --------------------------------------------------
		// No tool call = agent finished
		// --------------------------------------------------

		if response.ToolCall == nil {

			a.emit(graph.Event{
				Time:        time.Now(),
				RunID:       execution.RunID,
				Type:        graph.EventAgentCompleted,
				NodeID:      execution.NodeID,
				ExecutionID: execution.ExecutionID,
				AgentID:     a.AgentID,
				Data: map[string]any{
					"output": response.Text,
				},
			})

			return Result{
				Output: response.Text,
			}, nil
		}

		// --------------------------------------------------
		// Tool call
		// --------------------------------------------------

		call := response.ToolCall

		tool, ok :=
			a.Tools[call.Name]

		if !ok {
			return Result{}, fmt.Errorf(
				"agent %q requested unknown tool %q",
				a.AgentID,
				call.Name,
			)
		}

		a.emit(graph.Event{
			Time:        time.Now(),
			RunID:       execution.RunID,
			Type:        graph.EventToolStarted,
			NodeID:      execution.NodeID,
			ExecutionID: execution.ExecutionID,
			AgentID:     a.AgentID,
			ToolID:      call.Name,
			Data: map[string]any{
				"input": call.Input,
			},
		})

		toolResult, err :=
			tool.Execute(
				ctx,
				call.Input,
			)

		if err != nil {

			a.emit(graph.Event{
				Time:        time.Now(),
				RunID:       execution.RunID,
				Type:        graph.EventToolFailed,
				NodeID:      execution.NodeID,
				ExecutionID: execution.ExecutionID,
				AgentID:     a.AgentID,
				ToolID:      call.Name,
				Message:     err.Error(),
			})

			return Result{}, err
		}

		a.emit(graph.Event{
			Time:        time.Now(),
			RunID:       execution.RunID,
			Type:        graph.EventToolCompleted,
			NodeID:      execution.NodeID,
			ExecutionID: execution.ExecutionID,
			AgentID:     a.AgentID,
			ToolID:      call.Name,
			Data: map[string]any{
				"output": toolResult,
			},
		})

		// --------------------------------------------------
		// Feed tool result back to LLM
		// --------------------------------------------------

		messages = append(
			messages,
			Message{
				Role: "assistant",
				Content: fmt.Sprintf(
					"calling tool %s",
					call.Name,
				),
			},
			Message{
				Role: "tool",
				Content: fmt.Sprintf(
					"%v",
					toolResult,
				),
			},
		)
	}
}

// ------------------------------------------------------------
// Event helper
// ------------------------------------------------------------

func (a *LoopAgent) emit(
	event graph.Event,
) {
	if a.Emit != nil {
		a.Emit(event)
	}
}
