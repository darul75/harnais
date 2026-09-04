import type {
  GraphDefinition,
  GraphEvent,
  NodeExecution,
} from "./types";

interface Props {
  graph: GraphDefinition;

  executions: NodeExecution[];

  events: GraphEvent[];

  selectedExecutionId:
    | string
    | null;

  onSelectExecution: (
    execution: NodeExecution,
  ) => void;
}

function latestExecution(
  nodeId: string,
  executions: NodeExecution[],
): NodeExecution | undefined {

  const matches =
    executions.filter(
      (execution) =>
        execution.nodeId === nodeId,
    );

  return matches[matches.length - 1];
}

export function GraphView({
  graph,
  executions,
  selectedExecutionId,
  onSelectExecution,
}: Props) {

  const positions: Record<
    string,
    { x: number; y: number }
  > = {

    planner: {
      x: 350,
      y: 40,
    },

    coder: {
      x: 350,
      y: 160,
    },

    security: {
      x: 600,
      y: 160,
    },

    tester: {
      x: 350,
      y: 280,
    },

    reviewer: {
      x: 500,
      y: 410,
    },
  };

  return (
    <div className="graph-container">

      <svg
        viewBox="0 0 600 400"
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

        {graph.edges.map(
          (edge) => {

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
                x1={from.x + 70}
                y1={from.y + 35}
                x2={to.x + 70}
                y2={to.y}
                stroke="currentColor"
                strokeWidth="2"
                markerEnd="url(#arrow)"
              />
            );
          },
        )}

        {graph.nodes.map(
          (node) => {

            const position =
              positions[node.id] ?? {
                x: 100,
                y: 100,
              };

            const execution =
              latestExecution(
                node.id,
                executions,
              );

            const status =
              execution?.status ??
              "pending";

            const selected =
              execution?.id ===
              selectedExecutionId;

            return (
              <g
                key={node.id}
                transform={
                  `translate(${position.x}, ${position.y})`
                }
                className={
                  selected
                    ? "graph-node-selected"
                    : "graph-node"
                }
                onClick={() => {
                  if (execution) {
                    onSelectExecution(
                      execution,
                    );
                  }
                }}
              >

                <rect
                  width="140"
                  height="56"
                  rx="6"
                  className={
                    `node node-${status}`
                  }
                />

                <text
                  x="70"
                  y="24"
                  textAnchor="middle"
                  className="node-title"
                >
                  {node.id}
                </text>

                <text
                  x="70"
                  y="42"
                  textAnchor="middle"
                  className="node-status"
                >
                  {status}
                  {execution
                    ? ` · attempt ${execution.attempt}`
                    : ""}
                </text>

              </g>
            );
          },
        )}

      </svg>

    </div>
  );
}