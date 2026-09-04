package workflows

import (
	"context"
	"fmt"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

const PDFWorkflowID = "pdf"

const pdfSummarizerPrompt = `You are a document summarizer.

The runtime state contains a "pdf_text" value with the text extracted
from an uploaded PDF document.

Produce a thorough Markdown summary of the most important content in
that document. The summary must be detailed: do NOT shrink it too
much. Keep every important point, concept, argument, number, name,
finding, and conclusion. Structure it with:

# <Title of the document>

## Executive summary
A concise overview of what the document is about and its purpose.

## Key points
The most important ideas, organized by topic with subheadings as
needed. Preserve concrete facts, figures, definitions, and names.

## Detailed breakdown
Go section by section through the document, capturing what matters
in each part. Do not leave out substantial content.

## Conclusions / takeaways
What the document concludes, recommends, or implies.

Be faithful to the source. Do not invent content that is not in the
document. If the "pdf_text" value is empty or says extraction failed,
write a single sentence explaining that no readable text was found.`

// PDFWorkflow lets the user upload a PDF and get a comprehensive
// Markdown summary of its most important content.
//
// It is manual-only: it can only be selected explicitly from the
// sidebar, never by keyword or LLM classification, because it needs
// an uploaded file.
func PDFWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
) *Workflow {

	return &Workflow{
		ID: PDFWorkflowID,

		Title: "PDF Summary",

		Description: "Upload a PDF and get a detailed Markdown summary of its most important content, saved as a report.",

		ManualOnly: true,

		Build: func(ws *tools.Workspace) *graph.Graph {

			s :=
				NewShared(base, store, hub)

			s.SetRunWorkspace(ws)

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID: "read_pdf",

					Worker: s.ReadPDF(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "summarize",

					Worker: s.ProseAgent(
						"pdf-summarizer",
						pdfSummarizerPrompt,
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "write_report",

					Worker: s.WriteReport(
						"pdf-summary",
						"agent_output",
					),
				},
			)

			addEdge(g, "read_pdf", "summarize")
			addEdge(g, "summarize", "write_report")

			return g
		},
	}
}

// ReadPDF returns a worker that extracts text from the uploaded PDF
// referenced by state["pdf_path"] and stores it in state["pdf_text"].
func (s *Shared) ReadPDF() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"read_pdf",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			pdfPath, _ :=
				state["pdf_path"].(string)

			if pdfPath == "" {
				return nil, fmt.Errorf(
					"read_pdf: pdf_path is missing",
				)
			}

			resolved, err :=
				s.workspace.Resolve(pdfPath)

			if err != nil {
				return nil, err
			}

			text, err :=
				tools.ExtractPDFText(resolved)

			if err != nil {
				return graph.State{
					"pdf_text": fmt.Sprintf(
						"[error] %v",
						err,
					),
				}, nil
			}

			return graph.State{
				"pdf_text": text,
			}, nil
		},
	)
}
