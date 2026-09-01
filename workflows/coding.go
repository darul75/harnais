package workflows

import "harnais/graph"

const CodingWorkflowID = "coding"

const coderPrompt = `You are an autonomous coding agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Your job is to implement the user's requested change.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant existing code before changing anything.

Implement only the requested change.
Run appropriate tests.
Inspect the final diff when available.

Work only inside the workspace.
Do not access the harness source code.
Do not fabricate tool results or test results.`

const securityPrompt = `You are a security review agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Review the implementation related to the user's request for security issues.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant implementation before reaching conclusions.

Do not modify files.
Use git_diff when useful.

Report concrete security findings with affected files and reasoning.
If there are no significant issues, say so explicitly.

Work only inside the workspace.
Do not fabricate findings or tool results.`

// CodingWorkflow is the default workflow: plan, implement
// with a coder and security agent, test with a retry loop,
// and review. Used as the fallback for any user request.
func CodingWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: CodingWorkflowID,

		Title: "Coding Implementation",

		Description: "Plan, implement, and test a requested feature or bug fix, with security review and a retry loop until tests pass.",

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
			"refactor",
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
					ID:     "coder",
					Worker: s.Coder(coderPrompt),
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
