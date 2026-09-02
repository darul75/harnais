import {
  FormEvent,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  ApiError,
  createRun,
  getWorkflow,
  uploadPdf,
} from "./api";

import type {
  RunSummary,
  WorkflowDetail,
  WorkflowEdge,
  WorkflowNode,
} from "./types";

type LayoutNode = {
  id: string;
  nodeId: string;
  x: number;
  y: number;
};

type Props = {
  workflowId: string;

  onRunStarted: (runId: string) => void;

  onRunComplete: (summary: RunSummary) => void;
};

export function WorkflowView({
  workflowId,
  onRunStarted,
  onRunComplete,
}: Props) {
  const [workflow, setWorkflow] =
    useState<WorkflowDetail | null>(
      null,
    );

  const [error, setError] =
    useState<string | null>(null);

  const [loading, setLoading] =
    useState(false);

  const [task, setTask] =
    useState("");

  const [starting, setStarting] =
    useState(false);

  const [pdfFile, setPdfFile] =
    useState<File | null>(null);

  const [selectedNodeId, setSelectedNodeId] =
    useState<string | null>(null);

  // ------------------------------------------------------------
  // Load workflow
  // ------------------------------------------------------------

  useEffect(() => {
    let disposed = false;

    setWorkflow(null);
    setSelectedNodeId(null);
    setError(null);

    getWorkflow(workflowId)
      .then((detail) => {
        if (disposed) {
          return;
        }
        setWorkflow(detail);
      })
      .catch((err) => {
        if (disposed) {
          return;
        }
        console.error(
          "Failed to load workflow",
          err,
        );
        setError(
          err instanceof ApiError &&
            err.status === 404
            ? `Workflow not found: ${workflowId}`
            : err instanceof Error
              ? err.message
              : "Failed to load workflow",
        );
      });

    return () => {
      disposed = true;
    };
  }, [workflowId]);

  // ------------------------------------------------------------
  // Start run
  // ------------------------------------------------------------

  async function handleStart(
    event: FormEvent<HTMLFormElement>,
  ) {
    event.preventDefault();

    const isPDF =
      workflow?.id === "pdf";

    const value =
      task.trim();

    if (
      (isPDF && !pdfFile) ||
      (!isPDF && !value) ||
      starting
    ) {
      return;
    }

    try {
      setStarting(true);
      setError(null);

      let pdfPath: string | undefined;

      if (isPDF && pdfFile) {
        const uploaded =
          await uploadPdf(pdfFile);

        pdfPath = uploaded.path;
      }

      const result =
        await createRun(
          isPDF
            ? value ||
                `Summarize the uploaded PDF (${pdfFile?.name ?? ""})`
            : value,
          workflowId,
          pdfPath,
        );

      onRunStarted(
        result.runId,
      );

      onRunComplete({
        id: result.runId,
        task: result.task,
        workflowId,
        status: "pending",
        startedAt:
          new Date().toISOString(),
      });
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

  if (error) {
    return (
      <section className="panel">
        <div className="error">
          {error}
        </div>
      </section>
    );
  }

  if (!workflow) {
    return (
      <section className="panel">
        <div className="empty">
          Loading workflow...
        </div>
      </section>
    );
  }

  const layout = buildLayout(
    workflow.nodes,
    workflow.edges,
  );

  const selectedNode =
    workflow.nodes.find(
      (node) =>
        node.id ===
        selectedNodeId,
    ) ?? null;

  return (
    <div className="flex flex-col gap-5">
      {/* ------------------------------------------------ */}
      {/* Header */}
      {/* ------------------------------------------------ */}

      <section className="panel">
        <div className="panel-header">
          <h2>
            {workflow.title}
          </h2>

          <span>
            {workflow.nodes.length} nodes
            {" · "}
            {workflow.edges.length} edges
          </span>
        </div>

        <p className="workflow-page-desc">
          {workflow.description}
        </p>

        <div className="details-meta">
          <div>
            <span>
              Workflow ID
            </span>

            <strong>
              {workflow.id}
            </strong>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-3 gap-5 items-start">
        {/* ------------------------------------------------ */}
        {/* Graph */}
        {/* ------------------------------------------------ */}

        <section className="panel col-span-2">
          <div className="panel-header">
            <h2>
              Workflow graph
            </h2>

            <span>
              {workflow.id}
            </span>
          </div>

          <div className="graph-container">
            <svg
              className="graph-svg"
              viewBox={viewBoxFor(
                layout,
              )}
              preserveAspectRatio="xMinYMin meet"
            >
              {/* Edges */}

              {workflow.edges.map(
                (edge) => {
                  const from =
                    layout.find(
                      (node) =>
                        node.id ===
                        edge.from,
                    );

                  const to =
                    layout.find(
                      (node) =>
                        node.id ===
                        edge.to,
                    );

                  if (
                    !from ||
                    !to
                  ) {
                    return null;
                  }

                  const x1 =
                    from.x + 90;

                  const y1 =
                    from.y + 36;

                  const x2 =
                    to.x + 90;

                  const y2 =
                    to.y + 36;

                  return (
                    <g
                      key={edge.id}
                    >
                      <line
                        x1={x1}
                        y1={y1}
                        x2={x2}
                        y2={y2}
                        className={`wf-edge ${
                          edge.conditional
                            ? "wf-edge-conditional"
                            : ""
                        }`}
                      />

                      <polygon
                        points={arrowPoints(
                          x1,
                          y1,
                          x2,
                          y2,
                        )}
                        className="wf-edge-arrow"
                      />
                    </g>
                  );
                },
              )}

              {/* Nodes */}

              {layout.map(
                (node) => {
                  const def =
                    workflow.nodes.find(
                      (item) =>
                        item.id ===
                        node.id,
                    );

                  const selected =
                    node.id ===
                    selectedNodeId;

                  return (
                    <g
                      key={node.id}
                      className={
                        selected
                          ? "graph-node-selected"
                          : "graph-node"
                      }
                      onClick={() =>
                        setSelectedNodeId(
                          node.id,
                        )
                      }
                    >
                      <rect
                        className={`wf-node wf-node-${def?.kind ?? "worker"}`}
                        x={node.x}
                        y={node.y}
                        width="180"
                        height="72"
                        rx="10"
                      />

                      <text
                        className="node-title"
                        x={
                          node.x +
                          90
                        }
                        y={
                          node.y +
                          28
                        }
                        textAnchor="middle"
                      >
                        {node.nodeId}
                      </text>

                      <text
                        className="wf-node-kind"
                        x={
                          node.x +
                          90
                        }
                        y={
                          node.y +
                          50
                        }
                        textAnchor="middle"
                      >
                        {def?.kind ??
                          "worker"}
                        {def?.joinAll
                          ? " · join-all"
                          : ""}
                      </text>
                    </g>
                  );
                },
              )}
            </svg>
          </div>

          {/* Legend */}

          <div className="wf-legend">
            <span className="wf-legend-item">
              <span className="wf-swatch wf-swatch-agent" />
              agent
            </span>

            <span className="wf-legend-item">
              <span className="wf-swatch wf-swatch-function" />
              function
            </span>

            <span className="wf-legend-item">
              <span className="wf-edge-line wf-edge" />
              edge
            </span>

            <span className="wf-legend-item">
              <span className="wf-edge-line wf-edge wf-edge-conditional" />
              conditional
            </span>
          </div>
        </section>

        {/* ------------------------------------------------ */}
        {/* Node details */}
        {/* ------------------------------------------------ */}

        {selectedNode && (
        <section className="panel">
          <div className="panel-header">
            <h2>
              Node
            </h2>

            <span>
              {selectedNode.id}
            </span>
          </div>

          <NodeDetails
            node={selectedNode}
          />
        </section>
        )}
      </div>

      {/* ------------------------------------------------ */}
      {/* Start run */}
      {/* ------------------------------------------------ */}

      <section className="panel">
        <div className="panel-header">
          <h2>
            Run this workflow
          </h2>

          <span>
            {workflow.id}
          </span>
        </div>

        <form
          className="start-form"
          onSubmit={handleStart}
        >
          {workflow.id === "pdf" && (
            <label className="pdf-upload">
              <span className="pdf-upload-label">
                {pdfFile
                  ? pdfFile.name
                  : "Choose a PDF to summarize"}
              </span>

              <input
                type="file"
                accept="application/pdf,.pdf"
                onChange={(event) =>
                  setPdfFile(
                    event.target.files?.[0] ??
                      null,
                  )
                }
                disabled={starting}
              />
            </label>
          )}

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
            placeholder={
              workflow.id === "pdf"
                ? "Optional instructions for the summary..."
                : "Describe the feature, fix, or question..."
            }
            disabled={starting}
          />

          <button
            type="submit"
            className="start-button"
            disabled={
              starting ||
              (workflow.id === "pdf"
                ? !pdfFile
                : !task.trim())
            }
          >
            {starting
              ? "Starting..."
              : workflow.id === "pdf"
                ? "Summarize PDF"
                : "Start Run"}
          </button>
        </form>
      </section>
    </div>
  );
}

// ============================================================
// Node details
// ============================================================

function NodeDetails({
  node,
}: {
  node: WorkflowNode;
}) {
  return (
    <div className="execution-details">
      <div className="details-meta">
        <div>
          <span>
            Kind
          </span>

          <strong>
            {node.kind}
          </strong>
        </div>

        <div>
          <span>
            Worker
          </span>

          <strong>
            {node.agentId ??
              node.id}
          </strong>
        </div>

        <div>
          <span>
            Join all
          </span>

          <strong>
            {node.joinAll
              ? "yes"
              : "no"}
          </strong>
        </div>
      </div>

      {node.tools &&
        node.tools.length >
          0 && (
          <div className="details-grid">
            <div>
              <h3>
                Tools
              </h3>

              <div className="wf-tool-list">
                {node.tools.map(
                  (tool) => (
                    <span
                      key={tool}
                      className="wf-tool-chip"
                    >
                      {tool}
                    </span>
                  ),
                )}
              </div>
            </div>
          </div>
        )}

      {node.prompt && (
        <div className="details-grid">
          <div>
            <h3>
              Prompt
            </h3>

            <details>
              <summary>
                Show prompt
              </summary>

              <pre className="wf-prompt">
                {node.prompt}
              </pre>
            </details>
          </div>
        </div>
      )}
    </div>
  );
}

// ============================================================
// Layout
// ============================================================

function buildLayout(
  nodes: WorkflowNode[],
  edges: WorkflowEdge[],
): LayoutNode[] {
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
    indegree.set(node.id, 0);
  }

  for (const edge of edges) {
    if (
      indegree.has(
        edge.to,
      )
    ) {
      indegree.set(
        edge.to,
        (indegree.get(
          edge.to,
        ) ?? 0) + 1,
      );
    }
  }

  const queue = nodes
    .filter(
      (node) =>
        (indegree.get(
          node.id,
        ) ?? 0) === 0,
    )
    .map(
      (node) => node.id,
    );

  for (const id of queue) {
    columns.set(id, 0);
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
        edge.from !==
        current
      ) {
        continue;
      }

      const next =
        edge.to;

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

      if (
        remaining === 0
      ) {
        queue.push(next);
      }
    }
  }

  let maxColumn =
    Math.max(
      ...columns.values(),
      0,
    );

  for (const node of nodes) {
    if (
      !columns.has(
        node.id,
      )
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
      WorkflowNode[]
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

  const result: LayoutNode[] =
    [];

  for (const [
    column,
    columnNodes,
  ] of byColumn) {
    columnNodes.forEach(
      (node, index) => {
        result.push({
          id: node.id,

          nodeId:
            node.id,

          x:
            40 +
            column * 230,

          y:
            40 +
            index * 100,
        });
      },
    );
  }

  return result;
}

function viewBoxFor(
  nodes: LayoutNode[],
) {
  if (!nodes.length) {
    return "0 0 900 400";
  }

  const maxX =
    Math.max(
      ...nodes.map(
        (node) =>
          node.x + 180,
      ),
      900,
    );

  const maxY =
    Math.max(
      ...nodes.map(
        (node) =>
          node.y + 72,
      ),
      400,
    );

  return `0 0 ${maxX + 20} ${maxY + 20}`;
}

// ============================================================
// Arrow
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
        angle -
          Math.PI / 6,
      );

  const p1y =
    y2 -
    length *
      Math.sin(
        angle -
          Math.PI / 6,
      );

  const p2x =
    x2 -
    length *
      Math.cos(
        angle +
          Math.PI / 6,
      );

  const p2y =
    y2 -
    length *
      Math.sin(
        angle +
          Math.PI / 6,
      );

  return `${x2},${y2} ${p1x},${p1y} ${p2x},${p2y}`;
}
