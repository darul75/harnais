import type {
  NodeExecution,
} from "./types";

interface Props {
  executions: NodeExecution[];

  selectedExecutionId:
    | string
    | null;

  onSelect: (
    execution: NodeExecution,
  ) => void;
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
  selectedExecutionId,
  onSelect,
}: Props) {

  if (executions.length === 0) {
    return (
      <div className="empty">
        No executions yet.
      </div>
    );
  }

  return (
    <div className="execution-list">

      {executions.map(
        (execution) => {

          const selected =
            execution.id ===
            selectedExecutionId;

          return (
            <button
              key={execution.id}
              className={
                `execution-row ${
                  selected
                    ? "execution-selected"
                    : ""
                }`
              }
              onClick={() =>
                onSelect(execution)
              }
            >

              <span className="execution-node">
                {execution.nodeId}
              </span>

              <span className="execution-attempt">
                #{execution.attempt}
              </span>

              <span
                className={
                  `status status-${execution.status}`
                }
              >
                {execution.status}
              </span>

              <span className="execution-duration">
                {duration(execution)}
              </span>

            </button>
          );
        },
      )}

    </div>
  );
}