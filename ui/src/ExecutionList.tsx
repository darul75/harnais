import type {
  NodeExecution,
} from "./types";

interface Props {
  executions: NodeExecution[];
}

function duration(
  execution: NodeExecution,
): string {

  if (!execution.completedAt) {
    return "running";
  }

  const start =
    new Date(
      execution.startedAt,
    ).getTime();

  const end =
    new Date(
      execution.completedAt,
    ).getTime();

  return `${end - start} ms`;
}

export function ExecutionList({
  executions,
}: Props) {

  return (
    <div className="execution-list">

      {executions.map((execution) => (
        <div
          key={execution.id}
          className="execution-row"
        >

          <span className="execution-node">
            {execution.nodeId}
          </span>

          <span>
            #{execution.attempt}
          </span>

          <span
            className={`status status-${execution.status}`}
          >
            {execution.status}
          </span>

          <span>
            {duration(execution)}
          </span>

        </div>
      ))}

    </div>
  );
}