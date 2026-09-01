package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"harnais/graph"
	"harnais/llm"
	"harnais/server"
	"harnais/tools"
	"harnais/workflows"
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
	// Workflows
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

	registry, err :=
		workflows.Register(
			workspace,
		)

	if err != nil {
		log.Fatal(err)
	}

	selector :=
		workflows.NewSelector(
			registry,
			llm.NewOpenAI("", ""),
		)

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

			func(request server.StartRunRequest) *graph.Run {

				workflow, err :=
					selector.Select(
						context.Background(),
						request.Task,
						request.WorkflowID,
					)

				if err != nil {
					workflow =
						registry.Default()
				}

				fmt.Println(
					"[workflow] selected:",
					workflow.ID,
					"-",
					workflow.Title,
				)

				g :=
					workflow.Build()

				return executor.Start(
					context.Background(),
					g,
					graph.State{
						"task": request.Task,
					},
				)
			},

			func() []server.WorkflowInfo {

				all :=
					registry.All()

				info := make(
					[]server.WorkflowInfo,
					0,
					len(all),
				)

				for _, workflow := range all {

					info =
						append(
							info,
							server.WorkflowInfo{
								ID: workflow.ID,

								Title: workflow.Title,

								Description: workflow.Description,
							},
						)
				}

				return info
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
