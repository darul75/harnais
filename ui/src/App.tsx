import {
  type FormEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  ApiError,
  createEventSource,
  createRun,
  getAllReports,
  getRun,
  getRuns,
  getRunReport,
  getRunReports,
  getRunTree,
  getWorkflows,
} from "./api";

import type {
  AgentActivity,
  AgentExecution,
  ExecutionEdge,
  ExecutionNode,
  LLMCall,
  Report,
  Run,
  RunSummary,
  RunTree,
  RuntimeEvent,
  ToolCall,
  Workflow,
} from "./types";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { WorkflowView } from "./WorkflowView";
import { SettingsView } from "./SettingsView";

type GraphLayoutNode = {
  id: string;
  x: number;
  y: number;
  nodeId: string;
  status: string;
};

const EVENT_TYPES = [
  "run.started",
  "run.completed",
  "run.failed",
  "node.started",
  "node.completed",
  "node.failed",
  "edge.activated",
  "worker.started",
  "worker.completed",
  "worker.failed",
  "agent.started",
  "agent.completed",
  "llm.started",
  "llm.completed",
  "tool.started",
  "tool.completed",
  "tool.failed",
];

function App() {
  const [task, setTask] = useState("");
  const [starting, setStarting] = useState(false);

  const { runId, workflowId, settings, selectRun, selectWorkflow, openSettings, clear } =
    useRoute();

  const [run, setRun] =
    useState<Run | null>(null);

  const [runs, setRuns] =
    useState<RunSummary[]>([]);

  const [tree, setTree] =
    useState<RunTree | null>(null);

  const [events, setEvents] =
    useState<RuntimeEvent[]>([]);

  const [selectedExecutionId, setSelectedExecutionId] =
    useState<string | null>(null);

  const [selectedEventId, setSelectedEventId] =
    useState<number | null>(null);

  const [error, setError] =
    useState<string | null>(null);

  const [loading, setLoading] =
    useState(false);

  const [workflows, setWorkflows] =
    useState<Workflow[]>([]);

  // ------------------------------------------------------------
  // Load workflows
  // ------------------------------------------------------------

  useEffect(() => {
    let disposed = false;

    getWorkflows()
      .then((list) => {
        if (disposed) {
          return;
        }
        setWorkflows(list);
      })
      .catch((err) => {
        console.error(
          "Failed to load workflows",
          err,
        );
      });

    return () => {
      disposed = true;
    };
  }, []);

  // ------------------------------------------------------------
  // Load runs
  // ------------------------------------------------------------

  async function refreshRuns() {
    try {
      const list =
        await getRuns();

      setRuns(list);
    } catch (err) {
      console.error(
        "Failed to load runs",
        err,
      );
    }
  }

  useEffect(() => {
    void refreshRuns();
  }, []);

  // ------------------------------------------------------------
  // Load run
  // ------------------------------------------------------------

  async function refreshRun(
    id: string,
  ) {
    try {
      setLoading(true);

      const [
        currentRun,
        currentTree,
      ] = await Promise.all([
        getRun(id),
        getRunTree(id),
      ]);

      setRun(currentRun);
      setTree(currentTree);

      setSelectedExecutionId(
        (current) => {
          if (
            current &&
            currentTree.nodes.some(
              (node: ExecutionNode) =>
                node.id === current,
            )
          ) {
            return current;
          }

          const running =
            currentTree.nodes.find(
              (node: ExecutionNode) =>
                node.status === "running",
            );

          return (
            running?.id ??
            currentTree.nodes[
              currentTree.nodes.length - 1
            ]?.id ??
            null
          );
        },
      );
    } catch (err) {
      console.error(
        "Failed to refresh run",
        err,
      );

      if (
        err instanceof ApiError &&
        err.status === 404
      ) {
        clear();

        setError(
          `Run not found: ${id}`,
        );
      }
    } finally {
      setLoading(false);
    }
  }

  // ------------------------------------------------------------
  // Start run
  // ------------------------------------------------------------

  async function startRun(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();

    const value =
      task.trim();

    if (!value || starting) {
      return;
    }

    try {
      setStarting(true);
      setError(null);

      const result =
        await createRun(
          value,
          workflowId ?? undefined,
        );

      switchRun(
        result.runId,
      );

      await refreshRun(
        result.runId,
      );

      await refreshRuns();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to start run",
      );
    } finally {
      setStarting(false);
    }
  }

  // ------------------------------------------------------------
  // Run switching
  // ------------------------------------------------------------

  function switchRun(id: string) {
    setRun(null);
    setTree(null);
    setEvents([]);
    setSelectedExecutionId(null);
    setSelectedEventId(null);
    setError(null);
    selectRun(id);
  }

  function newRun() {
    setRun(null);
    setTree(null);
    setEvents([]);
    setSelectedExecutionId(null);
    setSelectedEventId(null);
    setError(null);
    clear();
  }

  // ------------------------------------------------------------
  // SSE
  // ------------------------------------------------------------

  useEffect(() => {
    if (!runId) {
      return;
    }

    let disposed = false;

    void refreshRun(runId);

    const source =
      createEventSource(runId);

    function handleEvent(
      event: MessageEvent,
    ) {
      if (disposed) {
        return;
      }

      try {
        const runtimeEvent:
          RuntimeEvent =
          JSON.parse(
            event.data,
          );

        setEvents(
          (current) => {
            if (
              runtimeEvent.id !==
                undefined &&
              current.some(
                (item) =>
                  item.id ===
                  runtimeEvent.id,
              )
            ) {
              return current;
            }

            return [
              ...current,
              runtimeEvent,
            ];
          },
        );

        if (
          runtimeEvent.executionID
        ) {
          setSelectedExecutionId(
            (current) =>
              current ??
              runtimeEvent.executionID ??
              null,
          );
        }

        void refreshRun(runId);

        if (
          runtimeEvent.type.startsWith(
            "run.",
          )
        ) {
          void refreshRuns();
        }
      } catch (err) {
        console.error(
          "Failed to parse SSE event",
          err,
        );
      }
    }

    for (
      const eventType of EVENT_TYPES
    ) {
      source.addEventListener(
        eventType,
        handleEvent,
      );
    }

    source.onerror = () => {
      // EventSource automatically retries.
    };

    return () => {
      disposed = true;

      for (
        const eventType of EVENT_TYPES
      ) {
        source.removeEventListener(
          eventType,
          handleEvent,
        );
      }

      source.close();
    };
  }, [runId]);

  // ------------------------------------------------------------
  // Clear run-page state when leaving the run view.
  //
  // Navigating from a run to a workflow (or settings) page does
  // not change runId through the route, so the previously loaded
  // run would otherwise keep rendering.
  // ------------------------------------------------------------

  useEffect(() => {
    if (!workflowId && !settings) {
      return;
    }

    setRun(null);
    setTree(null);
    setEvents([]);
    setSelectedExecutionId(null);
    setSelectedEventId(null);
    setError(null);
  }, [workflowId, settings]);

  // ------------------------------------------------------------
  // Derived data
  // ------------------------------------------------------------

  const executions =
    tree?.nodes ?? [];

  const selectedExecution =
    executions.find(
      (execution) =>
        execution.id ===
        selectedExecutionId,
    ) ?? null;

  const completedCount =
    executions.filter(
      (execution) =>
        execution.status ===
        "completed",
    ).length;

  const runningCount =
    executions.filter(
      (execution) =>
        execution.status ===
        "running",
    ).length;

  const failedCount =
    executions.filter(
      (execution) =>
        execution.status ===
        "failed",
    ).length;

  const graphLayout =
    useMemo(
      () =>
        buildGraphLayout(
          executions,
          tree?.edges ?? [],
        ),
      [executions, tree?.edges],
    );

  const selectedIndex =
    runs.findIndex(
      (summary) =>
        summary.id === runId,
    );

  const prevId =
    selectedIndex > 0
      ? runs[selectedIndex - 1].id
      : null;

  const nextId =
    selectedIndex >= 0 &&
    selectedIndex <
      runs.length - 1
      ? runs[selectedIndex + 1].id
      : null;

  return (
    <div className="app">
      {/* ======================================================= */}
      {/* Header */}
      {/* ======================================================= */}

      <header className="header">
        <div>
          <h1>
            Harnais
          </h1>

          {run && (
            <div className="run-id">
              {run.id}
            </div>
          )}
        </div>

        <div className="header-actions">
          <button
            type="button"
            className={`settings-button ${
              settings
                ? "settings-button-active"
                : ""
            }`}
            onClick={openSettings}
            aria-label="Settings"
            title="Settings"
          >
            {"\u2699"}
          </button>

          {runId && (
            <>
              <div className="run-nav">
                <button
                  type="button"
                  onClick={() =>
                    prevId &&
                    switchRun(prevId)
                  }
                  disabled={!prevId}
                  aria-label="Previous run"
                >
                  {"\u25C0"}
                </button>

                <span className="run-nav-count">
                  {selectedIndex + 1}{" "}
                  / {runs.length}
                </span>

                <button
                  type="button"
                  onClick={() =>
                    nextId &&
                    switchRun(nextId)
                  }
                  disabled={!nextId}
                  aria-label="Next run"
                >
                  {"\u25B6"}
                </button>
              </div>

              <button
                type="button"
                className="new-run"
                onClick={newRun}
              >
                New Run
              </button>
            </>
          )}

          {run && (
            <RunStatus
              status={run.status}
            />
          )}
        </div>
      </header>

      {error && (
        <div className="error">
          {error}
        </div>
      )}

      <div className="flex gap-5 items-start">
        {/* =================================================== */}
        {/* Sidebar: runs + workflow selection */}
        {/* =================================================== */}

        <aside className="w-64 shrink-0 flex flex-col gap-5">
          <div className="panel">
            <div className="panel-header">
              <h2>
                Runs
              </h2>

              <span>
                {runs.length}
              </span>
            </div>

            <div className="run-list">
              {runs.map((summary) => (
                <button
                  key={summary.id}
                  type="button"
                  className={`run-row ${
                    summary.id === runId
                      ? "run-selected"
                      : ""
                  }`}
                  onClick={() =>
                    switchRun(summary.id)
                  }
                >
                  <span
                    className={`status-dot status-${summary.status}`}
                  />

                  <span className="run-task">
                    {summary.task ||
                      summary.id}
                  </span>

                  <span className="run-meta">
                    {formatTime(
                      summary.startedAt,
                    )}
                  </span>
                </button>
              ))}

              {!runs.length && (
                <div className="empty">
                  No runs yet
                </div>
              )}
            </div>
          </div>

          <div className="panel">
            <div className="panel-header">
              <h2>
                Workflows
              </h2>

              <span>
                {workflows.length}
              </span>
            </div>

            <div className="workflow-list">
              <button
                type="button"
                className={`workflow-row ${
                  workflowId === null
                    ? "workflow-selected"
                    : ""
                }`}
                onClick={() =>
                  clear()
                }
              >
                <span className="workflow-title">
                  Auto
                </span>

                <span className="workflow-desc">
                  Choose automatically from the request
                </span>
              </button>

              {workflows.map(
                (workflow) => (
                  <button
                    key={workflow.id}
                    type="button"
                    className={`workflow-row ${
                      workflowId ===
                      workflow.id
                        ? "workflow-selected"
                        : ""
                    }`}
                    onClick={() =>
                      selectWorkflow(
                        workflow.id,
                      )
                    }
                  >
                    <span className="workflow-title">
                      {workflow.title}
                    </span>

                    <span className="workflow-desc">
                      {workflow.description}
                    </span>
                  </button>
                ),
              )}

              {!workflows.length && (
                <div className="empty">
                  No workflows available
                </div>
              )}
            </div>
          </div>
        </aside>

        {/* =================================================== */}
        {/* Main content */}
        {/* =================================================== */}

        <main className="flex-1 min-w-0">
          {/* ================================================= */}
          {/* Settings page */}
          {/* ================================================= */}

          {settings && (
            <SettingsView />
          )}

          {/* ================================================= */}
          {/* Workflow page */}
          {/* ================================================= */}

          {!settings &&
            workflowId && (
            <WorkflowView
              workflowId={workflowId}
              onRunStarted={switchRun}
              onRunComplete={refreshRuns}
            />
          )}

          {/* ================================================= */}
          {/* Run */}
          {/* ================================================= */}

          {!settings &&
            !workflowId &&
            run && (
            <>
              {/* --------------------------------------------------- */}
              {/* Run + Workflow side-by-side */}
              {/* --------------------------------------------------- */}

          <section className="panel run-bar">
            <div className="run-bar-main">
              <span className="run-bar-task">
                {run.task ??
                  task}
              </span>

              <RunStatus
                status={run.status}
              />
            </div>

            <span className="run-bar-times">
              Started{" "}
              {formatDate(
                run.startedAt,
              )}

              {run.completedAt
                ? ` · completed ${formatDate(
                    run.completedAt,
                  )}`
                : ""}
            </span>

            <div className="run-bar-counts">
              <span>
                Executions{" "}
                {executions.length}
              </span>

              <span>
                Completed{" "}
                {completedCount}
              </span>

              <span>
                Running{" "}
                {runningCount}
              </span>

              <span>
                Failed{" "}
                {failedCount}
              </span>

              <span>
                Events{" "}
                {events.length}
              </span>
            </div>

            <span className="run-bar-id">
              {run.id}
            </span>
          </section>


          {/* --------------------------------------------------- */}
          {/* Workflow graph */}
          {/* --------------------------------------------------- */}

          <section className="panel">
            <div className="panel-header">
              <h2>
                Workflow
              </h2>

              <span>
                {executions.length}{" "}
                executions
              </span>
            </div>

            <div className="graph-container">
              {executions.length ===
              0 ? (
                <div className="empty">
                  Waiting for workflow
                  executions...
                </div>
              ) : (
                <svg
                  className="graph-svg"
                  viewBox={runViewBoxFor(
                    graphLayout,
                  )}
                  preserveAspectRatio="xMinYMin meet"
                >
                  {/* Edges */}

                  {tree?.edges.map(
                    (edge) => {
                      const from =
                        graphLayout.find(
                          (node) =>
                            node.id ===
                            edge.fromExecutionId,
                        );

                      const to =
                        graphLayout.find(
                          (node) =>
                            node.id ===
                            edge.toExecutionId,
                        );

                      if (
                        !from ||
                        !to
                      ) {
                        return null;
                      }

                      return (
                        <g
                          key={edge.id}
                        >
                          <line
                            x1={
                              from.x +
                              100
                            }
                            y1={
                              from.y +
                              32
                            }
                            x2={
                              to.x
                            }
                            y2={
                              to.y +
                              32
                            }
                            stroke="#4b5563"
                            strokeWidth="2"
                          />

                          <polygon
                            points={arrowPoints(
                              from.x +
                                100,
                              from.y +
                                32,
                              to.x,
                              to.y +
                                32,
                            )}
                            fill="#6b7280"
                          />
                        </g>
                      );
                    },
                  )}

                  {/* Execution nodes */}

                  {graphLayout.map(
                    (node) => {
                      const selected =
                        node.id ===
                        selectedExecutionId;

                      return (
                        <g
                          key={node.id}
                          className={
                            selected
                              ? "graph-node-selected"
                              : "graph-node"
                          }
                          onClick={() =>
                            setSelectedExecutionId(
                              node.id,
                            )
                          }
                        >
                          <rect
                            className={`node node-${node.status}`}
                            x={node.x}
                            y={node.y}
                            width="100"
                            height="64"
                            rx="8"
                          />

                          <text
                            className="node-title"
                            x={
                              node.x +
                              50
                            }
                            y={
                              node.y +
                              27
                            }
                            textAnchor="middle"
                          >
                            {node.nodeId}
                          </text>

                          <text
                            className="node-status"
                            x={
                              node.x +
                              50
                            }
                            y={
                              node.y +
                              47
                            }
                            textAnchor="middle"
                          >
                            {node.status}
                          </text>
                        </g>
                      );
                    },
                  )}
                </svg>
              )}
            </div>
          </section>

          {/* --------------------------------------------------- */}
          {/* Executions (chip strip) + selected details */}
          {/* --------------------------------------------------- */}

          <section className="panel">
            <div className="panel-header">
              <h2>
                Executions
              </h2>

              <span>
                {executions.length}
              </span>
            </div>

            <div className="execution-chips">
              {executions.map(
                (execution) => (
                  <ExecutionChip
                    key={
                      execution.id
                    }
                    execution={
                      execution
                    }
                    selected={
                      execution.id ===
                      selectedExecutionId
                    }
                    onClick={() =>
                      setSelectedExecutionId(
                        execution.id,
                      )
                    }
                  />
                ),
              )}

              {!executions.length && (
                <div className="empty">
                  No executions yet.
                </div>
              )}
            </div>
          </section>

          <section className="panel">
            {selectedExecution ? (
              <ExecutionDetails
                execution={
                  selectedExecution
                }
              />
            ) : (
              <div className="empty">
                Select an execution
                to inspect it.
              </div>
            )}
          </section>

          {/* --------------------------------------------------- */}
          {/* State */}
          {/* --------------------------------------------------- */}

          <section className="panel">
            <details className="panel-collapsible">
              <summary className="panel-header">
                <span className="caret">
                  {"\u25B8"}
                </span>

                <h2>
                  State
                </h2>

                <span>
                  {selectedExecution
                    ? `when ${selectedExecution.nodeId} started`
                    : "Workflow state"}
                </span>
              </summary>

              <div className="execution-details">
                <pre>
                  {JSON.stringify(
                    selectedExecution
                      ? selectedExecution.input ??
                          {}
                      : run.state ?? {},
                    null,
                    2,
                  )}
                </pre>
              </div>
            </details>
          </section>

          {/* --------------------------------------------------- */}
          {/* Events */}
          {/* --------------------------------------------------- */}

          <section className="panel">
            <details className="panel-collapsible">
              <summary className="panel-header">
                <span className="caret">
                  {"\u25B8"}
                </span>

                <h2>
                  Events
                </h2>

                <span>
                  {events.length}
                </span>
              </summary>

              <div className="events">
                {events.map(
                  (event, index) => (
                    <EventRow
                      key={`${event.id ?? "event"}-${index}`}
                      event={event}
                      selected={
                        event.id ===
                        selectedEventId
                      }
                      onClick={() =>
                        setSelectedEventId(
                          event.id ??
                            null,
                        )
                      }
                    />
                  ),
                )}

                {!events.length && (
                  <div className="empty">
                    Waiting for events...
                  </div>
                )}
              </div>
            </details>
          </section>

          <ReportsPanel
            runId={run.id}
            active={
              run.status ===
                "running" ||
              run.status ===
                "pending"
            }
          />
        </>
      )}

          {/* ================================================= */}
          {/* Initial screen */}
          {/* ================================================= */}

          {!settings &&
            !workflowId &&
            !run && (
            <section className="panel start-panel">
              <h2>
                Start a workflow
              </h2>

              <p>
                Describe the feature, fix, or
                question below. The workflow
                will plan and run the steps,
                exposing every execution,
                agent, LLM, and tool step
                along the way.
              </p>

              <form
                className="start-form"
                onSubmit={startRun}
              >
                <textarea
                  className="task-input"
                  value={task}
                  onChange={(event) =>
                    setTask(
                      event.target.value,
                    )
                  }
                  onKeyDown={(event) => {
                    if (
                      event.key ===
                        "Enter" &&
                      !event.shiftKey
                    ) {
                      event.preventDefault();
                      event.currentTarget.form?.requestSubmit();
                    }
                  }}
                  placeholder="Describe the feature, fix, or question..."
                  disabled={starting}
                />

                <button
                  type="submit"
                  className="start-button"
                  disabled={
                    starting ||
                    !task.trim()
                  }
                >
                  {starting
                    ? "Starting..."
                    : "Start Run"}
                </button>
              </form>
            </section>
          )}
        </main>
      </div>
    </div>
  );
}

