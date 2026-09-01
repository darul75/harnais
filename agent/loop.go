package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"harnais/graph"
)

// ------------------------------------------------------------
// LoopAgent
// ------------------------------------------------------------
//
// A LoopAgent is a graph Worker.
//
// The graph executor invokes:
//
//	Node -> Worker -> LoopAgent
//
// The LoopAgent then performs:
//
//	LLM -> Tool -> LLM -> Tool -> ... -> final response
//
// The LoopAgent knows about:
//   - the LLM
//   - the available tools
//   - the agent execution
//
// It does NOT know about:
//   - EventBus
//   - HTTP
//   - SSE
//   - React
//

type LoopAgent struct {
	AgentID string

	Prompt string

	// Used when an LLM instance is provided directly.
	LLM LLM

	// Preferred when every AgentExecution should receive
	// its own independent LLM instance/state.
	LLMFactory func() LLM

	ToolRegistry *ToolRegistry
}

// ------------------------------------------------------------
// Worker interface
// ------------------------------------------------------------

func (a *LoopAgent) ID() string {
	return a.AgentID
}

// ------------------------------------------------------------
// LLM factory
// ------------------------------------------------------------

func (a *LoopAgent) newLLM() LLM {
	if a.LLMFactory != nil {
		return a.LLMFactory()
	}

	return a.LLM
}

// ------------------------------------------------------------
// Worker.Run
// ------------------------------------------------------------

func (a *LoopAgent) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	// --------------------------------------------------------
	// Get graph execution context.
	// --------------------------------------------------------

	executionContext, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return graph.WorkerResult{}, fmt.Errorf(
			"agent %q: missing execution context",
			a.AgentID,
		)
	}

	// --------------------------------------------------------
	// Create runtime AgentExecution.
	// --------------------------------------------------------

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

	// --------------------------------------------------------
	// Run actual agent loop.
	// --------------------------------------------------------

	result, err :=
		a.runAgent(
			ctx,
			agentExecution,
			Input{
				Message: a.Prompt,

				State: input.State,
			},
		)

	// --------------------------------------------------------
	// Complete AgentExecution.
	// --------------------------------------------------------

	executionContext.Run.CompleteAgentExecution(
		agentExecution.ID,
		err,
	)

	if err != nil {
		return graph.WorkerResult{}, err
	}

	// --------------------------------------------------------
	// Convert agent result into graph state.
	// --------------------------------------------------------

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

// ------------------------------------------------------------
// Agent loop
// ------------------------------------------------------------

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

	// --------------------------------------------------------
	// Obtain LLM.
	// --------------------------------------------------------

	llm :=
		a.newLLM()

	if llm == nil {
		return Result{}, fmt.Errorf(
			"agent %q: no LLM configured",
			a.AgentID,
		)
	}

	// --------------------------------------------------------
	// Obtain tool definitions.
	// --------------------------------------------------------

	var toolDefinitions []ToolDefinition

	if a.ToolRegistry != nil {
		toolDefinitions =
			a.ToolRegistry.Definitions()
	}

	// --------------------------------------------------------
	// Serialize runtime state.
	// --------------------------------------------------------

	stateJSON, err :=
		json.MarshalIndent(
			input.State,
			"",
			"  ",
		)

	if err != nil {
		return Result{}, fmt.Errorf(
			"serialize agent state: %w",
			err,
		)
	}

	// --------------------------------------------------------
	// Initial conversation.
	// --------------------------------------------------------

	messages :=
		[]Message{
			{
				Role: "user",

				Content: fmt.Sprintf(
					"%s\n\nRuntime state:\n%s",
					input.Message,
					string(stateJSON),
				),
			},
		}

	// --------------------------------------------------------
	// Sequence numbers.
	// --------------------------------------------------------

	activitySequence := 0
	llmSequence := 0
	toolSequence := 0

	// --------------------------------------------------------
	// Agent loop.
	// --------------------------------------------------------

	for {

		// ========================================================
		// LLM activity
		// ========================================================

		activitySequence++
		llmSequence++

		// --------------------------------------------------------
		// Record messages for observability.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// Create activity.
		// --------------------------------------------------------

		activity :=
			executionContext.Run.StartAgentActivity(
				agentExecution.ID,
				activitySequence,
				graph.ActivityLLM,
			)

		// --------------------------------------------------------
		// Create LLM call.
		// --------------------------------------------------------

		llmCall :=
			executionContext.Run.StartLLMCall(
				agentExecution.ID,
				activity.ID,
				llmSequence,
				messageRecords,
			)

		// --------------------------------------------------------
		// Emit llm.started.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// Call LLM.
		// --------------------------------------------------------

		response, err :=
			llm.Generate(
				ctx,
				messages,
				toolDefinitions,
			)

		requestedTool := ""

		if len(response.ToolCalls) > 0 {
			requestedTool =
				response.ToolCalls[0].Name
		}

		// --------------------------------------------------------
		// Store LLM result.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// LLM error.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// Emit llm.completed.
		// --------------------------------------------------------

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

					"hasToolCall": len(
						response.ToolCalls,
					) > 0,

					"tool": requestedTool,
				},
			},
		)

		// ========================================================
		// Final answer
		// ========================================================

		if len(response.ToolCalls) == 0 {

			return Result{
				Output: response.Text,
			}, nil
		}

		// ========================================================
		// Tool calls (a response may contain several, run in
		// parallel by the model; execute them all before the next
		// LLM turn).
		// ========================================================

		for _, call := range response.ToolCalls {

		if a.ToolRegistry == nil {
			return Result{}, fmt.Errorf(
				"agent %q has no tool registry",
				a.AgentID,
			)
		}

		tool, exists :=
			a.ToolRegistry.Get(
				call.Name,
			)

		if !exists {
			return Result{}, fmt.Errorf(
				"agent %q requested unknown tool %q",
				a.AgentID,
				call.Name,
			)
		}

		// --------------------------------------------------------
		// Create tool activity.
		// --------------------------------------------------------

		activitySequence++
		toolSequence++

		toolActivity :=
			executionContext.Run.StartAgentActivity(
				agentExecution.ID,
				activitySequence,
				graph.ActivityTool,
			)

		// --------------------------------------------------------
		// Create ToolCall.
		// --------------------------------------------------------

		toolCall :=
			executionContext.Run.StartToolCall(
				agentExecution.ID,
				toolActivity.ID,
				toolSequence,
				call.Name,
				call.Input,
			)

		// --------------------------------------------------------
		// Emit tool.started.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// Execute tool.
		// --------------------------------------------------------

		toolResult, err :=
			tool.Execute(
				ctx,
				call.Input,
			)

		// --------------------------------------------------------
		// Store tool result.
		// --------------------------------------------------------

		executionContext.Run.CompleteToolCall(
			toolCall.ID,
			toolResult,
			err,
		)

		executionContext.Run.CompleteAgentActivity(
			toolActivity.ID,
			err,
		)

		// --------------------------------------------------------
		// Tool failure
		// --------------------------------------------------------
		//
		// A tool failure is returned to the LLM as information.
		// It does not automatically terminate the agent.
		//

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

			// Give the LLM a chance to recover.
			messages =
				append(
					messages,

					Message{
						Role: "tool",

						Content: fmt.Sprintf(
							"Tool execution failed: %s",
							err.Error(),
						),

						CallID: call.CallID,
					},
				)

			continue
		}

		// --------------------------------------------------------
		// Tool completed.
		// --------------------------------------------------------

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

		// --------------------------------------------------------
		// Add successful tool output to conversation.
		// --------------------------------------------------------

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

					CallID: call.CallID,
				},
			)
		}
	}
}
