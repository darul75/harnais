import {
  useEffect,
  useRef,
  useState,
} from "react";

import {
  createRun,
  getExecutions,
  getGraph,
  getRun,
  getState,
  subscribeToEvents,
} from "./api";

import type {
  GraphDefinition,
  GraphEvent,
  NodeExecution,
  Run,
  WorkflowState,
} from "./types";

import { GraphView } from "./GraphView";
import { EventList } from "./EventList";
import { ExecutionList } from "./ExecutionList";
import { StateView } from "./StateView";

export default function App() {
  const [run, setRun] = useState<Run | null>(null);

  const [graph, setGraph] =
    useState<GraphDefinition | null>(null);

  const [state, setState] =
    useState<WorkflowState>({});

  const [executions, setExecutions] =
    useState<NodeExecution[]>([]);

  const [events, setEvents] =
    useState<GraphEvent[]>([]);

  const [starting, setStarting] =
    useState(false);

  const [error, setError] =
    useState<string | null>(null);

  const eventSourceRef =
    useRef<EventSource | null>(null);

  // ------------------------------------------------------------
  // Close SSE connection
  // ------------------------------------------------------------

  function closeEvents() {
    eventSourceRef.current?.close();
    eventSourceRef.current = null;
  }

  // ------------------------------------------------------------
  // Load current run state
  // ------------------------------------------------------------

  async function refreshRun(runId: string) {
    const [
      updatedRun,
      updatedState,
      updatedExecutions,
    ] = await Promise.all([
      getRun(runId),
      getState(runId),
      getExecutions(runId),
    ]);

    setRun(updatedRun);
    setState(updatedState);
    setExecutions(updatedExecutions);

    return updatedRun;
  }

  // ------------------------------------------------------------
  // Start a new run
  // ------------------------------------------------------------

  async function startRun() {
    try {
      setStarting(true);
      setError(null);

      closeEvents();

      setRun(null);
      setGraph(null);
      setState({});
      setExecutions([]);
      setEvents([]);

      const result = await createRun({
        task: "Fix authentication bug",
      });

      const runId = result.runId;

      window.history.replaceState(
        null,
        "",
        `?run=${runId}`,
      );

      // Load initial state.
      const [
        initialRun,
        initialGraph,
        initialState,
        initialExecutions,
      ] = await Promise.all([
        getRun(runId),
        getGraph(runId),
        getState(runId),
        getExecutions(runId),
      ]);

      setRun(initialRun);
      setGraph(initialGraph);
      setState(initialState);
      setExecutions(initialExecutions);

      // --------------------------------------------------------
      // IMPORTANT:
      //
      // Connect to SSE after the run has been registered.
      //
      // The server replays history, so events that happened
      // before this connection are not lost.
      // --------------------------------------------------------

      const source = subscribeToEvents(
        runId,

        async (message) => {
          try {
            const event =
              JSON.parse(
                message.data,
              ) as GraphEvent;

            setEvents((current) => [
              ...current,
              event,
            ]);

            // Refresh the materialized runtime state.
            const updatedRun =
              await refreshRun(runId);

            if (
              updatedRun.status === "completed" ||
              updatedRun.status === "failed"
            ) {
              closeEvents();
            }
          } catch (err) {
            console.error(
              "Failed to process SSE event",
              err,
            );
          }
        },

        (event) => {
          console.log(
            "SSE connection error",
            event,
          );
        },
      );

      eventSourceRef.current = source;
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : String(err),
      );
    } finally {
      setStarting(false);
    }
  }

  // ------------------------------------------------------------
  // Reconnect to ?run=...
  // ------------------------------------------------------------

  useEffect(() => {
    const params =
      new URLSearchParams(
        window.location.search,
      );

    const runId = params.get("run");

    if (!runId) {
      return () => {
        closeEvents();
      };
    }

    let disposed = false;

    async function connect() {
      try {
        const [
          initialRun,
          initialGraph,
          initialState,
          initialExecutions,
        ] = await Promise.all([
          getRun(runId),
          getGraph(runId),
          getState(runId),
          getExecutions(runId),
        ]);

        if (disposed) {
          return;
        }

        setRun(initialRun);
        setGraph(initialGraph);
        setState(initialState);
        setExecutions(initialExecutions);

        const source = subscribeToEvents(
          runId,

          async (message) => {
            try {
              const event =
                JSON.parse(
                  message.data,
                ) as GraphEvent;

              setEvents((current) => {
                // Avoid duplicate event IDs when
                // reconnecting/replaying.
                if (
                  current.some(
                    (item) =>
                      item.id === event.id,
                  )
                ) {
                  return current;
                }

                return [
                  ...current,
                  event,
                ];
              });

              const updatedRun =
                await refreshRun(runId);

              if (
                updatedRun.status ===
                  "completed" ||
                updatedRun.status ===
                  "failed"
              ) {
                source.close();

                if (
                  eventSourceRef.current ===
                  source
                ) {
                  eventSourceRef.current =
                    null;
                }
              }
            } catch (err) {
              console.error(
                "Failed to process SSE event",
                err,
              );
            }
          },

          (event) => {
            console.log(
              "SSE connection error",
              event,
            );
          },
        );

        eventSourceRef.current = source;
      } catch (err) {
        if (!disposed) {
          setError(
            err instanceof Error
              ? err.message
              : String(err),
          );
        }
      }
    }

    connect();

    return () => {
      disposed = true;
      closeEvents();
    };
  }, []);

  // ------------------------------------------------------------
  // Error
  // ------------------------------------------------------------

  if (error) {
    return (
      <div className="app">
        <header className="header">
          <div>
            <h1>
              Go Coding Harness
            </h1>
          </div>
        </header>

        <div className="error">
          {error}
        </div>

        <button
          onClick={startRun}
          disabled={starting}
        >
          {starting
            ? "Starting..."
            : "Start run"}
        </button>
      </div>
    );
  }

  // ------------------------------------------------------------
  // No run
  // ------------------------------------------------------------

  if (!run || !graph) {
    return (
      <div className="app">
        <header className="header">
          <div>
            <h1>
              Go Coding Harness
            </h1>

            <div className="run-id">
              Agent orchestration demo
            </div>
          </div>
        </header>

        <section className="panel start-panel">
          <h2>
            Coding workflow
          </h2>

          <p>
            Start a workflow and watch
            the graph execute in real time.
          </p>

          <button
            onClick={startRun}
            disabled={starting}
          >
            {starting
              ? "Starting..."
              : "Start run"}
          </button>
        </section>
      </div>
    );
  }

  // ------------------------------------------------------------
  // Main UI
  // ------------------------------------------------------------

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>
            Go Coding Harness
          </h1>

          <div className="run-id">
            {run.id}
          </div>
        </div>

        <div
          className={`run-status status-${run.status}`}
        >
          <span className="status-dot" />

          {run.status}
        </div>
      </header>

      <div className="toolbar">
        <button
          onClick={startRun}
          disabled={starting}
        >
          {starting
            ? "Starting..."
            : "New run"}
        </button>
      </div>

      <main>
        <section className="panel graph-panel">
          <div className="panel-header">
            <h2>
              Workflow
            </h2>

            <span>
              {executions.length}
              {" "}executions
            </span>
          </div>

          <GraphView
            graph={graph}
            executions={executions}
            events={events}
          />
        </section>

        <div className="two-column">
          <section className="panel">
            <div className="panel-header">
              <h2>
                Executions
              </h2>
            </div>

            <ExecutionList
              executions={executions}
            />
          </section>

          <section className="panel">
            <div className="panel-header">
              <h2>
                State
              </h2>
            </div>

            <StateView
              state={state}
            />
          </section>
        </div>

        <section className="panel">
          <div className="panel-header">
            <h2>
              Events
            </h2>

            <span>
              {events.length}
            </span>
          </div>

          <EventList
            events={events}
          />
        </section>
      </main>
    </div>
  );
}