// ============================================================
// Reports panel
// ============================================================

type ReportsMode = "run" | "all";

function ReportsPanel({
  runId,
  active,
}: {
  runId: string;
  active: boolean;
}) {
  const [mode, setMode] =
    useState<ReportsMode>("run");

  const [reports, setReports] =
    useState<Report[] | null>(null);

  const [selected, setSelected] =
    useState<Report | null>(null);

  const [content, setContent] =
    useState<string | null>(null);

  const [error, setError] =
    useState<string | null>(null);

  // ------------------------------------------------------------
  // Report list: load once, then poll while the run is active
  // ------------------------------------------------------------

  useEffect(() => {
    let disposed = false;

    setReports(null);
    setSelected(null);
    setContent(null);
    setError(null);

    const load = () => {
      const loader =
        mode === "run"
          ? getRunReports(runId)
          : getAllReports();

      loader
        .then((list) => {
          if (disposed) {
            return;
          }
          setReports(list);
        })
        .catch((err) => {
          if (disposed) {
            return;
          }
          setError(String(err));
        });
    };

    load();

    if (!active) {
      return () => {
        disposed = true;
      };
    }

    const timer =
      window.setInterval(
        load,
        3000,
      );

    return () => {
      disposed = true;
      window.clearInterval(
        timer,
      );
    };
  }, [mode, runId, active]);

  // ------------------------------------------------------------
  // Selected report content: load once, then poll while active
  // ------------------------------------------------------------

  useEffect(() => {
    if (!selected) {
      return;
    }

    let disposed = false;

    setContent(null);
    setError(null);

    const load = () => {
      getRunReport(
        selected.runId,
        selected.name,
      )
        .then((text) => {
          if (disposed) {
            return;
          }
          setContent(text);
        })
        .catch((err) => {
          if (disposed) {
            return;
          }
          setError(String(err));
        });
    };

    load();

    if (!active) {
      return () => {
        disposed = true;
      };
    }

    const timer =
      window.setInterval(
        load,
        3000,
      );

    return () => {
      disposed = true;
      window.clearInterval(
        timer,
      );
    };
  }, [
    selected?.runId,
    selected?.name,
    active,
  ]);

  // ------------------------------------------------------------
  // Open a report
  // ------------------------------------------------------------

  function open(report: Report) {
    setSelected(report);
  }

  // Group all-run reports by run ID.
  const grouped = useMemo(() => {
    if (mode !== "all" || !reports) {
      return [];
    }

    const byRun = new Map<
      string,
      Report[]
    >();

    for (const report of reports) {
      const list =
        byRun.get(report.runId) ?? [];
      list.push(report);
      byRun.set(report.runId, list);
    }

    return [...byRun.entries()];
  }, [mode, reports]);

  return (
    <section className="panel">
      <details
        className="panel-collapsible"
        open
      >
        <summary className="panel-header">
          <span className="caret">
            {"\u25B8"}
          </span>

          <h2>
            Reports
          </h2>

          <span className="reports-toggle">
            <button
              type="button"
              className={
                mode === "run"
                  ? "reports-toggle-active"
                  : ""
              }
              onClick={() =>
                setMode("run")
              }
            >
              This run
            </button>

            <button
              type="button"
              className={
                mode === "all"
                  ? "reports-toggle-active"
                  : ""
              }
              onClick={() =>
                setMode("all")
              }
            >
              All runs
            </button>
          </span>
        </summary>

        <div className="reports">
          <div className="report-list">
            {mode === "run" && (
              <div className="reports-group">
                {reports?.map((report) => (
                  <button
                    key={report.name}
                    type="button"
                    className={`report-row ${
                      selected?.name ===
                        report.name &&
                      selected?.runId ===
                        report.runId
                        ? "report-selected"
                        : ""
                    }`}
                    onClick={() =>
                      open(report)
                    }
                  >
                    <span className="report-name">
                      {report.name}
                    </span>

                    <span className="report-meta">
                      {formatBytes(
                        report.size,
                      )}
                    </span>
                  </button>
                ))}
              </div>
            )}

            {mode === "all" &&
              grouped.map(
                ([runID, list]) => (
                  <div
                    key={runID}
                    className="reports-group"
                  >
                    <div className="reports-run">
                      {runID}
                    </div>

                    {list.map((report) => (
                      <button
                        key={report.name}
                        type="button"
                        className={`report-row ${
                          selected?.name ===
                            report.name &&
                          selected?.runId ===
                            report.runId
                            ? "report-selected"
                            : ""
                        }`}
                        onClick={() =>
                          open(report)
                        }
                      >
                        <span className="report-name">
                          {report.name}
                        </span>

                        <span className="report-meta">
                          {formatBytes(
                            report.size,
                          )}
                        </span>
                      </button>
                    ))}
                  </div>
                ),
              )}

            {reports &&
              reports.length ===
                0 && (
                <div className="empty">
                  No reports yet.
                </div>
              )}

            {!reports &&
              !error && (
                <div className="empty">
                  Loading reports...
                </div>
              )}
          </div>

          <div className="report-view">
            {error && (
              <div className="error">
                {error}
              </div>
            )}

            {!selected &&
              !error && (
                <div className="empty">
                  Select a report to
                  view it.
                </div>
              )}

            {content !== null && (
              <div className="markdown">
                <ReactMarkdown
                  remarkPlugins={[
                    remarkGfm,
                  ]}
                  components={{
                    a: ({ node, ...props }) => (
                      <a
                        {...props}
                        target="_blank"
                        rel="noopener noreferrer"
                      />
                    ),
                  }}
                >
                  {cleanMarkdownContent(
                    content,
                  )}
                </ReactMarkdown>
              </div>
            )}

            {selected &&
              content === null &&
              !error && (
                <div className="empty">
                  Loading...
                </div>
              )}
          </div>
        </div>
      </details>
    </section>
  );
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  return `${(bytes / 1024).toFixed(
    1,
  )} KB`;
}

