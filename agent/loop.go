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
}

func (a *LoopAgent) ID() string {
	return a.AgentID
}

// ------------------------------------------------------------
// Worker implementation
// ------------------------------------------------------------

func (a *LoopAgent) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	result, err :=
		a.runAgent(
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

	output.Merge(
		result.State,
	)

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

	_, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return Result{}, fmt.Errorf(
			"agent %q: missing execution context",
			a.AgentID,
		)
	}

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentStarted,

			AgentID: a.AgentID,

			Data: map[string]any{
				"message": input.Message,
			},
		},
	)

	messages := []Message{
		{
			Role:    "user",
			Content: input.Message,
		},
	}

	for {

		// --------------------------------------------------
		// LLM
		// --------------------------------------------------

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventLLMStarted,

				AgentID: a.AgentID,
			},
		)

		response, err :=
			a.LLM.Generate(
				ctx,
				messages,
			)

		if err != nil {
			return Result{}, err
		}

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventLLMCompleted,

				AgentID: a.AgentID,

				Data: map[string]any{
					"hasToolCall": response.ToolCall != nil,
				},
			},
		)

		// --------------------------------------------------
		// Finished
		// --------------------------------------------------

		if response.ToolCall == nil {

			graph.EmitEvent(
				ctx,
				graph.Event{
					Time: time.Now(),

					Type: graph.EventAgentCompleted,

					AgentID: a.AgentID,

					Data: map[string]any{
						"output": response.Text,
					},
				},
			)

			return Result{
				Output: response.Text,
			}, nil
		}

		// --------------------------------------------------
		// Tool
		// --------------------------------------------------

		call :=
			response.ToolCall

		tool, ok :=
			a.Tools[call.Name]

		if !ok {
			return Result{}, fmt.Errorf(
				"agent %q requested unknown tool %q",
				a.AgentID,
				call.Name,
			)
		}

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventToolStarted,

				AgentID: a.AgentID,

				ToolID: call.Name,

				Data: map[string]any{
					"input": call.Input,
				},
			},
		)

		toolResult, err :=
			tool.Execute(
				ctx,
				call.Input,
			)

		if err != nil {

			graph.EmitEvent(
				ctx,
				graph.Event{
					Time: time.Now(),

					Type: graph.EventToolFailed,

					AgentID: a.AgentID,

					ToolID: call.Name,

					Message: err.Error(),
				},
			)

			return Result{}, err
		}

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventToolCompleted,

				AgentID: a.AgentID,

				ToolID: call.Name,

				Data: map[string]any{
					"output": toolResult,
				},
			},
		)

		// --------------------------------------------------
		// Feed result back into conversation.
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
