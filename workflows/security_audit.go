package workflows

import "harnais/graph"

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
	s *Shared,
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

					Worker: s.Reviewer(),

					JoinAll: true,
				},
			)

			addEdge(g, "planner", "security")
			addEdge(g, "security", "reviewer")

			return g
		},
	}
}