const INVISIBLE_CHARS =
  /[\u200B-\u200F\u2060-\u206F\uFEFF\u00AD\u061C\u115F\u1160\u17B4\u17B5\u180E\uFFF9-\uFFFB]/g;

// cleanMarkdownContent strips artifacts the web-search LLM copies
// into its prose before rendering: Turn0-style citation tokens
// (e.g. "[urn0search1]"), their "cite" labels (with any invisible
// characters interleaved), and stray invisible/zero-width unicode
// characters. Raw reports on disk are left untouched.
function cleanMarkdownContent(
  markdown: string,
): string {
  return (
    markdown
      // "cite" labels preceding citation tokens, case-insensitively,
      // with optional invisible characters between letters.
      .replace(
        /c\s*i\s*t\s*e/gi,
        "",
      )
      // Turn0-style citation clusters: [turn0search0], [urn0search1], ...
      .replace(
        /\[(?:t)?urn\d*search\d+\]/g,
        "",
      )
      // Leftover zero-width / invisible unicode characters.
      .replace(
        INVISIBLE_CHARS,
        "",
      )
  );
}

// ============================================================
// Route (hash-based URL navigation)
// ============================================================

type Route = {
  runId: string | null;
  workflowId: string | null;
  settings: boolean;
};

const EMPTY_ROUTE: Route = {
  runId: null,
  workflowId: null,
  settings: false,
};

