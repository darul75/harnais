import type {
  GraphDefinition,
  GraphEvent,
  NodeExecution,
} from "./types";

interface Props {
  graph: GraphDefinition;
  executions: NodeExecution[];
  events: GraphEvent[];
}

function statusForNode(
  nodeId: string,
  executions: NodeExecution[],
): string {
  const nodeExecutions = executions.filter(
    (execution) =>
      execution.nodeId === nodeId,
  );

  if (nodeExecutions.length === 0) {
    return "pending";
  }

  const latest =
    nodeExecutions[nodeExecutions.length - 1];

  return latest.status;
}

function latestAttempt(
  nodeId: string,
  executions: NodeExecution[],
): number {
  const matching = executions.filter(
    (execution) =>
      execution.nodeId === nodeId,
  );

  if (matching.length === 0) {
    return 0;
  }

  return matching[matching.length - 1].attempt;
}

export function GraphView({
  graph,
  executions,
}: Props) {

  const positions: Record<
    string,
    { x: number; y: number }
  > = {
    planner: { x: 350, y: 40 },
    coder: { x: 350, y: 150 },
    tester: { x: 350, y: 260 },
    reviewer: { x: 600, y: 390 },
  };

  return (
    <div className="graph-container">
      <svg
        viewBox="0 0 900 500"
        className="graph-svg"
      >

        <defs>
          <marker
            id="arrow"
            markerWidth="10"
            markerHeight="10"
            refX="8"
            refY="3"
            orient="auto"
          >
            <path
              d="M0,0 L0,6 L9,3 z"
              fill="currentColor"
            />
          </marker>
        </defs>

        {graph.edges.map((edge) => {

          const from =
            positions[edge.from];

          const to =
            positions[edge.to];

          if (!from || !to) {
            return null;
          }

          return (
            <line
              key={edge.id}
              x1={from.x + 90}
              y1={from.y + 45}
              x2={to.x + 90}
              y2={to.y}
              stroke="currentColor"
              strokeWidth="2"
              markerEnd="url(#arrow)"
            />
          );
        })}

        {graph.nodes.map((node) => {

          const position =
            positions[node.id] ?? {
              x: 100,
              y: 100,
            };

          const status =
            statusForNode(
              node.id,
              executions,
            );

          const attempt =
            latestAttempt(
              node.id,
              executions,
            );

          return (
            <g
              key={node.id}
              transform={`translate(${position.x}, ${position.y})`}
            >
              <rect
                width="180"
                height="80"
                rx="10"
                className={`node node-${status}`}
              />

              <text
                x="90"
                y="32"
                textAnchor="middle"
                className="node-title"
              >
                {node.id}
              </text>

              <text
                x="90"
                y="56"
                textAnchor="middle"
                className="node-status"
              >
                {status}
                {attempt > 0
                  ? ` · attempt ${attempt}`
                  : ""}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}