import type {
  AudioFile,
  Report,
  Settings,
  SettingsTestResult,
  WorkflowDetail,
  RunSummary,
  Workflow,
} from "./types";

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

export async function getWorkflow(
  workflowId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/workflows/${encodeURIComponent(workflowId)}`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  return (await response.json()) as WorkflowDetail;
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

export async function getSettings() {
  const response = await fetch(
    `${API_BASE}/api/settings`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  return (await response.json()) as Settings;
}

export async function updateSettings(
  providers: Record<
    string,
    Record<string, string>
  >,
) {
  const response = await fetch(
    `${API_BASE}/api/settings`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        providers,
      }),
    },
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  return (await response.json()) as Settings;
}

export async function testSettings(
  provider: string,
  values: Record<string, string>,
) {
  const response = await fetch(
    `${API_BASE}/api/settings/test`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        provider,
        values,
      }),
    },
  );

  const result =
    (await response.json()) as SettingsTestResult;

  if (!response.ok || !result.ok) {
    throw new ApiError(
      result.message ||
        (await response.text()),
      response.status,
    );
  }

  return result;
}

export async function getRunReports(
  runId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}/reports`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  const payload =
    (await response.json()) as {
      reports: Report[];
    };

  return payload.reports;
}

export async function getAllReports() {
  const response = await fetch(
    `${API_BASE}/api/reports`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  const payload =
    (await response.json()) as {
      reports: Report[];
    };

  return payload.reports;
}

export async function getRunReport(
  runId: string,
  name: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}/reports/${encodeURIComponent(name)}`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  return response.text();
}

export async function getRunAudio(
  runId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}/audio`,
  );

  if (!response.ok) {
    throw new ApiError(
      await response.text(),
      response.status,
    );
  }

  const payload =
    (await response.json()) as {
      audio: AudioFile[];
    };

  return payload.audio;
}

export function getAudioUrl(
  runId: string,
  name: string,
) {
  return `${API_BASE}/api/runs/${encodeURIComponent(runId)}/audio/${encodeURIComponent(name)}`;
}