function parseHashSegment(
  hash: string,
): string | null {
  const match =
    hash.match(/\/([^/]+)$/);

  if (!match) {
    return null;
  }

  try {
    return decodeURIComponent(
      match[1],
    );
  } catch {
    return null;
  }
}

function parseRoute(
  hash: string,
): Route {
  if (
    hash.startsWith(
      "#/runs/",
    )
  ) {
    return {
      runId:
        parseHashSegment(
          hash,
        ),

      workflowId: null,

      settings: false,
    };
  }

  if (
    hash.startsWith(
      "#/workflows/",
    )
  ) {
    return {
      runId: null,

      workflowId:
        parseHashSegment(
          hash,
        ),

      settings: false,
    };
  }

  if (
    hash.startsWith(
      "#/settings",
    )
  ) {
    return {
      runId: null,

      workflowId: null,

      settings: true,
    };
  }

  return EMPTY_ROUTE;
}

function runHash(id: string): string {
  return `#/runs/${encodeURIComponent(id)}`;
}

function workflowHash(id: string): string {
  return `#/workflows/${encodeURIComponent(id)}`;
}

function settingsHash(): string {
  return "#/settings";
}

function useRoute() {
  const [route, setRoute] =
    useState<Route>(
      () =>
        parseRoute(
          window.location.hash,
        ),
    );

  useEffect(() => {
    function syncFromURL() {
      setRoute(
        parseRoute(
          window.location.hash,
        ),
      );
    }

    window.addEventListener(
      "popstate",
      syncFromURL,
    );

    window.addEventListener(
      "hashchange",
      syncFromURL,
    );

    return () => {
      window.removeEventListener(
        "popstate",
        syncFromURL,
      );

      window.removeEventListener(
        "hashchange",
        syncFromURL,
      );
    };
  }, []);

  const push =
    useCallback(
      (url: string) => {
        if (
          window.location.hash !==
          url
        ) {
          window.history.pushState(
            null,
            "",
            url,
          );
        }

        setRoute(
          parseRoute(url),
        );
      },
      [],
    );

  const selectRun =
    useCallback(
      (id: string) =>
        push(runHash(id)),
      [push],
    );

  const selectWorkflow =
    useCallback(
      (id: string) =>
        push(
          workflowHash(id),
        ),
      [push],
    );

  const openSettings =
    useCallback(
      () => push(settingsHash()),
      [push],
    );

  const clear =
    useCallback(() => {
      const url =
        window.location.pathname +
        window.location.search;

      if (
        window.location.hash !==
        ""
      ) {
        window.history.pushState(
          null,
          "",
          url,
        );
      }

      setRoute(EMPTY_ROUTE);
    }, []);

  return {
    runId: route.runId,

    workflowId:
      route.workflowId,

    settings: route.settings,

    selectRun,

    selectWorkflow,

    openSettings,

    clear,
  };
}

