package workflows

import (
	"fmt"
	"sort"

	"harnais/graph"
	"harnais/tools"
)

// Workflow is a named, selectable graph definition.
type Workflow struct {
	ID string

	Title string

	Description string

	// Keywords used by the keyword-based selection step.
	// Matches are case-insensitive against the user request.
	Keywords []string

	// ManualOnly workflows are never chosen automatically by keyword
	// matching or LLM classification; they run only when explicitly
	// selected in the sidebar.
	ManualOnly bool

	// Isolated workflows operate in a per-run workspace directory
	// (workspace/coding/runs/<runID>/), so their agents only see the
	// run's content.
	Isolated bool

	// Build returns a fresh graph for each run, given the workspace the
	// run should operate in.
	Build func(runWorkspace *tools.Workspace) *graph.Graph
}

// Registry holds every available workflow.
type Registry struct {
	workflows []*Workflow

	byID map[string]*Workflow

	defaultID string
}

func NewRegistry(
	defaultID string,
	workflows ...*Workflow,
) (*Registry, error) {

	registry := &Registry{
		workflows: make(
			[]*Workflow,
			0,
			len(workflows),
		),

		byID: make(
			map[string]*Workflow,
			len(workflows),
		),

		defaultID: defaultID,
	}

	for _, workflow := range workflows {

		if workflow.ID == "" {
			return nil, fmt.Errorf(
				"workflow ID cannot be empty",
			)
		}

		if workflow.Build == nil {
			return nil, fmt.Errorf(
				"workflow %q has no build function",
				workflow.ID,
			)
		}

		if _, exists :=
			registry.byID[workflow.ID]; exists {

			return nil, fmt.Errorf(
				"duplicate workflow ID %q",
				workflow.ID,
			)
		}

		registry.workflows =
			append(
				registry.workflows,
				workflow,
			)

		registry.byID[workflow.ID] =
			workflow
	}

	if registry.Default() == nil {
		return nil, fmt.Errorf(
			"default workflow %q not found",
			defaultID,
		)
	}

	return registry, nil
}

func (r *Registry) All() []*Workflow {

	result := make(
		[]*Workflow,
		len(r.workflows),
	)

	copy(result, r.workflows)

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].ID < result[j].ID
		},
	)

	return result
}

func (r *Registry) Get(
	id string,
) (*Workflow, bool) {

	workflow, ok :=
		r.byID[id]

	return workflow, ok
}

func (r *Registry) Default() *Workflow {

	workflow, ok :=
		r.byID[r.defaultID]

	if !ok {
		return nil
	}

	return workflow
}

func (r *Registry) IDs() []string {

	workflows :=
		r.All()

	ids := make(
		[]string,
		len(workflows),
	)

	for i, workflow := range workflows {
		ids[i] = workflow.ID
	}

	return ids
}
