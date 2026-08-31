import type {
  GraphDefinition,
  NodeExecution,
  Run,
  WorkflowState,
} from "./types";

const API = "http://localhost:8080/api";

async function get<T>(url: string): Promise<T> {
  const response = await fetch(url);

  if (!response.ok) {
    throw new Error(
      `HTTP ${response.status}: ${response.statusText}`,
    );
  }

  return response.json();
}

export async function createRun(
  state: Record<string, unknown> = {},
): Promise<{ runId: string }> {
  const response = await fetch(`${API}/runs`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      state,
    }),
  });

  if (!response.ok) {
    throw new Error(
      `HTTP ${response.status}: ${response.statusText}`,
    );
  }

  return response.json();
}

export function getRun(runId: string): Promise<Run> {
  return get<Run>(
    `${API}/runs/${runId}`,
  );
}

export function getGraph(
  runId: string,
): Promise<GraphDefinition> {
  return get<GraphDefinition>(
    `${API}/runs/${runId}/graph`,
  );
}

export function getState(
  runId: string,
): Promise<WorkflowState> {
  return get<WorkflowState>(
    `${API}/runs/${runId}/state`,
  );
}

export function getExecutions(
  runId: string,
): Promise<NodeExecution[]> {
  return get<NodeExecution[]>(
    `${API}/runs/${runId}/executions`,
  );
}

export function subscribeToEvents(
  runId: string,
  onEvent: (event: MessageEvent) => void,
  onError?: (event: Event) => void,
): EventSource {
  const source = new EventSource(
    `${API}/runs/${runId}/events`,
  );

  // IMPORTANT:
  // The Go server sends named SSE events:
  //
  //   event: node.started
  //   event: node.completed
  //
  // Therefore source.onmessage does NOT receive them.

  const eventTypes = [
    "run.started",
    "node.started",
    "node.completed",
    "node.failed",
    "edge.activated",
    "run.completed",
    "run.failed",
  ];

  for (const eventType of eventTypes) {
    source.addEventListener(
      eventType,
      onEvent,
    );
  }

  if (onError) {
    source.onerror = onError;
  }

  return source;
}