// ============================================================
// Execution row
// ============================================================

function ExecutionChip({
  execution,
  selected,
  onClick,
}: {
  execution: ExecutionNode;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`execution-chip ${
        selected
          ? "execution-chip-selected"
          : ""
      }`}
      onClick={onClick}
    >
      <span
        className={`status-dot status-${execution.status}`}
      />

      <span className="execution-node">
        {execution.nodeId}
      </span>

      <span className="execution-attempt">
        #{execution.attempt}
      </span>
    </button>
  );
}

// ============================================================
// Execution details
// ============================================================

function ExecutionDetails({
  execution,
}: {
  execution: ExecutionNode;
}) {
  const agent =
    execution.agent;

  return (
    <div className="execution-details">
      <div className="details-header">
        <div>
          <h2>
            {execution.nodeId}
          </h2>

          <div className="details-subtitle">
            {execution.workerId}
            {" · "}
            attempt{" "}
            {execution.attempt}
          </div>
        </div>

        <RunStatus
          status={execution.status}
        />
      </div>

      <div className="details-meta">
        <div>
          <span>
            Execution ID
          </span>

          <strong>
            {execution.id}
          </strong>
        </div>

        <div>
          <span>
            Worker
          </span>

          <strong>
            {execution.workerId}
          </strong>
        </div>

        <div>
          <span>
            Attempt
          </span>

          <strong>
            {execution.attempt}
          </strong>
        </div>
      </div>

      {execution.triggeredBy &&
        execution.triggeredBy.length >
          0 && (
          <CollapsibleSection
            title={`Triggered by (${execution.triggeredBy.length})`}
            preview={previewText(
              execution.triggeredBy,
            )}
          >
            <pre>
              {JSON.stringify(
                execution.triggeredBy,
                null,
                2,
              )}
            </pre>
          </CollapsibleSection>
        )}

      {execution.input !==
        undefined && (
        <CollapsibleSection
          title="Input"
          preview={previewText(
            execution.input,
          )}
        >
          <pre>
            {JSON.stringify(
              execution.input ??
                {},
              null,
              2,
            )}
          </pre>
        </CollapsibleSection>
      )}

      {execution.output !==
        undefined && (
        <CollapsibleSection
          title="Output"
          preview={previewText(
            execution.output,
          )}
        >
          <pre>
            {JSON.stringify(
              execution.output ??
                {},
              null,
              2,
            )}
          </pre>
        </CollapsibleSection>
      )}

      {agent && (
        <CollapsibleSection
          title={`Agent · ${agent.agentId}`}
          open={
            agent.status ===
              "running" ||
            agent.status ===
              "pending"
          }
        >
          <AgentExecutionDetails
            agent={agent}
          />
        </CollapsibleSection>
      )}
    </div>
  );
}

