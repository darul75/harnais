package workflows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"harnais/agent"
	"harnais/config"
	"harnais/graph"
	"harnais/llm"
	"harnais/opencode"
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
// Planner
// ------------------------------------------------------------

func (s *Shared) Planner() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"planner",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			task, ok :=
				state["task"].(string)

			if !ok || task == "" {
				return nil, fmt.Errorf(
					"planner: task is missing",
				)
			}

			fmt.Println(
				"[planner] Task:",
				task,
			)

			time.Sleep(
				1 * time.Second,
			)

			return graph.State{
				"plan": task,
			}, nil
		},
	)
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
		tools.ReadFile{
			Workspace: s.workspace,
		},

		tools.WriteFile{
			Workspace: s.workspace,
		},
	)
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

			fmt.Printf(
				"[tester] Running tests, attempt %d...\n",
				attempt,
			)

			time.Sleep(
				1 * time.Second,
			)

			passed :=
				attempt >= 2

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
			}, nil
		},
	)
}

// ------------------------------------------------------------
// Reviewer
// ------------------------------------------------------------

func (s *Shared) Reviewer() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"reviewer",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			fmt.Println(
				"[reviewer] Reviewing all results...",
			)

			time.Sleep(
				1 * time.Second,
			)

			fmt.Println(
				"[reviewer] APPROVED",
			)

			return graph.State{
				"approved": true,
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
