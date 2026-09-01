import type {
  AgentExecution,
  GraphEvent,
  LLMCall,
  NodeExecution,
  ToolCall,
} from "./types";

interface Props {
  execution:
    | NodeExecution
    | null;

  agentExecutions:
    AgentExecution[];

  llmCalls:
    LLMCall[];

  toolCalls:
    ToolCall[];

  events:
    GraphEvent[];
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

function callDuration(
  startedAt: string,
  completedAt?: string | null,
): string {

  if (!completedAt) {
    return "running";
  }

  return `${
    new Date(completedAt).getTime() -
    new Date(startedAt).getTime()
  } ms`;
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
  agentExecutions,
  llmCalls,
  toolCalls,
  events,
}: Props) {

  if (!execution) {

    return (
      <div className="execution-details empty">
        Select an execution to inspect it.
      </div>
    );
  }

  const agents =
    agentExecutions.filter(
      agent =>
        agent.nodeExecutionId ===
        execution.id,
    );

  const agentIDs =
    new Set(
      agents.map(
        agent => agent.id,
      ),
    );

  const executionLLMs =
    llmCalls.filter(
      call =>
        agentIDs.has(
          call.agentExecutionId,
        ),
    );

  const executionTools =
    toolCalls.filter(
      call =>
        agentIDs.has(
          call.agentExecutionId,
        ),
    );

  const executionEvents =
    events.filter(
      event =>
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
            worker: {execution.workerId}
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
            Triggered by
          </span>

          <strong>
            {execution.triggeredBy.length}
            {" "}activation(s)
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

      {agents.map(
        agent => {

          const agentLLMs =
            executionLLMs.filter(
              call =>
                call.agentExecutionId ===
                agent.id,
            );

          const agentTools =
            executionTools.filter(
              call =>
                call.agentExecutionId ===
                agent.id,
            );

          return (
            <section
              key={agent.id}
              className="agent-execution"
            >

              <div className="activity-header">

                <div>

                  <h3>
                    Agent: {agent.agentId}
                  </h3>

                  <div className="details-subtitle">
                    {agent.id}
                  </div>

                </div>

                <div
                  className={
                    `status status-${agent.status}`
                  }
                >
                  {agent.status}
                </div>

              </div>

              <div className="activity-summary">

                <div className="activity-card">
                  <strong>
                    LLM calls
                  </strong>

                  <span>
                    {agentLLMs.length}
                  </span>
                </div>

                <div className="activity-card">
                  <strong>
                    Tool calls
                  </strong>

                  <span>
                    {agentTools.length}
                  </span>
                </div>

                <div className="activity-card">
                  <strong>
                    Duration
                  </strong>

                  <span>
                    {callDuration(
                      agent.startedAt,
                      agent.completedAt,
                    )}
                  </span>
                </div>

              </div>

              <div className="nested-calls">

                {agentLLMs.map(
                  call => (
                    <div
                      key={call.id}
                      className="nested-call llm-call"
                    >

                      <div className="nested-call-header">

                        <span>
                          LLM #{call.sequence}
                        </span>

                        <span
                          className={
                            `status status-${call.status}`
                          }
                        >
                          {call.status}
                        </span>

                      </div>

                      <div className="nested-call-meta">

                        <span>
                          {callDuration(
                            call.startedAt,
                            call.completedAt,
                          )}
                        </span>

                        {call.requestedTool && (
                          <span>
                            tool →{" "}
                            {call.requestedTool}
                          </span>
                        )}

                      </div>

                      <details>

                        <summary>
                          Messages / response
                        </summary>

                        <pre>
                          {pretty({
                            messages:
                              call.messages,

                            response:
                              call.response,
                          })}
                        </pre>

                      </details>

                    </div>
                  ),
                )}

                {agentTools.map(
                  call => (
                    <div
                      key={call.id}
                      className="nested-call tool-call"
                    >

                      <div className="nested-call-header">

                        <span>
                          Tool #{call.sequence}
                          {" · "}
                          {call.toolId}
                        </span>

                        <span
                          className={
                            `status status-${call.status}`
                          }
                        >
                          {call.status}
                        </span>

                      </div>

                      <div className="nested-call-meta">

                        <span>
                          {callDuration(
                            call.startedAt,
                            call.completedAt,
                          )}
                        </span>

                      </div>

                      <details>

                        <summary>
                          Input / output
                        </summary>

                        <pre>
                          {pretty({
                            input:
                              call.input,

                            output:
                              call.output,
                          })}
                        </pre>

                      </details>

                    </div>
                  ),
                )}

              </div>

            </section>
          );
        },
      )}

      {agents.length === 0 && (
        <div className="empty">
          No agent execution for this node.
        </div>
      )}

      <section className="activity">

        <div className="activity-header">

          <h3>
            Events for this execution
          </h3>

          <span>
            {executionEvents.length}
          </span>

        </div>

        <div className="activity-list">

          {executionEvents.map(
            event => (

              <div
                key={event.id}
                className="activity-row"
              >

                <span className="activity-time">
                  {new Date(
                    event.time,
                  ).toLocaleTimeString()}
                </span>

                <span className="activity-type">
                  {event.type}
                </span>

                <span className="activity-agent">
                  {event.agentID ?? ""}
                </span>

                <span className="activity-tool">
                  {event.toolID ?? ""}
                </span>

                <span className="activity-message">
                  {event.message ?? ""}
                </span>

              </div>
            ),
          )}

        </div>

      </section>

    </div>
  );
}