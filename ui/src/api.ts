import type { RunSummary, Workflow } from "./types";

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

const API_BASE =
  import.meta.env.VITE_API_BASE_URL ||
  "http://localhost:8080";

export async function createRun(
  task: string,
  workflowId?: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        task,
        workflowId,
      }),
    },
  );

  if (!response.ok) {
    throw new Error(
      await response.text(),
    );
  }

  return (await response.json()) as {
    runId: string;
    task: string;
  };
}

export async function getWorkflows() {
  const response = await fetch(
    `${API_BASE}/api/workflows`,
  );

  if (!response.ok) {
    throw new Error(
      await response.text(),
    );
  }

  return (await response.json()) as Workflow[];
}

export async function getRun(
  runId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  return response.json();
}

export async function getRuns() {
  const response = await fetch(
    `${API_BASE}/api/runs`,
  );

  if (!response.ok) {
    throw new Error(
      await response.text(),
    );
  }

  return (await response.json()) as RunSummary[];
}

export async function getRunTree(
  runId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}/tree`,
  );

  if (!response.ok) {
    throw new Error(
      await response.text(),
    );
  }

  return response.json();
}

export function createEventSource(
  runId: string,
) {
  return new EventSource(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}/events`,
  );
}