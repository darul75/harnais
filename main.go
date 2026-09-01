package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"harnais/agent"
	"harnais/graph"
	"harnais/server"
)

func main() {

	// ============================================================
	// Infrastructure
	// ============================================================

	eventBus :=
		server.NewEventBus()

	runManager :=
		server.NewRunManager()

	// ============================================================
	// Graph
	// ============================================================

	g :=
		buildGraph()

	// ============================================================
	// Executor
	// ============================================================

	executor :=
		graph.NewExecutor(

			// ------------------------------------------------
			// THE SINGLE EVENT PIPELINE
			// ------------------------------------------------

			func(event graph.Event) {

				printEvent(event)

				eventBus.Publish(
					event,
				)
			},

			// ------------------------------------------------
			// Register every new run immediately.
			// ------------------------------------------------

			func(run *graph.Run) {

				runManager.Add(
					run,
				)

				fmt.Println()
				fmt.Println(
					"Run registered:",
					run.ID,
				)
			},
		)

	// ============================================================
	// HTTP API
	// ============================================================

	api :=
		server.NewServer(

			eventBus,

			runManager,

			func(initial graph.State) *graph.Run {

				return executor.Start(
					context.Background(),
					g,
					initial,
				)
			},
		)

	// ============================================================
	// HTTP server
	// ============================================================

	fmt.Println()
	fmt.Println(
		"========================================",
	)

	fmt.Println(
		"        GO CODING HARNESS",
	)

	fmt.Println(
		"========================================",
	)

	fmt.Println()
	fmt.Println(
		"API: http://localhost:8080",
	)

	fmt.Println(
		"UI:  http://localhost:5173",
	)

	fmt.Println()

	if err :=
		http.ListenAndServe(
			":8080",
			api.Handler(),
		); err != nil {

		log.Fatal(err)
	}
}

// ============================================================
// Graph
// ============================================================

