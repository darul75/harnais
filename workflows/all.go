package workflows

import "harnais/tools"

// Register builds the full registry of available workflows
// around the provided workspace.
func Register(
	workspace *tools.Workspace,
) (*Registry, error) {

	s :=
		NewShared(workspace)

	return NewRegistry(
		CodingWorkflowID,

		CodingWorkflow(s),

		SecurityAuditWorkflow(s),

		RefactorWorkflow(s),
	)
}