// ============================================================
// Collapsible section
// ============================================================

function CollapsibleSection({
  title,
  preview,
  open,
  children,
}: {
  title: string;
  preview?: string;
  open?: boolean;
  children: ReactNode;
}) {
  return (
    <details
      className="collapsible-section"
      open={open}
    >
      <summary>
        <span className="caret">
          {"\u25B8"}
        </span>

        <span className="collapsible-title">
          {title}
        </span>

        {preview && (
          <span className="collapsible-preview">
            {preview}
          </span>
        )}
      </summary>

      <div className="collapsible-body">
        {children}
      </div>
    </details>
  );
}

function previewText(
  value: unknown,
  max = 90,
): string {
  let text = "";

  try {
    text =
      JSON.stringify(
        value ?? {},
      ) ?? "";
  } catch {
    text =
      String(value);
  }

  if (
    text.length >
    max
  ) {
    return `${text.slice(
      0,
      max,
    )}\u2026`;
  }

  return text;
}

// ============================================================
// Agent execution
// ============================================================

function AgentExecutionDetails({
  agent,
}: {
  agent: AgentExecution;
}) {
  const llmCalls =
    agent.llmCalls ?? [];

  const toolCalls =
    agent.toolCalls ?? [];

  return (
    <div className="agent-execution">
      <div className="details-header">
        <div>
          <h2>
            Agent
          </h2>

          <div className="details-subtitle">
            {agent.agentId}
          </div>
        </div>

        <RunStatus
          status={agent.status}
        />
      </div>

      <div className="details-meta">
        <div>
          <span>
            Agent ID
          </span>

          <strong>
            {agent.agentId}
          </strong>
        </div>

        <div>
          <span>
            LLM Calls
          </span>

          <strong>
            {llmCalls.length}
          </strong>
        </div>

        <div>
          <span>
            Tool Calls
          </span>

          <strong>
            {toolCalls.length}
          </strong>
        </div>
      </div>

      <AgentActivityList
        agent={agent}
        llmCalls={llmCalls}
        toolCalls={toolCalls}
      />
    </div>
  );
}

// ============================================================
// Agent activity
// ============================================================

