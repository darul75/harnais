package workflows

import (
	"context"
	"testing"

	"harnais/tools"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()

	registry, err := Register(
		tools.NewWorkspace("/tmp/harnais-test-workspace"),
	)

	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	return registry
}

func TestSelectExplicit(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"anything here",
		RefactorWorkflowID,
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != RefactorWorkflowID {
		t.Errorf("expected %s, got %s", RefactorWorkflowID, workflow.ID)
	}
}

func TestSelectKeywordSecurity(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"audit the auth code for vulnerabilities",
		"",
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != SecurityAuditWorkflowID {
		t.Errorf("expected %s, got %s", SecurityAuditWorkflowID, workflow.ID)
	}
}

func TestSelectKeywordRefactor(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"please refactor and simplify the payment module",
		"",
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != RefactorWorkflowID {
		t.Errorf("expected %s, got %s", RefactorWorkflowID, workflow.ID)
	}
}

func TestSelectFallbackDefault(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"completely unrelated gibberish request",
		"",
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != CodingWorkflowID {
		t.Errorf("expected default %s, got %s", CodingWorkflowID, workflow.ID)
	}
}