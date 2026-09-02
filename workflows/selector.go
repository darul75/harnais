package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"harnais/agent"
)

// Selector picks the most appropriate workflow for a user request.
type Selector struct {
	registry *Registry

	// Optional LLM factory used for classification when keyword
	// matching cannot decide. If nil, the keyword match result
	// (or default) is used directly.
	factory LLMFactory
}

// LLMFactory builds an LLM instance from the current settings.
type LLMFactory func() agent.LLM

func NewSelector(
	registry *Registry,
	factory LLMFactory,
) *Selector {

	return &Selector{
		registry: registry,
		factory:  factory,
	}
}

// Select returns the workflow to use for the given task.
//
//  1. If explicitID is provided and exists, it wins (sidebar override).
//  2. Keyword scoring against each workflow.
//  3. LLM classification when no keyword match is conclusive.
//  4. Default workflow as the final fallback.
func (s *Selector) Select(
	ctx context.Context,
	task string,
	explicitID string,
) (*Workflow, error) {

	if explicitID != "" {

		if workflow, ok :=
			s.registry.Get(explicitID); ok {

			return workflow, nil
		}
	}

	if workflow :=
		s.keywordMatch(task); workflow != nil {

		return workflow, nil
	}

	if s.factory != nil {

		workflow, err :=
			s.llmMatch(
				ctx,
				s.factory(),
				task,
			)

		if err == nil && workflow != nil {
			return workflow, nil
		}
	}

	return s.registry.Default(), nil
}

// ------------------------------------------------------------
// Keyword matching
// ------------------------------------------------------------

func (s *Selector) keywordMatch(
	task string,
) *Workflow {

	lower :=
		strings.ToLower(task)

	if lower == "" {
		return nil
	}

	var best *Workflow

	bestScore := 0

	for _, workflow := range s.registry.All() {

		// The default workflow is the fallback. It only wins
		// via explicit selection or the final fallback step,
		// never by competing on keywords.

		if workflow.ID ==
			s.registry.Default().ID {
			continue
		}

		// Manual-only workflows (e.g. PDF upload) are never
		// auto-selected; they require explicit sidebar choice.

		if workflow.ManualOnly {
			continue
		}

		score :=
			s.scoreWorkflow(
				workflow,
				lower,
			)

		if score > bestScore {

			bestScore = score
			best = workflow
		}
	}

	if bestScore > 0 {
		return best
	}

	return nil
}

func (s *Selector) scoreWorkflow(
	workflow *Workflow,
	lowerTask string,
) int {

	score := 0

	for _, keyword := range workflow.Keywords {

		keyword = strings.ToLower(keyword)

		if keyword == "" {
			continue
		}

		if strings.Contains(
			lowerTask,
			keyword,
		) {
			score++
		}
	}

	title :=
		strings.ToLower(workflow.Title)

	for _, word := range strings.Fields(title) {

		if len(word) < 3 {
			continue
		}

		if strings.Contains(
			lowerTask,
			word,
		) {
			score++
		}
	}

	return score
}

// ------------------------------------------------------------
// LLM classification
// ------------------------------------------------------------

func (s *Selector) llmMatch(
	ctx context.Context,
	llm agent.LLM,
	task string,
) (*Workflow, error) {

	ctx, cancel :=
		context.WithTimeout(
			ctx,
			30*time.Second,
		)

	defer cancel()

	var builder strings.Builder

	builder.WriteString(
		"Select the single best workflow for the user request below.\n",
	)

	builder.WriteString(
		"Reply with ONLY the workflow ID (one word).\n\n",
	)

	builder.WriteString(
		"Available workflows:\n",
	)

	for _, workflow := range s.registry.All() {

		// Manual-only workflows cannot be auto-selected.
		if workflow.ManualOnly {
			continue
		}

		fmt.Fprintf(
			&builder,
			"- %s: %s. %s\n",
			workflow.ID,
			workflow.Title,
			workflow.Description,
		)
	}

	fmt.Fprintf(
		&builder,
		"\nUser request: %s\n",
		task,
	)

	response, err :=
		llm.Generate(
			ctx,
			[]agent.Message{
				{
					Role:    "user",
					Content: builder.String(),
				},
			},
			nil,
		)

	if err != nil {
		return nil, err
	}

	id :=
		strings.TrimSpace(
			response.Text,
		)

	id =
		strings.ToLower(id)

	if workflow, ok :=
		s.registry.Get(id); ok {

		return workflow, nil
	}

	return nil, fmt.Errorf(
		"LLM returned unknown workflow ID %q",
		id,
	)
}
