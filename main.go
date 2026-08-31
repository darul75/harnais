package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

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

	graphDefinition :=
		buildGraph()

	// ============================================================
	// Executor
	// ============================================================

	executor :=
		graph.NewExecutor(

			// --------------------------------------------
			// Event handler
			// --------------------------------------------

			func(event graph.Event) {

				printEvent(event)

				eventBus.Publish(
					event,
				)
			},

			// --------------------------------------------
			// Run created
			// --------------------------------------------

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
	// API server
	// ============================================================

	api := server.NewServer(
		eventBus,
		runManager,

		func(initialState graph.State) *graph.Run {

			return executor.Start(
				context.Background(),
				graphDefinition,
				initialState,
			)
		},
	)

	// ============================================================
	// HTTP
	// ============================================================

	fmt.Println()
	fmt.Println(
		"API listening on http://localhost:8080",
	)

	fmt.Println(
		"React UI expected on http://localhost:5173",
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

	must(
		g.AddNode(
			&graph.Node{
				ID: "planner",

				Execute: func(
					ctx context.Context,
					state graph.State,
				) (graph.State, error) {

					fmt.Println(
						"[planner] Creating plan...",
					)

					time.Sleep(
						2 * time.Second,
					)

					return graph.State{
						"plan": "Fix authentication bug",
					}, nil
				},
			},
		),
	)

	// ------------------------------------------------------------
	// Coder
	// ------------------------------------------------------------

	must(
		g.AddNode(
			&graph.Node{
				ID: "coder",

				Execute: func(
					ctx context.Context,
					state graph.State,
				) (graph.State, error) {

					attempt := 0

					if value, ok :=
						state["coder_attempts"]; ok {

						attempt =
							value.(int)
					}

					attempt++

					fmt.Printf(
						"[coder] Implementing (attempt %d)...\n",
						attempt,
					)

					time.Sleep(
						2 * time.Second,
					)

					return graph.State{
						"code_changed":   true,
						"coder_attempts": attempt,
					}, nil
				},
			},
		),
	)

	// ------------------------------------------------------------
	// Tester
	// ------------------------------------------------------------

	must(
		g.AddNode(
			&graph.Node{
				ID: "tester",

				Execute: func(
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
						"[tester] Running tests (attempt %d)...\n",
						attempt,
					)

					time.Sleep(
						2 * time.Second,
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
						"tests_passed":  passed,
						"test_attempts": attempt,
					}, nil
				},
			},
		),
	)

	// ------------------------------------------------------------
	// Reviewer
	// ------------------------------------------------------------

	must(
		g.AddNode(
			&graph.Node{
				ID: "reviewer",

				Execute: func(
					ctx context.Context,
					state graph.State,
				) (graph.State, error) {

					fmt.Println(
						"[reviewer] Reviewing...",
					)

					time.Sleep(
						2 * time.Second,
					)

					fmt.Println(
						"[reviewer] APPROVED",
					)

					return graph.State{
						"approved": true,
					}, nil
				},
			},
		),
	)

	// ============================================================
	// Edges
	// ============================================================

	must(
		g.AddEdge(
			"planner",
			"coder",
		),
	)

	must(
		g.AddEdge(
			"coder",
			"tester",
		),
	)

	// tester -> coder if failed

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

	// tester -> reviewer if passed

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

	return g
}

// ============================================================
// Events
// ============================================================

func printEvent(
	event graph.Event,
) {

	switch event.Type {

	case graph.EventRunStarted:

		fmt.Printf(
			"[EVENT] RUN STARTED     %s\n",
			event.RunID,
		)

	case graph.EventNodeStarted:

		fmt.Printf(
			"[EVENT] NODE STARTED    %-10s\n",
			event.NodeID,
		)

	case graph.EventNodeCompleted:

		fmt.Printf(
			"[EVENT] NODE COMPLETED  %-10s\n",
			event.NodeID,
		)

	case graph.EventNodeFailed:

		fmt.Printf(
			"[EVENT] NODE FAILED     %-10s %s\n",
			event.NodeID,
			event.Message,
		)

	case graph.EventEdgeActivated:

		fmt.Printf(
			"[EVENT] EDGE ACTIVATED  %-10s %s\n",
			event.NodeID,
			event.Message,
		)

	case graph.EventRunCompleted:

		fmt.Printf(
			"[EVENT] RUN COMPLETED   %s\n",
			event.RunID,
		)

	case graph.EventRunFailed:

		fmt.Printf(
			"[EVENT] RUN FAILED      %s\n",
			event.Message,
		)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
