package workflows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"harnais/agent"
	"harnais/config"
	"harnais/graph"
	"harnais/llm"
	"harnais/workflows/opencode"
	"harnais/tools"
)

// Shared holds reusable node/worker builders so each workflow
// can compose distinct graphs from common components.
type Shared struct {
	workspace *tools.Workspace

	store *config.Store
}

func NewShared(
	workspace *tools.Workspace,
	store *config.Store,
) *Shared {

	return &Shared{
		workspace: workspace,
		store:     store,
	}
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
				s.workspace.Resolve(
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

// testCommands returns the auto-detected test command for the
// workspace root: go.mod selects "go test ./...", package.json
// selects the npm/pnpm/yarn test script. Returns nil when no
// project test command is detected.
func (s *Shared) testCommand() (string, []string) {

	root :=
		s.workspace.Root

	if _, err :=
		os.Stat(
			filepath.Join(root, "go.mod"),
		); err == nil {

		return "go", []string{
			"test",
			"./...",
		}
	}

	if _, err :=
		os.Stat(
			filepath.Join(root, "package.json"),
		); err == nil {

		return "npm", []string{"test"}
	}

	return "", nil
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

			program, args :=
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
				s.workspace.Root

			output, err :=
				cmd.CombinedOutput()

			passed :=
				err == nil

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

				"test_output": string(output),
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
