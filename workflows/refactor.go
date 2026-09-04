package workflows

import (
	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

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

// RefactorWorkflow plans, refactors, tests, and reviews with the
// same bounded retry loop as the coding workflow: test failures and
// reviewer rejections send the result back to the coder, capped at
// two review iterations.
func RefactorWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
) *Workflow {

	return &Workflow{
		ID: RefactorWorkflowID,

		Title: "Code Refactor",

		Description: "Plan and apply a behaviour-preserving refactor, verify with tests, then review with a bounded retry loop until approved.",

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

		Isolated: true,

		Build: func(ws *tools.Workspace) *graph.Graph {

			s :=
				NewShared(base, store, hub)

			s.SetRunWorkspace(ws)

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID:     "planner",
					Worker: s.OpenCodePlanner(opencodePlannerPrompt),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "write_plan_report",

					Worker: s.WriteReport(
						"plan",
						"plan",
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID:     "coder",
					Worker: s.OpenCodeCoder(refactorPrompt),
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

					Worker: s.OpenCodeReviewer(
						opencodeReviewerPrompt,
					),

					JoinAll: true,
				},
			)

			addNode(
				g,
				&graph.Node{
					ID:     "review_gate",
					Worker: s.ReviewGate(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "write_review_report",

					Worker: s.WriteReport(
						"review",
						"review_feedback",
					),
				},
			)

			addEdge(g, "planner", "coder")
			addEdge(g, "planner", "write_plan_report")

			addEdge(g, "coder", "tester")
			addEdge(g, "coder", "security")

			addConditionalEdge(
				g,
				"tester",
				"coder",
				func(state graph.State) bool {
					passed, _ :=
						state["tests_passed"].(bool)

					attempts, _ :=
						state["test_attempts"].(int)

					return !passed && attempts < 3
				},
			)

			addConditionalEdge(
				g,
				"tester",
				"reviewer",
				func(state graph.State) bool {
					passed, _ :=
						state["tests_passed"].(bool)

					attempts, _ :=
						state["test_attempts"].(int)

					return passed || attempts >= 3
				},
			)

			addEdge(g, "security", "reviewer")
			addEdge(g, "reviewer", "review_gate")

			addConditionalEdge(
				g,
				"review_gate",
				"coder",
				func(state graph.State) bool {
					approved, _ :=
						state["approved"].(bool)

					attempts, _ :=
						state["review_attempts"].(int)

					return !approved && attempts < 2
				},
			)

			addConditionalEdge(
				g,
				"review_gate",
				"write_review_report",
				func(state graph.State) bool {
					approved, _ :=
						state["approved"].(bool)

					attempts, _ :=
						state["review_attempts"].(int)

					return approved || attempts >= 2
				},
			)

			return g
		},
	}
}
