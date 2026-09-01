package workflows

import (
	"context"
	"fmt"
	"time"

	"harnais/agent"
	"harnais/config"
	"harnais/graph"
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
