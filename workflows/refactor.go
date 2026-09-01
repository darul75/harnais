package workflows

import "harnais/graph"

const RefactorWorkflowID = "refactor"

const refactorPrompt = `You are a refactoring agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Your job is to refactor the requested code without changing its behaviour.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant existing code before changing anything.

Improve structure, naming, and readability.
Do not introduce new features or change external behaviour.
Run tests to confirm nothing is broken.
Inspect the final diff when available.

Work only inside the workspace.
Do not access the harness source code.
Do not fabricate tool results or test results.`

// RefactorWorkflow plans, refactors, and tests the requested
// code, then reviews the result.
func RefactorWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: RefactorWorkflowID,

		Title: "Code Refactor",

		Description: "Plan and apply a behaviour-preserving refactor of the requested code, verify with tests, then review.",

		Keywords: []string{
			"refactor",
			"refactoring",
			"clean up",
			"cleanup",
			"simplify",
			"improve",
			"restructure",
			"rename",
			"extract",
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
					Worker: s.Coder(refactorPrompt),
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

			return g
		},
	}
}
