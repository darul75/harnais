package workflows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"harnais/agent"
	"harnais/config"
	"harnais/graph"
	"harnais/llm"
	"harnais/tools"
	"harnais/workflows/opencode"
)

// Shared holds reusable node/worker builders so each workflow
// can compose distinct graphs from common components.
type Shared struct {
	// workspace is the directory agents operate in (the run workspace
	// for isolated workflows, otherwise the base workspace).
	workspace *tools.Workspace

	// base is the base workspace used for reports/uploads resolution.
	base *tools.Workspace

	store *config.Store

	// QuestionHub dispatches user answers to workers blocked on an
	// OpenCode clarifying question.
	QuestionHub *graph.QuestionHub
}

func NewShared(
	workspace *tools.Workspace,
	store *config.Store,
	questionHub *graph.QuestionHub,
) *Shared {

	return &Shared{
		workspace: workspace,
		base:      workspace,
		store:     store,

		QuestionHub: questionHub,
	}
}

// SetRunWorkspace points the agents at a per-run workspace while
// keeping the base workspace for reports/uploads resolution.
func (s *Shared) SetRunWorkspace(
	workspace *tools.Workspace,
) {

	s.workspace = workspace
}

// LLMFactory returns a factory that builds an LLM from the
// current settings store. It is evaluated when a run starts,
// so updated settings apply without a restart.
func (s *Shared) LLMFactory(
	kind string,
) func() agent.LLM {

	return s.store.LLMFactory(kind)
}

// ------------------------------------------------------------
// Coder agent
// ------------------------------------------------------------

func (s *Shared) Coder(
	prompt string,
) *agent.LoopAgent {

	toolRegistry :=
		agent.NewToolRegistry(

			tools.ListFiles{
				Workspace: s.workspace,
			},

			tools.ReadFile{
				Workspace: s.workspace,
			},

			tools.WriteFile{
				Workspace: s.workspace,
			},

			tools.RunCommand{
				Workspace: s.workspace,
			},

			tools.GitDiff{
				Workspace: s.workspace,
			},
		)

	return &agent.LoopAgent{
		AgentID: "coder-agent",

		Prompt: prompt,

		LLMFactory: s.LLMFactory(
			"openai",
		),

		ToolRegistry: toolRegistry,
	}
}

// ------------------------------------------------------------
// Security agent (read-only)
// ------------------------------------------------------------

const securityPrompt = `You are a security review agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Review the implementation related to the user's request for security issues.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant implementation before reaching conclusions.

Do not modify files.
Use git_diff when useful.

Report concrete security findings with affected files and reasoning.
If there are no significant issues, say so explicitly.

Work only inside the workspace.
Do not fabricate findings or tool results.`

func (s *Shared) Security(
	prompt string,
) *agent.LoopAgent {

	toolRegistry :=
		agent.NewToolRegistry(

			tools.ListFiles{
				Workspace: s.workspace,
			},

			tools.ReadFile{
				Workspace: s.workspace,
			},

			tools.GitDiff{
				Workspace: s.workspace,
			},
		)

	return &agent.LoopAgent{
		AgentID: "security-agent",

		Prompt: prompt,

		LLMFactory: s.LLMFactory(
			"openai",
		),

		ToolRegistry: toolRegistry,
	}
}

// ------------------------------------------------------------
// OpenCode coder
// ------------------------------------------------------------

// OpenCodeCoder returns a worker that drives the OpenCode CLI to
// implement a change inside the workspace. The existing LoopAgent
// coder (Coder) remains available alongside it.
func (s *Shared) OpenCodeCoder(
	prompt string,
) *opencode.Worker {

	return &opencode.Worker{
		AgentID: "opencode-coder",

		Prompt: prompt,

		Dir: s.workspace.Root,

		Model: s.store.Get(
			"opencode",
			"model",
		),

		ServerURL: s.store.Get(
			"opencode",
			"serverUrl",
		),

		QuestionHub: s.QuestionHub,
	}
}

// OpenCodePlanner returns a read-only OpenCode worker that produces
// a plan and stores it under state["plan"], which the coder's prompt
// then consumes. The model defaults to the opencode provider model
// but can be overridden with the plannerModel setting.
func (s *Shared) OpenCodePlanner(
	prompt string,
) *opencode.Worker {

	return &opencode.Worker{
		AgentID: "opencode-planner",

		Prompt: prompt,

		Dir: s.workspace.Root,

		Model: s.store.Get(
			"opencode",
			"plannerModel",
		),

		ReadOnly: true,

		OutputKey: "plan",

		ServerURL: s.store.Get(
			"opencode",
			"serverUrl",
		),

		QuestionHub: s.QuestionHub,
	}
}

