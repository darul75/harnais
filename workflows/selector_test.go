package workflows

import (
	"context"
	"testing"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()

	registry, err := Register(
		tools.NewWorkspace("/tmp/harnais-test-workspace"),
		config.NewStore(""),
		graph.NewQuestionHub(),
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

	if workflow.ID != BasicWorkflowID {
		t.Errorf("expected default %s, got %s", BasicWorkflowID, workflow.ID)
	}
}

func TestRegistryIncludesOpenCode(t *testing.T) {

	registry := testRegistry(t)

	if _, ok :=
		registry.Get(OpenCodeCodingWorkflowID); !ok {

		t.Fatalf(
			"expected %q workflow",
			OpenCodeCodingWorkflowID,
		)
	}
}

func TestRegistryIncludesResearchAndContent(t *testing.T) {

	registry := testRegistry(t)

	for _, id := range []string{
		ResearchWorkflowID,
		ContentWorkflowID,
		PDFWorkflowID,
	} {

		if _, ok :=
			registry.Get(id); !ok {

			t.Errorf(
				"expected %q workflow",
				id,
			)
		}
	}
}

func TestSelectPDFManualOnly(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"summarize the uploaded pdf document",
		"",
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID == PDFWorkflowID {
		t.Errorf("pdf workflow must not be auto-selected (manual only), got %s", workflow.ID)
	}
}

func TestSelectPDFExplicit(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"anything",
		PDFWorkflowID,
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != PDFWorkflowID {
		t.Errorf("expected %s, got %s", PDFWorkflowID, workflow.ID)
	}
}

func TestSelectWeather(t *testing.T) {
	registry := testRegistry(t)
	selector := NewSelector(registry, nil)

	workflow, err := selector.Select(
		context.Background(),
		"What is the weather in Paris",
		"",
	)

	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if workflow.ID != BasicWorkflowID {
		t.Errorf("expected %s, got %s", BasicWorkflowID, workflow.ID)
	}
}