func buildGraph() *graph.Graph {

	g :=
		graph.NewGraph()

	// ------------------------------------------------------------
	// Planner
	// ------------------------------------------------------------

	planner :=
		graph.NewFuncWorker(
			"planner",
			func(
				ctx context.Context,
				state graph.State,
			) (graph.State, error) {

				fmt.Println(
					"[planner] Creating plan...",
				)

				time.Sleep(
					1 * time.Second,
				)

				return graph.State{
					"plan": "Fix authentication bug",
				}, nil
			},
		)

	must(
		g.AddNode(
			&graph.Node{
				ID:     "planner",
				Worker: planner,
			},
		),
	)

	// ------------------------------------------------------------
	// Coder agent
	// ------------------------------------------------------------

	coder :=
		&agent.LoopAgent{

			AgentID: "coder-agent",

			Prompt: "Implement the authentication fix.",

			LLM: &FakeLLM{
				Name: "coder-llm",
			},

			Tools: map[string]agent.Tool{

				"read_file": ReadFileTool{},

				"edit_file": EditFileTool{},

				"run_tests": RunTestsTool{},
			},
		}

	must(
		g.AddNode(
			&graph.Node{
				ID:     "coder",
				Worker: coder,
			},
		),
	)

	// ------------------------------------------------------------
	// Security agent
	// ------------------------------------------------------------

	security :=
		&agent.LoopAgent{

			AgentID: "security-agent",

			Prompt: "Check the authentication implementation for security issues.",

			LLM: &FakeLLM{
				Name: "security-llm",
			},

			Tools: map[string]agent.Tool{

				"read_file": ReadFileTool{},
			},
		}

	must(
		g.AddNode(
			&graph.Node{
				ID:     "security",
				Worker: security,
			},
		),
	)

	// ------------------------------------------------------------
	// Tester
	// ------------------------------------------------------------

	tester :=
		graph.NewFuncWorker(
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

	must(
		g.AddNode(
			&graph.Node{
				ID:     "tester",
				Worker: tester,
			},
		),
	)

	// ------------------------------------------------------------
	// Reviewer
	// ------------------------------------------------------------

	reviewer :=
		graph.NewFuncWorker(
			"reviewer",
			func(
				ctx context.Context,
				state graph.State,
			) (graph.State, error) {

				fmt.Println(
					"[reviewer] Reviewing...",
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

	must(
		g.AddNode(
			&graph.Node{
				ID:     "reviewer",
				Worker: reviewer,
			},
		),
	)

	// ============================================================
	// Edges
	// ============================================================

	// planner -> coder
	must(
		g.AddEdge(
			"planner",
			"coder",
		),
	)

	// planner -> security
	//
	// Parallel branch.
	must(
		g.AddEdge(
			"planner",
			"security",
		),
	)

	// coder -> tester
	must(
		g.AddEdge(
			"coder",
			"tester",
		),
	)

	// tester -> coder when failed
	must(
		g.AddConditionalEdge(
			"tester",
			"coder",
			func(state graph.State) bool {

				passed, _ :=
					state["tests_passed"].(bool)

				return !passed
			},
		),
	)

	// tester -> reviewer when passed
	must(
		g.AddConditionalEdge(
			"tester",
			"reviewer",
			func(state graph.State) bool {

				passed, _ :=
					state["tests_passed"].(bool)

				return passed
			},
		),
	)

	// security -> reviewer
	must(
		g.AddEdge(
			"security",
			"reviewer",
		),
	)

	return g
}

// ============================================================
// Fake LLM
// ============================================================

type FakeLLM struct {
	Name string

	Called int
}

func (l *FakeLLM) Generate(
	ctx context.Context,
	messages []agent.Message,
) (agent.LLMResponse, error) {

	l.Called++

	if l.Name == "coder-llm" {

		switch l.Called {

		case 1:

			return agent.LLMResponse{
				ToolCall: &agent.ToolCall{
					Name: "read_file",

					Input: map[string]any{
						"path": "auth.go",
					},
				},
			}, nil

		case 2:

			return agent.LLMResponse{
				ToolCall: &agent.ToolCall{
					Name: "edit_file",

					Input: map[string]any{
						"path": "auth.go",
					},
				},
			}, nil

		case 3:

			return agent.LLMResponse{
				ToolCall: &agent.ToolCall{
					Name: "run_tests",
				},
			}, nil

		default:

			return agent.LLMResponse{
				Text: "Authentication fix implemented.",
			}, nil
		}
	}

	// Security agent.

	if l.Called == 1 {

		return agent.LLMResponse{
			ToolCall: &agent.ToolCall{
				Name: "read_file",

				Input: map[string]any{
					"path": "auth.go",
				},
			},
		}, nil
	}

	return agent.LLMResponse{
		Text: "No security issues found.",
	}, nil
}

// ============================================================
// Fake tools
// ============================================================

type ReadFileTool struct{}

func (ReadFileTool) ID() string {
	return "read_file"
}

func (ReadFileTool) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	time.Sleep(
		500 * time.Millisecond,
	)

	return map[string]any{
		"content": "func authenticate() { /* existing code */ }",
	}, nil
}

type EditFileTool struct{}

func (EditFileTool) ID() string {
	return "edit_file"
}

func (EditFileTool) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	time.Sleep(
		700 * time.Millisecond,
	)

	return map[string]any{
		"changed": true,

		"path": input["path"],
	}, nil
}

type RunTestsTool struct{}

func (RunTestsTool) ID() string {
	return "run_tests"
}

func (RunTestsTool) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	time.Sleep(
		800 * time.Millisecond,
	)

	return map[string]any{
		"passed": true,
	}, nil
}

// ============================================================
// Console event output
// ============================================================

func printEvent(
	event graph.Event,
) {

	extra := ""

	if event.AgentID != "" {
		extra +=
			" agent=" +
				event.AgentID
	}

	if event.WorkerID != "" {
		extra +=
			" worker=" +
				event.WorkerID
	}

	if event.ToolID != "" {
		extra +=
			" tool=" +
				event.ToolID
	}

	switch event.Type {

	case graph.EventRunStarted:

		fmt.Printf(
			"[EVENT] RUN STARTED %s\n",
			event.RunID,
		)

	case graph.EventNodeStarted:

		fmt.Printf(
			"[EVENT] NODE STARTED %s%s\n",
			event.NodeID,
			extra,
		)

	case graph.EventNodeCompleted:

		fmt.Printf(
			"[EVENT] NODE COMPLETED %s\n",
			event.NodeID,
		)

	case graph.EventNodeFailed:

		fmt.Printf(
			"[EVENT] NODE FAILED %s %s\n",
			event.NodeID,
			event.Message,
		)

	case graph.EventEdgeActivated:

		fmt.Printf(
			"[EVENT] EDGE %s\n",
			event.Message,
		)

	case graph.EventWorkerStarted:

		fmt.Printf(
			"[EVENT] WORKER STARTED %s\n",
			event.WorkerID,
		)

	case graph.EventWorkerCompleted:

		fmt.Printf(
			"[EVENT] WORKER COMPLETED %s\n",
			event.WorkerID,
		)

	case graph.EventAgentStarted:

		fmt.Printf(
			"[EVENT] AGENT STARTED %s\n",
			event.AgentID,
		)

	case graph.EventAgentCompleted:

		fmt.Printf(
			"[EVENT] AGENT COMPLETED %s\n",
			event.AgentID,
		)

	case graph.EventLLMStarted:

		fmt.Printf(
			"[EVENT] LLM STARTED%s\n",
			extra,
		)

	case graph.EventLLMCompleted:

		fmt.Printf(
			"[EVENT] LLM COMPLETED%s\n",
			extra,
		)

	case graph.EventToolStarted:

		fmt.Printf(
			"[EVENT] TOOL STARTED %s\n",
			event.ToolID,
		)

	case graph.EventToolCompleted:

		fmt.Printf(
			"[EVENT] TOOL COMPLETED %s\n",
			event.ToolID,
		)

	case graph.EventToolFailed:

		fmt.Printf(
			"[EVENT] TOOL FAILED %s %s\n",
			event.ToolID,
			event.Message,
		)

	case graph.EventRunCompleted:

		fmt.Println(
			"[EVENT] RUN COMPLETED",
		)

	case graph.EventRunFailed:

		fmt.Println(
			"[EVENT] RUN FAILED",
			event.Message,
		)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