// OpenCodeReviewer returns a read-only OpenCode worker that reviews
// the implemented change and ends with a strict VERDICT line. The
// model defaults to the opencode provider model but can be
// overridden with the reviewerModel setting.
func (s *Shared) OpenCodeReviewer(
	prompt string,
) *opencode.Worker {

	return &opencode.Worker{
		AgentID: "opencode-reviewer",

		Prompt: prompt,

		Dir: s.workspace.Root,

		Model: s.store.Get(
			"opencode",
			"reviewerModel",
		),

		ReadOnly: true,

		ServerURL: s.store.Get(
			"opencode",
			"serverUrl",
		),

		QuestionHub: s.QuestionHub,
	}
}

// ------------------------------------------------------------
// Prose agent
// ------------------------------------------------------------

// ProseAgent returns a LoopAgent that can read and write workspace
// files but has no shell or network access. Used for planning,
// writing, editing, and synthesis steps.
func (s *Shared) ProseAgent(
	agentID string,
	prompt string,
) *agent.LoopAgent {

	return &agent.LoopAgent{
		AgentID: agentID,

		Prompt: prompt,

		LLMFactory: s.LLMFactory(
			"openai",
		),

		ToolRegistry: proseTools(s),
	}
}

// ------------------------------------------------------------
// Research agent (web search)
// ------------------------------------------------------------

// ResearchAgent returns a LoopAgent that can search the web via the
// OpenAI Responses API web_search tool and write findings to files.
func (s *Shared) ResearchAgent(
	agentID string,
	prompt string,
) *agent.LoopAgent {

	return &agent.LoopAgent{
		AgentID: agentID,

		Prompt: prompt,

		LLMFactory: func() agent.LLM {

			provider :=
				llm.NewOpenAI(
					s.store.Get(
						"openai",
						"apiKey",
					),
					s.store.Get(
						"openai",
						"model",
					),
				)

			provider.WebSearch = true

			return provider
		},

		ToolRegistry: proseTools(s),
	}
}

func proseTools(
	s *Shared,
) *agent.ToolRegistry {

	return agent.NewToolRegistry(
		tools.ListFiles{
			Workspace: s.workspace,
		},

		tools.ReadFile{
			Workspace: s.workspace,
		},

		tools.WriteFile{
			Workspace: s.workspace,
		},
	)
}

// ------------------------------------------------------------
// Skippable agent
// ------------------------------------------------------------

// skippableAgent wraps an agent so its work is skipped (without
// invoking the LLM) when a runtime state key is empty. It lets
// workflows keep fixed parallel slots (e.g. three researchers)
// while only doing real work for the slots that were filled.
type skippableAgent struct {
	agent *agent.LoopAgent

	stateKey string
}

func (w *skippableAgent) ID() string {
	return w.agent.AgentID
}

func (w *skippableAgent) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	value, _ :=
		input.State[w.stateKey].(string)

	if strings.TrimSpace(value) == "" {
		return graph.WorkerResult{
			State: graph.State{
				"skipped": true,
			},
		}, nil
	}

	return w.agent.Run(
		ctx,
		input,
	)
}

// skipWhenEmpty returns a worker that runs agent only when the
// state value at stateKey is non-empty, skipping it otherwise.
func skipWhenEmpty(
	agent *agent.LoopAgent,
	stateKey string,
) graph.Worker {

	return &skippableAgent{
		agent:    agent,
		stateKey: stateKey,
	}
}

// ------------------------------------------------------------
// Report writer
// ------------------------------------------------------------

// WriteReport returns a function worker that saves the markdown in
// state[stateKey] to workspace/reports/<runID>/<name>.md and records
// the saved path in state.
func (s *Shared) WriteReport(
	name string,
	stateKey string,
) *graph.FuncWorker {

	return graph.NewFuncWorker(
		"write_report",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			content, _ :=
				state[stateKey].(string)

			if strings.TrimSpace(content) == "" {
				return nil, fmt.Errorf(
					"write_report: %q is empty",
					stateKey,
				)
			}

			if !strings.HasSuffix(
				name,
				".md",
			) {

				name += ".md"
			}

			executionContext, ok :=
				graph.GetExecutionContext(
					ctx,
				)

			if !ok {
				return nil, fmt.Errorf(
					"write_report: missing execution context",
				)
			}

			relative :=
				filepath.Join(
					"reports",
					executionContext.RunID,
					name,
				)

			resolved, err :=
				s.base.Resolve(
					relative,
				)

			if err != nil {
				return nil, err
			}

			if err :=
				os.MkdirAll(
					filepath.Dir(resolved),
					0o755,
				); err != nil {

				return nil, err
			}

			if err :=
				os.WriteFile(
					resolved,
					[]byte(content),
					0o644,
				); err != nil {

				return nil, err
			}

			return graph.State{
				"report_path": relative,

				"report_name": name,
			}, nil
		},
	)
}

