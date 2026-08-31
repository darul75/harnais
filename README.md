# harnais

A minimal event-driven agent graph executor in Go, with a React UI for live run monitoring.

## Overview

`harnais` (French for "harness") runs a graph of nodes — planner, coder, tester, reviewer — connected by edges, including conditional loops for retrying until tests pass. Node executions emit events that are streamed to the browser via Server-Sent Events.

## Structure

- `graph/` — graph definition and executor (nodes, edges, runs, events)
- `server/` — HTTP API, event bus, and run manager
- `main.go` — example graph wiring and server bootstrap
- `ui/` — React + Vite dashboard

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
