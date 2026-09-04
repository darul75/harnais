package workflows

import (
	"context"
	"fmt"
	"strings"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

const ResearchWorkflowID = "research"

const researchPlannerPrompt = `You are a research planner.

A user request is provided at runtime.

Break the request into one to three distinct research sub-topics that
together cover the request well. Use as many as you need, at most three.

Reply with ONLY the sub-topics, one per line.
Do not number them, do not add headings, and do not add any other text.`

const researchSubTopicPrompt = `You are a web researcher.

Research the sub-topic provided under "subtopic_%d" in the runtime state.
Use web search to gather accurate, up-to-date information.

The runtime state contains a "reports_dir" value with the directory
for this run's reports.

Write your findings as Markdown to a file named "research-%d.md"
inside the reports_dir directory, using the write_file tool.
Structure the report with:
- A short introduction
- Key findings (bullet points with concrete facts)
- Sources (list the URLs you used)

Aim for 400-700 words. Do not invent facts or citations.`

const researchSynthesizerPrompt = `You are a research synthesizer.

The runtime state contains a "reports_dir" value with the directory
for this run's reports.

Before reading, call the list_files tool on the reports_dir directory
to see which files exist. Each researcher writes a Markdown report
named "research-<n>.md", but only for the sub-topics that were
assigned; the files for unassigned streams do not exist.

Read every "research-*.md" file that exists and merge them into ONE
coherent briefing that answers the user's original request. Produce
the briefing as Markdown with:

# <Title>

## Executive summary
Short overview.

## Key findings
The most important points across all streams, with citations where
available.

## Sources
Combined source list.

Be concise and factual. Do not fabricate anything.`

// splitResearchTopics turns the planner's newline plan into up to
// three dense, non-empty sub-topics. Blank lines and empty slots are
// dropped so leftover slots stay empty and their researcher can skip.
func splitResearchTopics(
	plan string,
) []string {

	topics :=
		make([]string, 0, 3)

	for _, line := range strings.Split(
		strings.TrimSpace(plan),
		"\n",
	) {

		topic :=
			strings.TrimSpace(line)

		if topic == "" {
			continue
		}

		topics = append(
			topics,
			topic,
		)

		if len(topics) == 3 {
			break
		}
	}

	return topics
}

// ResearchWorkflow plans a research question into three parallel
// web-research streams and synthesizes a briefing report.
func ResearchWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
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

		Build: func(ws *tools.Workspace) *graph.Graph {

			s :=
				NewShared(base, store, hub)

			s.SetRunWorkspace(ws)

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

							topics :=
								splitResearchTopics(plan)

							next :=
								graph.State{}

							for i := 0; i < 3; i++ {

								topic := ""

								if i < len(topics) {
									topic =
										topics[i]
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
			// overwrite each other in shared state. Researchers
			// whose sub-topic is empty skip instantly without
			// calling the LLM.
			for i := 1; i <= 3; i++ {

				addNode(
					g,
					&graph.Node{
						ID: fmt.Sprintf(
							"researcher-%d",
							i,
						),

						Worker: skipWhenEmpty(
							s.ResearchAgent(
								fmt.Sprintf(
									"researcher-%d",
									i,
								),
								fmt.Sprintf(
									researchSubTopicPrompt,
									i-1,
									i,
								),
							),
							fmt.Sprintf(
								"subtopic_%d",
								i-1,
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
