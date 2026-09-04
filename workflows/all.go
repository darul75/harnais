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

	return NewRegistry(
		BasicWorkflowID,

		BasicWorkflow(workspace, store, questionHub),

		OpenCodeCodingWorkflow(workspace, store, questionHub),

		ResearchWorkflow(workspace, store, questionHub),

		ContentWorkflow(workspace, store, questionHub),

		PDFWorkflow(workspace, store, questionHub),

		SecurityAuditWorkflow(workspace, store, questionHub),

		RefactorWorkflow(workspace, store, questionHub),

		TTSWorkflow(workspace, store, questionHub),

		GmailWorkflow(workspace, store, questionHub),
	)
}
