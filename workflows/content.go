package workflows

import (
	"fmt"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

const ContentWorkflowID = "content"

const outlinePrompt = `You are a content outliner.

A user request is provided at runtime.

Produce a Markdown outline for the requested content:
- A title (H1)
- Section headings (H2) with a one-line note on what each covers

Reply with ONLY the outline. Do not write the full content.`

const draftPrompt = `You are a content writer.

The outline is available in the runtime state under "agent_output".
The runtime state contains a "reports_dir" value with the directory
for this run's reports.

Write the full piece of content following the outline, as Markdown.
Write it to a file named "content-draft.md" inside the reports_dir
directory with the write_file tool.

Make it well-structured, clear, and useful. Include the title and
headings from the outline.`

const editorPrompt = `You are a %s editor.

The runtime state contains a "reports_dir" value with the directory
for this run's reports.

Read "content-draft.md" inside the reports_dir directory with
read_file. Edit the draft for %s.
Write your edited version to a file named "%s" inside the reports_dir
directory with write_file, keeping the full document (do not summarize
or truncate it).`

const finalizerPrompt = `You are a content finalizer.

The runtime state contains a "reports_dir" value with the directory
for this run's reports.

Read these files inside the reports_dir directory:
- content-draft.md (the original draft)
- editor-tone.md
- editor-facts.md
- editor-clarity.md

Merge the best of the draft and the editors' versions into ONE final
document. Preserve the full content. Output the final document as
Markdown and nothing else.`

var contentEditors = []struct {
	id     string
	focus  string
	detail string
}{
	{"tone", "voice and tone", "improve voice and tone, making the writing consistent, engaging, and appropriate for the audience"},
	{"facts", "accuracy and facts", "verify facts, numbers, and claims; flag or fix anything inaccurate or unsupported"},
	{"clarity", "clarity and structure", "improve clarity, flow, and structure; simplify convoluted sentences without losing meaning"},
}

// ContentWorkflow turns a request into an outline, a full draft, and
// three parallel editorial passes, then produces a final document.
func ContentWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
) *Workflow {

	return &Workflow{
		ID: ContentWorkflowID,

		Title: "Content Pipeline",

		Description: "Outline, draft, and run three parallel editorial passes (tone, facts, clarity) to produce a polished Markdown document.",

		Keywords: []string{
			"content",
			"write",
			"draft",
			"article",
			"blog",
			"document",
			"essay",
			"edit",
			"copy",
			"prose",
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
					ID: "outline",

					Worker: s.ProseAgent(
						"content-outliner",
						outlinePrompt,
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "draft",

					Worker: s.ProseAgent(
						"content-drafter",
						draftPrompt,
					),
				},
			)

			for _, editor := range contentEditors {

				addNode(
					g,
					&graph.Node{
						ID: fmt.Sprintf(
							"editor-%s",
							editor.id,
						),

						Worker: s.ProseAgent(
							fmt.Sprintf(
								"content-editor-%s",
								editor.id,
							),
							fmt.Sprintf(
								editorPrompt,
								editor.focus,
								editor.detail,
								fmt.Sprintf(
									"editor-%s.md",
									editor.id,
								),
							),
						),
					},
				)

				addEdge(
					g,
					"draft",
					fmt.Sprintf(
						"editor-%s",
						editor.id,
					),
				)
			}

			addNode(
				g,
				&graph.Node{
					ID: "finalizer",

					Worker: s.ProseAgent(
						"content-finalizer",
						finalizerPrompt,
					),

					JoinAll: true,
				},
			)

			for _, editor := range contentEditors {

				addEdge(
					g,
					fmt.Sprintf(
						"editor-%s",
						editor.id,
					),
					"finalizer",
				)
			}

			addNode(
				g,
				&graph.Node{
					ID: "write_report",

					Worker: s.WriteReport(
						"content-final",
						"agent_output",
					),
				},
			)

			addEdge(g, "outline", "draft")
			addEdge(g, "finalizer", "write_report")

			return g
		},
	}
}
