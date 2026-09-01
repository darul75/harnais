# harnais

A minimal event-driven agent graph executor in Go, with a React UI for live run monitoring.

## Overview

`harnais` (French for "harness") runs a directed graph of nodes connected by edges (including conditional loops). Each node wraps a **Worker** — a pluggable execution unit that can be a simple function, an LLM-powered agent with tool use, or any custom implementation. Node executions emit lifecycle events streamed to the browser via Server-Sent Events.

## Structure

- `graph/` — core primitives: `Worker` interface, `Graph`, `Executor`, `Run`, events
- `agent/` — `Agent` interface, `LoopAgent` (LLM loop with tool calls), `Tool` interface
- `server/` — HTTP API, event bus, SSE stream, and run manager
- `main.go` — example graph wiring (planner, coder agent, security agent, tester, reviewer) and server bootstrap
- `ui/` — React + Vite dashboard

## Core concepts

### Worker

The `Worker` interface (`graph/types.go`) is the execution unit for every node:

```go
type Worker interface {
    ID() string
    Run(ctx context.Context, input WorkerInput) (WorkerResult, error)
}
```

`FuncWorker` (`graph/worker.go`) adapts a plain function into a `Worker` for simple nodes.

### Agent

The `agent` package provides higher-level abstractions on top of `Worker`:

- **`Agent`** — interface for agents that take a message + state and return output + state
- **`LoopAgent`** — an LLM-powered agent that runs a generate → tool-call → feedback loop until the LLM returns a final answer
- **`Tool`** — interface for tools an agent can call (e.g. `read_file`, `edit_file`, `run_tests`)

`LoopAgent` implements `Worker`, so it plugs directly into graph nodes.

### Executor

The `Executor` (`graph/executor.go`) runs the graph:

- **`Start`** — launches execution asynchronously, returns a `*Run` immediately
- **`Run`** — executes synchronously and blocks until completion

Each wave of ready nodes is executed concurrently. After a wave completes, outputs are merged into the shared `State` and outgoing edges are evaluated.

### Events

The executor emits lifecycle events at every stage:

| Category | Events |
|----------|--------|
| Run      | `run.started`, `run.completed`, `run.failed` |
| Node     | `node.started`, `node.completed`, `node.failed` |
| Worker   | `worker.started`, `worker.completed`, `worker.failed` |
| Agent    | `agent.started`, `agent.completed` |
| LLM      | `llm.started`, `llm.completed` |
| Tool     | `tool.started`, `tool.completed`, `tool.failed` |
| Edge     | `edge.activated` |

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST   | `/api/runs` | Create and start a new run |
| GET    | `/api/runs/{runID}` | Get run status |
| GET    | `/api/runs/{runID}/graph` | Get graph structure (nodes + edges) |
| GET    | `/api/runs/{runID}/state` | Get current workflow state |
| GET    | `/api/runs/{runID}/executions` | Get all node executions |
| GET    | `/api/runs/{runID}/events` | SSE stream of run events |

## Running

Backend:

```sh
go run .
```

Serves the API on `http://localhost:8080`.

Frontend:

```sh
cd ui
npm install
npm run dev
```

Serves the UI on `http://localhost:5173`.
