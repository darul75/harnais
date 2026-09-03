package workflows

import "harnais/graph"

const OpenCodeCodingWorkflowID = "coding-opencode"

const opencodePlannerPrompt = `You are a planning agent working inside the provided workspace.

A user request is provided at runtime.

Inspect the workspace with list_files and read the relevant existing code before proposing anything. Do not assume a project type or specific files.

Produce a concrete, structured Markdown plan that includes:
- The goal in one or two sentences
- The exact steps to implement the change, in order
- Which files to create or modify, and why
- Which tests to run to verify the change

Be specific enough that a coding agent can follow the plan without re-planning from scratch. Do not modify any files.`

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

const opencodeReviewerPrompt = `You are a code reviewer.

A user request, plan, and test output are provided when available. Review the change that was just implemented in the current working directory.

Inspect the diff with git diff and read the changed files before reaching conclusions.
Evaluate the change against the plan and the request:
- Does it implement what was asked?
- Is it correct, readable, and maintainable?
- Are tests passing or are failures explained?

End your response with a single line on its own, exactly one of:
VERDICT: APPROVED
VERDICT: REJECTED

Then provide concise, concrete feedback listing anything that must be fixed before approval. Do not modify any files.`

const planRevisionPrompt = `You are revising the plan you previously produced for the user's request in this same session.

The user has requested changes to the plan. Incorporate their feedback (provided below) and output the COMPLETE revised plan in the same structured Markdown format as before: the goal, the exact steps in order, which files to create or modify and why, and which tests to run.

Do not modify any files.`

// maxPlanAttempts caps how many times the user may request changes to
// the plan before the workflow proceeds to the coder anyway.
const maxPlanAttempts = 3

// OpenCodeCodingWorkflow plans, implements through the OpenCode CLI,
// tests, security-reviews, and then reviews with a bounded retry
// loop that sends the result back to the coder when tests fail or
// the reviewer rejects, capped at two review iterations.
func OpenCodeCodingWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: OpenCodeCodingWorkflowID,

		Title: "Coding Implementation (OpenCode)",

		Description: "Plan, implement with OpenCode, test, and review with a retry loop until tests pass and the reviewer approves (capped at two review iterations).",

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
					ID:     "plan_gate",
					Worker: s.PlanGate(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID:     "plan_revision",
					Worker: s.OpenCodePlanRevision(),
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

			addEdge(g, "planner", "write_plan_report")
			addEdge(g, "write_plan_report", "plan_gate")

			addConditionalEdge(
				g,
				"plan_gate",
				"coder",
				func(state graph.State) bool {
					approved, _ :=
						state["plan_approved"].(bool)

					attempts, _ :=
						state["plan_attempts"].(int)

					return approved ||
						attempts >= maxPlanAttempts
				},
			)

			addConditionalEdge(
				g,
				"plan_gate",
				"plan_revision",
				func(state graph.State) bool {
					approved, _ :=
						state["plan_approved"].(bool)

					attempts, _ :=
						state["plan_attempts"].(int)

					return !approved &&
						attempts < maxPlanAttempts
				},
			)

			addEdge(g, "plan_revision", "write_plan_report")

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
