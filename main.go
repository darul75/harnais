package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"harnais/agent"
	"harnais/graph"
	"harnais/llm"
	"harnais/server"
	"harnais/tools"
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
			// Single event pipeline
			// ------------------------------------------------

			func(event graph.Event) {

				printEvent(event)

				eventBus.Publish(
					event,
				)
			},

			// ------------------------------------------------
			// Register run
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

	// ============================================================
	// Workspace
	// ============================================================

	workspaceRoot :=
		os.Getenv("HARNAIS_WORKSPACE")

	if workspaceRoot == "" {
		workspaceRoot =
			"./workspace"
	}

	workspace :=
		tools.NewWorkspace(
			workspaceRoot,
		)

	// ============================================================
	// Planner
	// ============================================================

	planner :=
		graph.NewFuncWorker(
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

	must(
		g.AddNode(
			&graph.Node{
				ID: "planner",

				Worker: planner,
			},
		),
	)

	// ============================================================
	// Coder agent
	// ============================================================

	coderTools :=
		agent.NewToolRegistry(

			tools.ListFiles{
				Workspace: workspace,
			},

			tools.ReadFile{
				Workspace: workspace,
			},

			tools.WriteFile{
				Workspace: workspace,
			},

			tools.RunCommand{
				Workspace: workspace,
			},

			tools.GitDiff{
				Workspace: workspace,
			},
		)

	coder :=
		&agent.LoopAgent{

			AgentID: "coder-agent",

			Prompt: `You are an autonomous coding agent working inside the provided workspace.

A user request and workflow state are provided at runtime.

Your job is to implement the user's requested change.

First inspect the workspace with list_files.
Do not assume a project type or specific files.
Read the relevant existing code before changing anything.

Implement only the requested change.
Run appropriate tests.
Inspect the final diff when available.

Work only inside the workspace.
Do not access the harness source code.
Do not fabricate tool results or test results.`,

			LLMFactory: func() agent.LLM {
				return llm.NewOpenAI("", "")
			},

			ToolRegistry: coderTools,
		}

	must(
		g.AddNode(
			&graph.Node{
				ID: "coder",

				Worker: coder,
			},
		),
	)

	// ============================================================
	// Security agent
	// ============================================================

	securityTools :=
		agent.NewToolRegistry(

			tools.ListFiles{
				Workspace: workspace,
			},

			tools.ReadFile{
				Workspace: workspace,
			},

			tools.GitDiff{
				Workspace: workspace,
			},
		)

	security :=
		&agent.LoopAgent{

			AgentID: "security-agent",

			Prompt: `You are a security review agent working inside the provided workspace.

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
Do not fabricate findings or tool results.`,

			LLMFactory: func() agent.LLM {

				return llm.NewOpenAI("", "")
			},

			ToolRegistry: securityTools,
		}

	must(
		g.AddNode(
			&graph.Node{
				ID: "security",

				Worker: security,
			},
		),
	)

	// ============================================================
	// Tester
	// ============================================================

	tester :=
		graph.NewFuncWorker(
			"tester",

			func(
				ctx context.Context,
				state graph.State,
			) (graph.State, error) {

				attempt :=
					0

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
				ID: "tester",

				Worker: tester,
			},
		),
	)

	// ============================================================
	// Reviewer
	// ============================================================

	reviewer :=
		graph.NewFuncWorker(
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

	must(
		g.AddNode(
			&graph.Node{
				ID: "reviewer",

				Worker: reviewer,

				JoinAll: true,
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

	// tester -> coder when tests fail
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

	// tester -> reviewer when tests pass
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
// Console events
// ============================================================

func printEvent(
	event graph.Event,
) {

	extra :=
		""

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
