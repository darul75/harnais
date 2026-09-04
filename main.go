package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"harnais/config"
	"harnais/graph"
	"harnais/server"
	"harnais/store"
	"harnais/tools"
	"harnais/workflows"
)

func main() {

	// ============================================================
	// Infrastructure
	// ============================================================

	eventBus :=
		server.NewEventBus()

	dbPath := os.Getenv("HARNAIS_DB")
	if dbPath == "" {
		dbPath = "./harnais.db"
	}

	runStore, err := store.NewRunStore(dbPath)
	if err != nil {
		log.Fatalf("init run store: %v", err)
	}
	defer runStore.Close()

	runManager :=
		server.NewRunManager(runStore)

	// Load persisted events into EventBus for SSE replay
	runs, err := runStore.ListRuns()
	if err != nil {
		log.Printf("list runs: %v", err)
	}
	for _, r := range runs {
		events, err := runStore.GetEvents(r.RunID)
		if err != nil {
			log.Printf("load events for %s: %v", r.RunID, err)
			continue
		}
		eventBus.LoadHistory(r.RunID, events)
	}
	if len(runs) > 0 {
		fmt.Printf("[PERSIST] loaded events for %d runs\n", len(runs))
	}

	// ============================================================
	// Settings
	// ============================================================

	settingsPath :=
		os.Getenv("HARNAIS_SETTINGS")

	settings :=
		config.NewStore(
			settingsPath,
		)

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

	questionHub :=
		graph.NewQuestionHub()

	registry, err :=
		workflows.Register(
			workspace,
			settings,
			questionHub,
		)

	if err != nil {
		log.Fatal(err)
	}

	selector :=
		workflows.NewSelector(
			registry,
			settings.LLMFactory(
				"openai",
			),
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

				// Persist event to store
				if err := runManager.Store().AddEvent(&event); err != nil {
					fmt.Printf("[PERSIST] failed to persist event: %v\n", err)
				}

				// Update run status and persist snapshot on completion/failure
				switch event.Type {
				case graph.EventRunCompleted:
					now := time.Now()
					runManager.UpdateStatus(event.RunID, graph.StatusCompleted, &now)
					if err := runManager.PersistRunSnapshot(event.RunID); err != nil {
						fmt.Printf("[PERSIST] failed to persist run %s: %v\n", event.RunID, err)
					} else {
						fmt.Printf("[PERSIST] run %s snapshot persisted\n", event.RunID)
					}

				case graph.EventRunFailed:
					now := time.Now()
					runManager.UpdateStatus(event.RunID, graph.StatusFailed, &now)
					if err := runManager.PersistRunSnapshot(event.RunID); err != nil {
						fmt.Printf("[PERSIST] failed to persist run %s: %v\n", event.RunID, err)
					} else {
						fmt.Printf("[PERSIST] run %s snapshot persisted\n", event.RunID)
					}
				}
			},

			// ------------------------------------------------
			// Run registration happens in the StartRun
			// closure, where task + workflow metadata is
			// known.
			// ------------------------------------------------

			nil,
		)

	// ============================================================
	// HTTP API
	// ============================================================

	api :=
		server.NewServer(

			eventBus,

			runManager,

			settings,

			workspace,

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

				runID := fmt.Sprintf(
					"run-%d",
					time.Now().UnixNano(),
				)

				runWorkspace :=
					workspace

				if workflow.Isolated {

					// Each coding run works in an isolated directory
					// seeded from the base workspace, so agents only
					// see this run's content.
					runDir :=
						filepath.Join(
							workspace.Root,
							"coding",
							"runs",
							runID,
						)

					if err :=
						tools.SeedRunDir(
							workspace.Root,
							runDir,
						); err != nil {

						fmt.Println(
							"[workflow] seed run workspace:",
							err,
						)
					}

					runWorkspace =
						tools.NewWorkspace(
							runDir,
						)
				}

				g :=
					workflow.Build(
						runWorkspace,
					)

				initial :=
					graph.State{
						"task": request.Task,
					}

				if request.PDFPath != "" {
					initial["pdf_path"] =
						request.PDFPath
				}

				meta := server.RunMeta{
					Task:       request.Task,
					WorkflowID: workflow.ID,
				}

				runManager.CreateRun(runID, meta, time.Now())

				run :=
					executor.StartWithID(
						context.Background(),
						g,
						initial,
						runID,
					)

				runManager.Add(
					run,
					meta,
				)

				fmt.Println()
				fmt.Println(
					"Run registered:",
					run.ID,
				)

				return run
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

			func(id string) (*server.WorkflowDetail, bool) {

				workflow, ok :=
					registry.Get(id)

				if !ok {
					return nil, false
				}

				graphInfo :=
					workflows.Describe(
						workflow.Build(
							workspace,
						),
					)

				nodes := make(
					[]server.WorkflowNodeInfo,
					0,
					len(graphInfo.Nodes),
				)

				for _, node := range graphInfo.Nodes {

					nodes =
						append(
							nodes,
							server.WorkflowNodeInfo{
								ID:      node.ID,
								Kind:    string(node.Kind),
								AgentID: node.AgentID,
								Prompt:  node.Prompt,
								Tools:   node.Tools,
								JoinAll: node.JoinAll,
							},
						)
				}

				edges := make(
					[]server.WorkflowEdgeInfo,
					0,
					len(graphInfo.Edges),
				)

				for _, edge := range graphInfo.Edges {

					edges =
						append(
							edges,
							server.WorkflowEdgeInfo{
								ID:          edge.ID,
								From:        edge.From,
								To:          edge.To,
								Conditional: edge.Conditional,
							},
						)
				}

				return &server.WorkflowDetail{
					ID:          workflow.ID,
					Title:       workflow.Title,
					Description: workflow.Description,
					Nodes:       nodes,
					Edges:       edges,
				}, true
			},

			questionHub,
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
