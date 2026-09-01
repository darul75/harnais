package workflows

import (
	"context"
	"fmt"
	"strings"

	"harnais/graph"
)

const ResearchWorkflowID = "research"

const researchPlannerPrompt = `You are a research planner.

A user request is provided at runtime.

Break the request into EXACTLY three distinct research sub-topics that
together cover the request well.

Reply with ONLY the three sub-topics, one per line.
Do not number them, do not add headings, and do not add any other text.`

const researchSubTopicPrompt = `You are a web researcher.

Research the sub-topic provided under "subtopic_%d" in the runtime state.
Use web search to gather accurate, up-to-date information.

Write your findings to %q as Markdown.
Structure the report with:
- A short introduction
- Key findings (bullet points with concrete facts)
- Sources (list the URLs you used)

Aim for 400-700 words. Do not invent facts or citations.`

const researchSynthesizerPrompt = `You are a research synthesizer.

Read the research reports at:
- reports/research-1.md
- reports/research-2.md
- reports/research-3.md

Merge them into ONE coherent briefing that answers the user's original
request. Produce the briefing as Markdown with:

# <Title>

## Executive summary
Short overview.

## Key findings
The most important points across all streams, with citations where
available.

## Sources
Combined source list.

Be concise and factual. Do not fabricate anything.`

// ResearchWorkflow plans a research question into three parallel
// web-research streams and synthesizes a briefing report.
func ResearchWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: ResearchWorkflowID,

		Title: "Deep Research",

		Description: "Split a question into three parallel web-research streams, then synthesize a Markdown briefing report.",

		Keywords: []string{
			"research",
			"investigate",
			"briefing",
			"deep dive",
			"explore",
			"learn about",
			"summary",
			"find out",
		},

		Build: func() *graph.Graph {

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID: "planner",

					Worker: s.ProseAgent(
						"research-planner",
						researchPlannerPrompt,
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "split_topics",

					Worker: graph.NewFuncWorker(
						"split_topics",

						func(
							ctx context.Context,
							state graph.State,
						) (graph.State, error) {

							plan, _ :=
								state["agent_output"].(string)

							lines :=
								strings.Split(
									strings.TrimSpace(plan),
									"\n",
								)

							next :=
								graph.State{}

							for i := 0; i < 3; i++ {

								topic := ""

								if i < len(lines) {
									topic =
										strings.TrimSpace(
											lines[i],
										)
								}

								next[fmt.Sprintf(
									"subtopic_%d",
									i,
								)] = topic
							}

							return next, nil
						},
					),
				},
			)

			// Three fixed parallel research streams. Each writes
			// its own report file so parallel outputs do not
			// overwrite each other in shared state.
			for i := 1; i <= 3; i++ {

				addNode(
					g,
					&graph.Node{
						ID: fmt.Sprintf(
							"researcher-%d",
							i,
						),

						Worker: s.ResearchAgent(
							fmt.Sprintf(
								"researcher-%d",
								i,
							),
							fmt.Sprintf(
								researchSubTopicPrompt,
								i-1,
								fmt.Sprintf(
									"reports/research-%d.md",
									i,
								),
							),
						),
					},
				)

				addEdge(
					g,
					"split_topics",
					fmt.Sprintf(
						"researcher-%d",
						i,
					),
				)
			}

			addNode(
				g,
				&graph.Node{
					ID: "synthesizer",

					Worker: s.ProseAgent(
						"research-synthesizer",
						researchSynthesizerPrompt,
					),

					JoinAll: true,
				},
			)

			for i := 1; i <= 3; i++ {

				addEdge(
					g,
					fmt.Sprintf(
						"researcher-%d",
						i,
					),
					"synthesizer",
				)
			}

			addNode(
				g,
				&graph.Node{
					ID: "write_report",

					Worker: s.WriteReport(
						"research-brief",
						"agent_output",
					),
				},
			)

			addEdge(g, "planner", "split_topics")
			addEdge(g, "synthesizer", "write_report")

			return g
		},
	}
}