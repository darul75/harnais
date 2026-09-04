package workflows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
	"harnais/tools/imapmail"
)

const GmailWorkflowID = "gmail"

const gmailDigestPrompt = `You are a personal email assistant.

The runtime state contains an "emails_markdown" value listing the emails
received in the configured Gmail account over the last few days, each
with its sender, subject, date, a snippet, and a link that opens the
message in Gmail.

Produce a Markdown digest of these emails as your final response. Use:

# Daily Gmail Digest

## Emails
For each email, an entry like:

### <Subject>
- **From:** <sender>
- **Date:** <date>
- **Summary:** one or two concise sentences about what the email says.
- **Open in Gmail:** <link>

Keep each summary short and factual. Do not invent details that are not
present in the email content. If the "emails_markdown" value is empty or
says there are no emails, write a single sentence saying no emails were
found for the period.`

// GmailWorkflow reads recent emails from the configured Gmail account,
// summarizes each one with an LLM, and writes a Markdown digest with a
// direct link to every message.
func GmailWorkflow(
	base *tools.Workspace,
	store *config.Store,
	hub *graph.QuestionHub,
) *Workflow {

	return &Workflow{
		ID: GmailWorkflowID,

		Title: "Gmail Digest",

		Description: "Fetch your recent Gmail messages via IMAP, summarize each one with an LLM, and save a Markdown digest with direct links.",
		Keywords: []string{
			"email",
			"gmail",
			"inbox",
			"digest",
			"mail",
			"messages",
			"daily summary",
			"inbox zero",
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
					ID: "fetch_emails",

					Worker: s.FetchEmails(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "digest",

					Worker: s.ProseAgent(
						"gmail-digest",
						gmailDigestPrompt,
					),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "write_report",

					Worker: s.WriteReport(
						"gmail-digest",
						"agent_output",
					),
				},
			)

			addEdge(g, "fetch_emails", "digest")
			addEdge(g, "digest", "write_report")

			return g
		},
	}
}

// FetchEmails connects to the configured Gmail account, reads the
// emails from the last daysBack days, and formats them as a Markdown
// block in the runtime state for the digest agent.
func (s *Shared) FetchEmails() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"fetch_emails",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			daysBack, _ :=
				strconv.Atoi(
					s.store.Get(
						"gmail",
						"daysBack",
					),
				)

			emails, err :=
				imapmail.FetchLatest(
					ctx,
					imapmail.Config{
						Email: s.store.Get(
							"gmail",
							"email",
						),
						AppPassword: s.store.Get(
							"gmail",
							"appPassword",
						),
						Host: s.store.Get(
							"gmail",
							"host",
						),
						Port: s.store.Get(
							"gmail",
							"port",
						),
						Mailbox: s.store.Get(
							"gmail",
							"mailbox",
						),
						DaysBack:    daysBack,
						MaxMessages: 100,
					},
				)

			if err != nil {
				return nil, err
			}

			markdown, err :=
				emailsMarkdown(emails)

			if err != nil {
				return nil, err
			}

			return graph.State{
				"emails_markdown": markdown,

				"email_count": len(emails),
			}, nil
		},
	)
}

// emailsMarkdown formats emails as a compact Markdown list, keeping a
// trimmed snippet of each body so the LLM prompt stays small.
func emailsMarkdown(
	emails []imapmail.Email,
) (string, error) {

	if len(emails) == 0 {
		return "No emails found for the configured period.", nil
	}

	var builder strings.Builder

	for i, email := range emails {

		if _, err :=
			fmt.Fprintf(
				&builder,
				"%d. **%s** — %s\n",
				i+1,
				email.Subject,
				email.Sender,
			); err != nil {

			return "", err
		}

		if !email.Date.IsZero() {
			fmt.Fprintf(
				&builder,
				"   Date: %s\n",
				email.Date.Format("2006-01-02 15:04"),
			)
		}

		if _, err :=
			fmt.Fprintf(
				&builder,
				"   Link: %s\n   Body: %s\n\n",
				email.Link,
				truncateEmailBody(email.Body),
			); err != nil {

			return "", err
		}
	}

	return builder.String(), nil
}

// truncateEmailBody caps an email body so the prompt stays within
// limits, keeping the opening lines which usually carry the gist.
func truncateEmailBody(body string) string {

	const maxRunes = 2000

	text :=
		strings.Join(
			strings.Fields(body),
			" ",
		)

	runes :=
		[]rune(text)

	if len(runes) <= maxRunes {
		return text
	}

	return string(runes[:maxRunes]) + "\u2026"
}
