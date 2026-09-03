package workflows

import (
	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

// Register builds the full registry of available workflows
// around the provided workspace and settings store. questionHub
// dispatches user answers to workers blocked on OpenCode questions.
func Register(
	workspace *tools.Workspace,
	store *config.Store,
	questionHub *graph.QuestionHub,
) (*Registry, error) {

	s :=
		NewShared(workspace, store, questionHub)

	return NewRegistry(
		BasicWorkflowID,

		BasicWorkflow(s),

		OpenCodeCodingWorkflow(s),

		ResearchWorkflow(s),

		ContentWorkflow(s),

		PDFWorkflow(s),

		SecurityAuditWorkflow(s),

		RefactorWorkflow(s),

		TTSWorkflow(s),

		GmailWorkflow(s),
	)
}