// ------------------------------------------------------------
// Tester
// ------------------------------------------------------------

// summarizeTestOutput counts the per-test PASS/FAIL lines in a
// `go test -v` run and returns a one-line summary, or "" when no
// tests ran.
func summarizeTestOutput(
	output string,
) string {

	passed :=
		strings.Count(
			output,
			"--- PASS:",
		)

	failed :=
		strings.Count(
			output,
			"--- FAIL:",
		)

	if passed+failed == 0 {
		return ""
	}

	return fmt.Sprintf(
		"[summary] %d tests: %d passed, %d failed",
		passed+failed,
		passed,
		failed,
	)
}

// testCommand auto-detects the project test command by checking the
// workspace root and its immediate parent for go.mod or package.json:
// go.mod selects "go test ./...", package.json selects "npm test".
// It returns the program, its arguments, and the directory the
// command should run in. Go tests always run from the workspace root
// so only the workspace package is tested, never the parent module's
// own suite. Returns empty program when no project test command is
// detected.
func (s *Shared) testCommand() (string, []string, string) {

	root :=
		s.workspace.Root

	dirs :=
		[]string{root}

	if parent :=
		filepath.Dir(root); parent != root {

		dirs =
			append(
				dirs,
				parent,
			)
	}

	for _, dir := range dirs {

		if _, err :=
			os.Stat(
				filepath.Join(dir, "go.mod"),
			); err == nil {

			return "go", []string{
				"test",
				"-v",
				"./...",
			}, root
		}
	}

	for _, dir := range dirs {

		if _, err :=
			os.Stat(
				filepath.Join(dir, "package.json"),
			); err == nil {

			return "npm", []string{"test"}, dir
		}
	}

	return "", nil, ""
}

// Tester returns a deterministic worker that runs the project's
// auto-detected test command in the workspace and reports whether
// it passed from the process exit code.
func (s *Shared) Tester() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"tester",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			attempt := 0

			if value, ok :=
				state["test_attempts"]; ok {

				attempt =
					value.(int)
			}

			attempt++

			program, args, dir :=
				s.testCommand()

			if program == "" {

				fmt.Println(
					"[tester] No test command detected, marking as passed",
				)

				return graph.State{
					"tests_passed": true,

					"test_attempts": attempt,

					"test_output": "No test command detected in the workspace.",
				}, nil
			}

			fmt.Printf(
				"[tester] Running %s %v (attempt %d)...\n",
				program,
				args,
				attempt,
			)

			cmd :=
				exec.CommandContext(
					ctx,
					program,
					args...,
				)

			cmd.Dir =
				dir

			output, err :=
				cmd.CombinedOutput()

			passed :=
				err == nil

			text :=
				string(output)

			summary :=
				summarizeTestOutput(text)

			if summary != "" {
				text =
					summary + "\n" + text
			}

			if passed {

				fmt.Println(
					"[tester] PASS",
				)

			} else {

				fmt.Println(
					"[tester] FAIL",
				)
			}

			return graph.State{
				"tests_passed": passed,

				"test_attempts": attempt,

				"test_output": text,
			}, nil
		},
	)
}

// ------------------------------------------------------------
// Reviewer gate
// ------------------------------------------------------------

// ReviewGate returns a worker that parses the reviewer's verdict
// from its text output into state: approved (bool), review_feedback
// (string), and an incremented review_attempts counter.
func (s *Shared) ReviewGate() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"review_gate",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			output, _ :=
				state["output"].(string)

			attempt := 0

			if value, ok :=
				state["review_attempts"]; ok {

				attempt =
					value.(int)
			}

			attempt++

			approved :=
				strings.Contains(
					strings.ToUpper(output),
					"VERDICT: APPROVED",
				)

			fmt.Printf(
				"[review_gate] Attempt %d: approved=%v\n",
				attempt,
				approved,
			)

			return graph.State{
				"approved": approved,

				"review_feedback": output,

				"review_attempts": attempt,
			}, nil
		},
	)
}

