package workflows

import (
	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

const SecurityAuditWorkflowID = "security_audit"

const securityAuditPrompt = `You are a security audit agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Audit the codebase related to the user's request for security issues.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant code before reaching conclusions.

Do not modify files.
Use git_diff when useful.

Report concrete security findings with affected files and reasoning.
If there are no significant issues, say so explicitly.

Work only inside the workspace.
Do not fabricate findings or tool results.`

// SecurityAuditWorkflow performs a read-only security review
// of the requested area, then reviews the findings.
func SecurityAuditWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
) *Workflow {

	return &Workflow{
		ID: SecurityAuditWorkflowID,

		Title: "Security Audit",

		Description: "Perform a read-only security review of the code related to the request, then review the findings.",

		Keywords: []string{
			"security",
			"audit",
			"vulnerability",
			"vulnerabilities",
			"injection",
			"exploit",
			"threat",
			"harden",
			"review security",
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
					ID: "security",
					Worker: s.Security(
						securityAuditPrompt,
					),
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

			addEdge(g, "planner", "security")
			addEdge(g, "planner", "write_plan_report")

			addEdge(g, "security", "reviewer")
			addEdge(g, "reviewer", "review_gate")
			addEdge(g, "review_gate", "write_review_report")

			return g
		},
	}
}
