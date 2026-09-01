package workflows

import "harnais/graph"

const OpenCodeCodingWorkflowID = "coding-opencode"

const opencodeCoderPrompt = `You are an autonomous coding agent.

A user request and, when available, a plan are provided.

Work in the current working directory.

Inspect the project first with list_files and read the relevant code before changing anything.
Implement only the requested change.
Run the relevant tests.
Inspect the final diff.

Work only inside the current working directory.
Do not modify the harness itself.
Do not fabricate tool results or test results.

Finish with a concise summary of what you changed and the test results.`

// OpenCodeCodingWorkflow mirrors the coding workflow but implements
// the coding step through the OpenCode CLI instead of the built-in
// LoopAgent coder. Both variants remain available in the registry.
func OpenCodeCodingWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: OpenCodeCodingWorkflowID,

		Title: "Coding Implementation (OpenCode)",

		Description: "Plan, implement with OpenCode, and test a requested feature or bug fix, with security review and a retry loop until tests pass.",

		Keywords: []string{
			"implement",
			"implementation",
			"feature",
			"fix",
			"bug",
			"code",
			"build",
			"add",
			"change",
			"opencode",
		},

		Build: func() *graph.Graph {

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID:     "planner",
					Worker: s.Planner(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "coder",

					Worker: s.OpenCodeCoder(
						opencodeCoderPrompt,
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID:     "security",
					Worker: s.Security(securityPrompt),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID:     "tester",
					Worker: s.Tester(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "reviewer",

					Worker: s.Reviewer(),

					JoinAll: true,
				},
			)

			addEdge(g, "planner", "coder")
			addEdge(g, "planner", "security")
			addEdge(g, "coder", "tester")

			addConditionalEdge(
				g,
				"tester",
				"coder",
				func(state graph.State) bool {
					passed, _ :=
						state["tests_passed"].(bool)
					return !passed
				},
			)

			addConditionalEdge(
				g,
				"tester",
				"reviewer",
				func(state graph.State) bool {
					passed, _ :=
						state["tests_passed"].(bool)
					return passed
				},
			)

			addEdge(g, "security", "reviewer")

			return g
		},
	}
}