// OpenCodePlanRevision resumes the planner's existing OpenCode session
// with the user's requested plan changes, so the agent keeps its full
// context while producing a revised plan.
func (s *Shared) OpenCodePlanRevision() *opencode.Worker {

	return &opencode.Worker{
		AgentID: "opencode-planner",

		Prompt: planRevisionPrompt,

		Dir: s.workspace.Root,

		Model: s.store.Get(
			"opencode",
			"plannerModel",
		),

		ReadOnly: true,

		OutputKey: "plan",

		Resume: true,

		ServerURL: s.store.Get(
			"opencode",
			"serverUrl",
		),

		QuestionHub: s.QuestionHub,
	}
}

// PlanGate asks the user to approve the plan (or request changes)
// before the coder runs. Change requests are fed back to the planner
// via state["plan_feedback"] for a bounded number of rounds.
func (s *Shared) PlanGate() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"plan_gate",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			plan, _ :=
				state["plan"].(string)

			attempt := 0

			if value, ok :=
				state["plan_attempts"]; ok {

				attempt =
					value.(int)
			}

			attempt++

			executionContext, ok :=
				graph.GetExecutionContext(ctx)

			if !ok {
				return nil, fmt.Errorf(
					"plan_gate: missing execution context",
				)
			}

			// Without a hub (headless) auto-approve so the run
			// is not blocked waiting for a user.
			approved :=
				s.QuestionHub == nil

			feedback := ""

			if s.QuestionHub != nil {

				answers, answered :=
					s.askUser(
						ctx,
						executionContext.RunID,
						fmt.Sprintf("plan_gate_approve_%d", attempt),
						[]opencode.QuestionInfo{
							{
								Question: plan,

								Header: "Plan review",

								Options: []opencode.QuestionOption{
									{Label: "Approve"},

									{Label: "Request changes"},
								},
							},
						},
					)

				if !answered {
					return nil, fmt.Errorf(
						"plan_gate: no answer for plan approval",
					)
				}

				if len(answers) > 0 &&
					len(answers[0]) > 0 &&
					answers[0][0] == "Approve" {

					approved = true
				} else {

					text, got :=
						s.askUser(
							ctx,
							executionContext.RunID,
							fmt.Sprintf("plan_gate_changes_%d", attempt),
							[]opencode.QuestionInfo{
								{
									Question: "Describe the changes you want to make to the plan:",

									Header: "Changes",

									Custom: true,
								},
							},
						)

					if !got {
						return nil, fmt.Errorf(
							"plan_gate: no changes provided",
						)
					}

					if len(text) > 0 &&
						len(text[0]) > 0 {

						feedback =
							strings.TrimSpace(
								text[0][0],
							)
					}

					// No changes specified -> treat as approval.
					approved =
						feedback == ""
				}
			}

			fmt.Printf(
				"[plan_gate] Attempt %d: approved=%v\n",
				attempt,
				approved,
			)

			return graph.State{
				"plan_approved": approved,

				"plan_feedback": feedback,

				"plan_attempts": attempt,
			}, nil
		},
	)
}

// askUser surfaces a question through the run UI and blocks until the
// user answers via the HTTP answer endpoint. Returns the selected
// labels (one []string per question) or ok=false if no answer arrived.
func (s *Shared) askUser(
	ctx context.Context,
	runID string,
	requestID string,
	questions []opencode.QuestionInfo,
) ([][]string, bool) {

	if s.QuestionHub == nil {
		return nil, false
	}

	ch, cleanup :=
		s.QuestionHub.Register(
			runID,
			requestID,
		)

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentQuestion,

			AgentID: "harnais",

			Data: map[string]any{
				"requestId": requestID,

				"questions": questions,
			},
		},
	)

	defer cleanup()

	select {

	case <-ctx.Done():
		return nil, false

	case got, ok := <-ch:
		if !ok {
			return nil, false
		}

		// Emit the answer so a replayed run-history removes the
		// question card (mirrors the opencode worker's handleQuestion).
		graph.EmitEvent(
			ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventAgentQuestionAnswer,

				AgentID: "harnais",

				Data: map[string]any{
					"requestId": requestID,

					"answers": got,
				},
			},
		)

		return got, true
	}
}

// ------------------------------------------------------------
// Graph helpers
// ------------------------------------------------------------

func addNode(
	g *graph.Graph,
	node *graph.Node,
) {

	if err :=
		g.AddNode(node); err != nil {

		panic(err)
	}
}

func addEdge(
	g *graph.Graph,
	from string,
	to string,
) {

	if err :=
		g.AddEdge(from, to); err != nil {

		panic(err)
	}
}

func addConditionalEdge(
	g *graph.Graph,
	from string,
	to string,
	condition func(graph.State) bool,
) {

	if err :=
		g.AddConditionalEdge(
			from,
			to,
			condition,
		); err != nil {

		panic(err)
	}
}
