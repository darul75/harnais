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
import {
  ExecutionList,
} from "./ExecutionList";
import {
  ExecutionDetails,
} from "./ExecutionDetails";
import { StateView } from "./StateView";

export default function App() {

  const [run, setRun] =
    useState<Run | null>(null);

  const [graph, setGraph] =
    useState<GraphDefinition | null>(
      null,
    );

  const [state, setState] =
    useState<WorkflowState>({});

  const [executions, setExecutions] =
    useState<NodeExecution[]>([]);

  const [events, setEvents] =
    useState<GraphEvent[]>([]);

  const [
    selectedExecutionId,
    setSelectedExecutionId,
  ] = useState<string | null>(
    null,
  );

  const [starting, setStarting] =
    useState(false);

  const [error, setError] =
    useState<string | null>(null);

  const eventSourceRef =
    useRef<EventSource | null>(null);

  function closeEvents() {

    eventSourceRef.current?.close();

    eventSourceRef.current =
      null;
  }

  async function refreshRun(
    runId: string,
  ) {

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

    setExecutions(
      updatedExecutions,
    );

    return updatedRun;
  }

  function selectExecution(
    execution: NodeExecution,
  ) {

    setSelectedExecutionId(
      execution.id,
    );
  }

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

      setSelectedExecutionId(
        null,
      );

      const result =
        await createRun({
          task:
            "Fix authentication bug",
        });

      const runId =
        result.runId;

      window.history.replaceState(
        null,
        "",
        `?run=${runId}`,
      );

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

      setExecutions(
        initialExecutions,
      );

      const source =
        subscribeToEvents(

          runId,

          async (message) => {

            try {

              const event =
                JSON.parse(
                  message.data,
                ) as GraphEvent;

              setEvents(
                (current) => {

                  if (
                    current.some(
                      (item) =>
                        item.id ===
                        event.id,
                    )
                  ) {
                    return current;
                  }

                  return [
                    ...current,
                    event,
                  ];
                },
              );

              const updatedRun =
                await refreshRun(
                  runId,
                );

              if (
                updatedRun.status ===
                  "completed" ||
                updatedRun.status ===
                  "failed"
              ) {

                source.close();

                eventSourceRef.current =
                  null;
              }

            } catch (err) {

              console.error(
                "SSE event error",
                err,
              );
            }
          },

          (event) => {

            console.log(
              "SSE error",
              event,
            );
          },
        );

      eventSourceRef.current =
        source;

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
  // Reconnect to existing ?run=...
  // ------------------------------------------------------------

  useEffect(() => {

    const params =
      new URLSearchParams(
        window.location.search,
      );

    const runId =
      params.get("run");

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

        setExecutions(
          initialExecutions,
        );

        const source =
          subscribeToEvents(

            runId,

            async (message) => {

              if (disposed) {
                return;
              }

              try {

                const event =
                  JSON.parse(
                    message.data,
                  ) as GraphEvent;

                setEvents(
                  (current) => {

                    if (
                      current.some(
                        (item) =>
                          item.id ===
                          event.id,
                      )
                    ) {
                      return current;
                    }

                    return [
                      ...current,
                      event,
                    ];
                  },
                );

                const updatedRun =
                  await refreshRun(
                    runId,
                  );

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
                  err,
                );
              }
            },

            (event) => {

              console.log(
                "SSE error",
                event,
              );
            },
          );

        eventSourceRef.current =
          source;

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

  const selectedExecution =
    executions.find(
      (execution) =>
        execution.id ===
        selectedExecutionId,
    ) ?? null;

  if (error) {

    return (
      <div className="app">

        <header className="header">

          <h1>
            Go Coding Harness
          </h1>

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
            Start a workflow and inspect
            every node, worker, agent,
            LLM and tool execution.
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
          className={
            `run-status status-${run.status}`
          }
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

            selectedExecutionId={
              selectedExecutionId
            }

            onSelectExecution={
              selectExecution
            }

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

              executions={
                executions
              }

              selectedExecutionId={
                selectedExecutionId
              }

              onSelect={
                selectExecution
              }

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

          <ExecutionDetails

            execution={
              selectedExecution
            }

            events={
              events
            }

          />

        </section>

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

            selectedExecutionId={
              selectedExecutionId
            }

            onSelectExecution={
              setSelectedExecutionId
            }

          />

        </section>

      </main>

    </div>
  );
}