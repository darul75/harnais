import type {
  GraphEvent,
  NodeExecution,
} from "./types";

interface Props {
  execution:
    | NodeExecution
    | null;

  events: GraphEvent[];
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

function pretty(
  value: unknown,
): string {

  return JSON.stringify(
    value ?? {},
    null,
    2,
  );
}

export function ExecutionDetails({
  execution,
  events,
}: Props) {

  if (!execution) {

    return (
      <div className="execution-details empty">
        Select an execution to inspect it.
      </div>
    );
  }

  const executionEvents =
    events.filter(
      (event) =>
        event.executionID ===
        execution.id,
    );

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
            attempt #{execution.attempt}
          </div>
        </div>

        <div
          className={
            `status status-${execution.status}`
          }
        >
          {execution.status}
        </div>

      </div>

      <div className="details-meta">

        <div>
          <span>
            Execution
          </span>

          <strong>
            {execution.id}
          </strong>
        </div>

        <div>
          <span>
            Duration
          </span>

          <strong>
            {duration(execution)}
          </strong>
        </div>

        <div>
          <span>
            Started
          </span>

          <strong>
            {new Date(
              execution.startedAt,
            ).toLocaleTimeString()}
          </strong>
        </div>

      </div>

      {execution.error && (
        <div className="detail-error">
          {execution.error}
        </div>
      )}

      <div className="details-grid">

        <section>

          <h3>
            Input
          </h3>

          <pre>
            {pretty(
              execution.input,
            )}
          </pre>

        </section>

        <section>

          <h3>
            Output
          </h3>

          <pre>
            {pretty(
              execution.output,
            )}
          </pre>

        </section>

      </div>

      <section className="activity">

        <div className="activity-header">

          <h3>
            Agent activity
          </h3>

          <span>
            {executionEvents.length}
            {" "}events
          </span>

        </div>

        {executionEvents.length === 0 ? (
          <div className="empty">
            Waiting for activity...
          </div>
        ) : (
          <div className="activity-list">

            {executionEvents.map(
              (event) => {

                const time =
                  new Date(
                    event.time,
                  ).toLocaleTimeString();

                return (
                  <div
                    key={event.id}
                    className="activity-row"
                  >

                    <span className="activity-time">
                      {time}
                    </span>

                    <span
                      className={
                        "activity-type"
                      }
                    >
                      {event.type}
                    </span>

                    {event.agentID && (
                      <span className="activity-agent">
                        {event.agentID}
                      </span>
                    )}

                    {event.toolID && (
                      <span className="activity-tool">
                        {event.toolID}
                      </span>
                    )}

                    {event.message && (
                      <span className="activity-message">
                        {event.message}
                      </span>
                    )}

                  </div>
                );
              },
            )}

          </div>
        )}

      </section>

    </div>
  );
}