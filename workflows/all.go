package workflows

import (
	"harnais/config"
	"harnais/tools"
)

// Register builds the full registry of available workflows
// around the provided workspace and settings store.
func Register(
	workspace *tools.Workspace,
	store *config.Store,
) (*Registry, error) {

	s :=
		NewShared(workspace, store)

	return NewRegistry(
		BasicWorkflowID,

		BasicWorkflow(s),

		CodingWorkflow(s),

		OpenCodeCodingWorkflow(s),

		ResearchWorkflow(s),

		ContentWorkflow(s),

		SecurityAuditWorkflow(s),

		RefactorWorkflow(s),

		TTSWorkflow(s),

		GmailWorkflow(s),
	)
}
