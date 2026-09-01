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

type LoopAgent struct {
	AgentID string

	Prompt string

	LLM LLM

	LLMFactory func() LLM

	Tools map[string]Tool
}

func (a *LoopAgent) ID() string {
	return a.AgentID
}

func (a *LoopAgent) newLLM() LLM {

	if a.LLMFactory != nil {
		return a.LLMFactory()
	}

	return a.LLM
}

func (a *LoopAgent) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	executionContext, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return graph.WorkerResult{}, fmt.Errorf(
			"agent %q: missing execution context",
			a.AgentID,
		)
	}

	agentExecution :=
		executionContext.Run.StartAgentExecution(
			executionContext.ExecutionID,
			a.AgentID,
		)

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentStarted,

			AgentID: a.AgentID,

			Data: map[string]any{
				"agentExecutionId": agentExecution.ID,

				"message": a.Prompt,
			},
		},
	)

	result, err :=
		a.runAgent(
			ctx,
			agentExecution,
			Input{
				Message: a.Prompt,
				State:   input.State,
			},
		)

	executionContext.Run.CompleteAgentExecution(
		agentExecution.ID,
		err,
	)

	if err != nil {
		return graph.WorkerResult{}, err
	}

	output :=
		graph.State{
			"agent_output": result.Output,
		}

	output.Merge(
		result.State,
	)

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentCompleted,

			AgentID: a.AgentID,

			Data: map[string]any{
				"agentExecutionId": agentExecution.ID,

				"output": result.Output,
			},
		},
	)

	return graph.WorkerResult{
		State: output,
	}, nil
}

func (a *LoopAgent) runAgent(
	ctx context.Context,
	agentExecution *graph.AgentExecution,
	input Input,
) (Result, error) {

	executionContext, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return Result{}, fmt.Errorf(
			"agent %q: missing execution context",
			a.AgentID,
		)
	}

	llm := a.newLLM()

	if llm == nil {
		return Result{}, fmt.Errorf(
			"agent %q: no LLM configured",
			a.AgentID,
		)
	}

	messages :=
		[]Message{
			{
				Role: "user",

				Content: input.Message,
			},
		}

	activitySequence := 0
	llmSequence := 0
	toolSequence := 0

	for {

		// ========================================================
		// LLM
		// ========================================================

		activitySequence++
		llmSequence++

		messageRecords :=
			make(
				[]graph.MessageRecord,
				0,
				len(messages),
			)

		for _, message := range messages {

			messageRecords =
				append(
					messageRecords,
					graph.MessageRecord{
						Role: message.Role,

						Content: message.Content,
					},
				)
		}

		activity :=
			executionContext.Run.StartAgentActivity(
				agentExecution.ID,
				activitySequence,
				graph.ActivityLLM,
			)

		llmCall :=
			executionContext.Run.StartLLMCall(
				agentExecution.ID,
				activity.ID,
				llmSequence,
				messageRecords,
			)

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventLLMStarted,

				AgentID: a.AgentID,

				Data: map[string]any{
					"agentExecutionId": agentExecution.ID,

					"activityId": activity.ID,

					"llmCallId": llmCall.ID,

					"sequence": llmSequence,
				},
			},
		)

		response, err :=
			llm.Generate(
				ctx,
				messages,
			)

		requestedTool := ""

		if response.ToolCall != nil {
			requestedTool =
				response.ToolCall.Name
		}

		executionContext.Run.CompleteLLMCall(
			llmCall.ID,
			response.Text,
			requestedTool,
			err,
		)

		executionContext.Run.CompleteAgentActivity(
			activity.ID,
			err,
		)

		if err != nil {

			graph.EmitEvent(
				ctx,
				graph.Event{
					Time: time.Now(),

					Type: graph.EventLLMCompleted,

					AgentID: a.AgentID,

					Message: err.Error(),

					Data: map[string]any{
						"agentExecutionId": agentExecution.ID,

						"activityId": activity.ID,

						"llmCallId": llmCall.ID,

						"sequence": llmSequence,
					},
				},
			)

			return Result{}, err
		}

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventLLMCompleted,

				AgentID: a.AgentID,

				Data: map[string]any{
					"agentExecutionId": agentExecution.ID,

					"activityId": activity.ID,

					"llmCallId": llmCall.ID,

					"sequence": llmSequence,

					"hasToolCall": response.ToolCall != nil,

					"tool": requestedTool,
				},
			},
		)

		// ========================================================
		// Done
		// ========================================================

		if response.ToolCall == nil {
			return Result{
				Output: response.Text,
			}, nil
		}

		// ========================================================
		// Tool
		// ========================================================

		call :=
			response.ToolCall

		tool, exists :=
			a.Tools[call.Name]

		if !exists {
			return Result{}, fmt.Errorf(
				"agent %q requested unknown tool %q",
				a.AgentID,
				call.Name,
			)
		}

		activitySequence++
		toolSequence++

		toolActivity :=
			executionContext.Run.StartAgentActivity(
				agentExecution.ID,
				activitySequence,
				graph.ActivityTool,
			)

		toolCall :=
			executionContext.Run.StartToolCall(
				agentExecution.ID,
				toolActivity.ID,
				toolSequence,
				call.Name,
				call.Input,
			)

		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventToolStarted,

				AgentID: a.AgentID,

				ToolID: call.Name,

				Data: map[string]any{
					"agentExecutionId": agentExecution.ID,

					"activityId": toolActivity.ID,

					"toolCallId": toolCall.ID,

					"sequence": toolSequence,

					"input": call.Input,
				},
			},
		)

		toolResult, err :=
			tool.Execute(
				ctx,
				call.Input,
			)

		executionContext.Run.CompleteToolCall(
			toolCall.ID,
			toolResult,
			err,
		)

		executionContext.Run.CompleteAgentActivity(
			toolActivity.ID,
			err,
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

					Data: map[string]any{
						"agentExecutionId": agentExecution.ID,

						"activityId": toolActivity.ID,

						"toolCallId": toolCall.ID,

						"sequence": toolSequence,
					},
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
					"agentExecutionId": agentExecution.ID,

					"activityId": toolActivity.ID,

					"toolCallId": toolCall.ID,

					"sequence": toolSequence,

					"output": toolResult,
				},
			},
		)

		// ========================================================
		// Continue conversation
		// ========================================================

		messages =
			append(
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