function AgentActivityList({
  agent,
  llmCalls,
  toolCalls,
}: {
  agent: AgentExecution;
  llmCalls: LLMCall[];
  toolCalls: ToolCall[];
}) {
  const llmById =
    new Map(
      llmCalls.map(
        (call) => [
          call.id,
          call,
        ],
      ),
    );

  const llmByActivityId =
    new Map(
      llmCalls.map(
        (call) => [
          call.activityId,
          call,
        ],
      ),
    );

  const toolById =
    new Map(
      toolCalls.map(
        (call) => [
          call.id,
          call,
        ],
      ),
    );

  const toolByActivityId =
    new Map(
      toolCalls.map(
        (call) => [
          call.activityId,
          call,
        ],
      ),
    );

  const activities =
    [...(agent.activities ??
      [])].sort(
      (a, b) =>
        a.sequence -
        b.sequence,
    );

  return (
    <div className="activity">
      <div className="activity-header">
        <h3>
          Activity
        </h3>

        <span>
          {activities.length}{" "}
          activities
        </span>
      </div>

      <div className="activity-list">
        {activities.map(
          (activity) => {
            if (
              activity.kind ===
              "llm"
            ) {
              const call =
                (activity.llmCallId
                  ? llmById.get(
                      activity.llmCallId,
                    )
                  : undefined) ??
                llmByActivityId.get(
                  activity.id,
                );

              return (
                <LLMActivityRow
                  key={
                    activity.id
                  }
                  activity={
                    activity
                  }
                  call={
                    call
                  }
                />
              );
            }

            const call =
              (activity.toolCallId
                ? toolById.get(
                    activity.toolCallId,
                  )
                : undefined) ??
              toolByActivityId.get(
                activity.id,
              );

            return (
              <ToolActivityRow
                key={
                  activity.id
                }
                activity={
                  activity
                }
                call={
                  call
                }
              />
            );
          },
        )}

        {!activities.length && (
          <div className="empty">
            No activity yet.
          </div>
        )}
      </div>
    </div>
  );
}

// ============================================================
// LLM row
// ============================================================

function LLMActivityRow({
  activity,
  call,
}: {
  activity: AgentActivity;
  call?: LLMCall;
}) {
  return (
    <details className="activity-details llm-call">
      <summary className="activity-row">
        <span className="caret">
          {"\u25B8"}
        </span>

        <span className="activity-time">
          {formatTime(
            activity.startedAt,
          )}
        </span>

        <span className="activity-type">
          LLM #{activity.sequence}
        </span>

        <span className="activity-agent">
          {call?.requestedTool
            ? "tool request"
            : "response"}
        </span>

        <span>
          <RunStatus
            status={
              activity.status
            }
          />
        </span>

        <span className="activity-message">
          {call?.requestedTool
            ? `requested ${call.requestedTool}`
            : call?.response ??
              "LLM call"}
        </span>
      </summary>

      {call && (
        <div className="nested-calls">
          <div className="nested-call">
            <div className="nested-call-header">
              <span>
                LLM #{call.sequence}
              </span>

              <span>
                {call.status}
              </span>
            </div>

            <div className="nested-call-meta">
              <span>
                {formatDurationFromTimestamps(
                  call.startedAt,
                  call.completedAt,
                )}
              </span>

              {call.requestedTool && (
                <span>
                  tool:{" "}
                  {
                    call.requestedTool
                  }
                </span>
              )}
            </div>

            {call.messages &&
              call.messages.length >
                0 && (
                <details>
                  <summary>
                    Messages
                  </summary>

                  <pre>
                    {JSON.stringify(
                      call.messages,
                      null,
                      2,
                    )}
                  </pre>
                </details>
              )}

            {call.response && (
              <details>
                <summary>
                  Response
                </summary>

                <pre>
                  {
                    call.response
                  }
                </pre>
              </details>
            )}

            {call.error && (
              <div className="detail-error">
                {call.error}
              </div>
            )}
          </div>
        </div>
      )}
    </details>
  );
}

// ============================================================
// Tool row
// ============================================================

function ToolActivityRow({
  activity,
  call,
}: {
  activity: AgentActivity;
  call?: ToolCall;
}) {
  return (
    <details className="activity-details tool-call">
      <summary className="activity-row">
        <span className="caret">
          {"\u25B8"}
        </span>

        <span className="activity-time">
          {formatTime(
            activity.startedAt,
          )}
        </span>

        <span className="activity-type">
          TOOL #{activity.sequence}
        </span>

        <span className="activity-tool">
          {call?.toolId ??
            "tool"}
        </span>

        <span>
          <RunStatus
            status={
              activity.status
            }
          />
        </span>

        <span className="activity-message">
          {call
            ? describeToolCall(
                call,
              )
            : "Tool call"}
        </span>
      </summary>

      {call && (
        <div className="nested-calls">
          <div className="nested-call">
            <div className="nested-call-header">
              <span>
                {call.toolId}
              </span>

              <span>
                {call.status}
              </span>
            </div>

            <div className="nested-call-meta">
              <span>
                {formatDurationFromTimestamps(
                  call.startedAt,
                  call.completedAt,
                )}
              </span>
            </div>

            {call.input && (
              <details>
                <summary>
                  Input
                </summary>

                <pre>
                  {JSON.stringify(
                    call.input,
                    null,
                    2,
                  )}
                </pre>
              </details>
            )}

            {call.output && (
              <details>
                <summary>
                  Output
                </summary>

                <pre>
                  {JSON.stringify(
                    call.output,
                    null,
                    2,
                  )}
                </pre>
              </details>
            )}

            {call.error && (
              <div className="detail-error">
                {call.error}
              </div>
            )}
          </div>
        </div>
      )}
    </details>
  );
}

// ============================================================
// Event row
// ============================================================

function EventRow({
  event,
  selected,
  onClick,
}: {
  event: RuntimeEvent;
  selected: boolean;
  onClick: () => void;
}) {
  const clickable =
    Boolean(event.executionID);

  const className = [
    "event-row",
    clickable
      ? "event-clickable"
      : "",
    selected
      ? "event-selected"
      : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={className}
      onClick={
        clickable
          ? onClick
          : undefined
      }
    >
      <span className="event-time">
        {formatTime(
          event.time,
        )}
      </span>

      <span className="event-type">
        {event.type}
      </span>

      <span className="event-node">
        {event.nodeID ?? "-"}
      </span>

      <span className="event-agent">
        {event.agentID ??
          event.workerID ??
          "-"}
      </span>

      <span className="event-tool">
        {event.toolID ?? "-"}
      </span>

      <span className="event-message">
        {event.message ??
          eventMessage(
            event,
          )}
      </span>
    </div>
  );
}

// ============================================================
// Run status
// ============================================================

function RunStatus({
  status,
}: {
  status: string;
}) {
  return (
    <span
      className={`run-status status-${status}`}
    >
      <span className="status-dot" />
      {status}
    </span>
  );
}

