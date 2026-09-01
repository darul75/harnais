package workflows

import (
	"harnais/agent"
	"harnais/graph"
	"harnais/llm"
)

const BasicWorkflowID = "basic"

const basicPrompt = `You are a general assistant.

A user request is provided at runtime.

Answer the request directly with a concise, helpful response.

Do not attempt to modify files, run commands, or use any tools.
If the request requires actions beyond answering (such as implementing code
or auditing a codebase), say so and stop.`

// BasicWorkflow is the fallback workflow for any request that is not
// clearly a specialized task. It answers directly with a single LLM
// step and no tools.
func BasicWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: BasicWorkflowID,

		Title: "General Assistant",

		Description: "Answer a general question or request directly with a single LLM step.",

		Build: func() *graph.Graph {

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID: "answerer",

					Worker: &agent.LoopAgent{
						AgentID: "answerer",

						Prompt: basicPrompt,

						LLMFactory: func() agent.LLM {
							return llm.NewOpenAI("", "")
						},
					},
				},
			)

			return g
		},
	}
}