// ============================================================
// Graph layout
// ============================================================

function buildGraphLayout(
  nodes: ExecutionNode[],
  edges: ExecutionEdge[],
): GraphLayoutNode[] {
  if (!nodes.length) {
    return [];
  }

  const columns = new Map<
    string,
    number
  >();

  const indegree = new Map<
    string,
    number
  >();

  for (const node of nodes) {
    indegree.set(
      node.id,
      0,
    );
  }

  for (const edge of edges) {
    if (
      indegree.has(
        edge.toExecutionId,
      )
    ) {
      indegree.set(
        edge.toExecutionId,
        (indegree.get(
          edge.toExecutionId,
        ) ?? 0) + 1,
      );
    }
  }

  const queue =
    nodes
      .filter(
        (node) =>
          (indegree.get(
            node.id,
          ) ?? 0) === 0,
      )
      .map(
        (node) =>
          node.id,
      );

  for (const id of queue) {
    columns.set(
      id,
      0,
    );
  }

  while (queue.length) {
    const current =
      queue.shift()!;

    const currentColumn =
      columns.get(
        current,
      ) ?? 0;

    for (const edge of edges) {
      if (
        edge.fromExecutionId !==
        current
      ) {
        continue;
      }

      const next =
        edge.toExecutionId;

      const nextColumn =
        Math.max(
          columns.get(next) ??
            0,
          currentColumn + 1,
        );

      columns.set(
        next,
        nextColumn,
      );

      const remaining =
        (indegree.get(
          next,
        ) ?? 1) - 1;

      indegree.set(
        next,
        remaining,
      );

      if (remaining === 0) {
        queue.push(next);
      }
    }
  }

  // Any execution not reached by the
  // traversal gets placed after the
  // discovered graph.

  let maxColumn =
    Math.max(
      ...columns.values(),
      0,
    );

  for (const node of nodes) {
    if (
      !columns.has(node.id)
    ) {
      columns.set(
        node.id,
        ++maxColumn,
      );
    }
  }

  const byColumn =
    new Map<
      number,
      ExecutionNode[]
    >();

  for (const node of nodes) {
    const column =
      columns.get(
        node.id,
      ) ?? 0;

    const items =
      byColumn.get(
        column,
      ) ?? [];

    items.push(node);

    byColumn.set(
      column,
      items,
    );
  }

  const result:
    GraphLayoutNode[] = [];

  for (const [
    column,
    columnNodes,
  ] of byColumn) {
    columnNodes.forEach(
      (node, index) => {
        result.push({
          id: node.id,

          nodeId:
            node.nodeId,

          status:
            node.status,

          x:
            40 +
            column * 180,

          y:
            40 +
            index * 100,
        });
      },
    );
  }

  return result;
}

// runViewBoxFor computes an SVG viewBox that fits the run graph's
// actual node extents, so the container height follows the graph
// rather than a fixed 540px.
function runViewBoxFor(
  nodes: GraphLayoutNode[],
) {
  if (!nodes.length) {
    return "0 0 900 240";
  }

  const maxX =
    Math.max(
      ...nodes.map(
        (node) =>
          node.x + 100,
      ),
      900,
    );

  const maxY =
    Math.max(
      ...nodes.map(
        (node) =>
          node.y + 64,
      ),
      240,
    );

  return `0 0 ${maxX + 20} ${maxY + 20}`;
}

// ============================================================
// SVG arrow
// ============================================================

function arrowPoints(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
) {
  const angle =
    Math.atan2(
      y2 - y1,
      x2 - x1,
    );

  const length = 8;

  const p1x =
    x2 -
    length *
      Math.cos(
        angle - Math.PI / 6,
      );

  const p1y =
    y2 -
    length *
      Math.sin(
        angle - Math.PI / 6,
      );

  const p2x =
    x2 -
    length *
      Math.cos(
        angle + Math.PI / 6,
      );

  const p2y =
    y2 -
    length *
      Math.sin(
        angle + Math.PI / 6,
      );

  return `${x2},${y2} ${p1x},${p1y} ${p2x},${p2y}`;
}

// ============================================================
// Helpers
// ============================================================

function formatTime(
  value?: string,
) {
  if (!value) {
    return "-";
  }

  return new Date(
    value,
  ).toLocaleTimeString();
}

function formatDate(
  value?: string,
) {
  if (!value) {
    return "-";
  }

  return new Date(
    value,
  ).toLocaleString();
}

function formatDuration(
  startedAt: string,
  completedAt: string,
) {
  const start =
    new Date(
      startedAt,
    ).getTime();

  const end =
    new Date(
      completedAt,
    ).getTime();

  return formatMilliseconds(
    Math.max(
      0,
      end - start,
    ),
  );
}

function formatDurationFromTimestamps(
  startedAt?: string,
  completedAt?: string,
) {
  if (
    !startedAt ||
    !completedAt
  ) {
    return "-";
  }

  return formatDuration(
    startedAt,
    completedAt,
  );
}

function formatMilliseconds(
  milliseconds: number,
) {
  if (
    milliseconds <
    1000
  ) {
    return `${milliseconds} ms`;
  }

  return `${(
    milliseconds / 1000
  ).toFixed(2)} s`;
}

function describeToolCall(
  call: ToolCall,
) {
  if (!call.input) {
    return call.toolId;
  }

  if (
    typeof call.input.path ===
    "string"
  ) {
    return `${call.toolId}(${call.input.path})`;
  }

  if (
    typeof call.input.program ===
    "string"
  ) {
    const args =
      Array.isArray(
        call.input.args,
      )
        ? call.input.args.join(
            " ",
          )
        : "";

    return `${call.input.program} ${args}`;
  }

  return call.toolId;
}

function eventMessage(
  event: RuntimeEvent,
) {
  const data =
    event.data;

  if (!data) {
    return "";
  }

  if (
    typeof data.message ===
    "string"
  ) {
    return data.message;
  }

  if (
    typeof data.tool ===
    "string"
  ) {
    return `tool: ${data.tool}`;
  }

  return "";
}

export default App;