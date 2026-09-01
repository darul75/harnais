const API_BASE =
  import.meta.env.VITE_API_BASE_URL ||
  "http://localhost:8080";

export async function createRun(
  task: string,
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

export async function getRun(
  runId: string,
) {
  const response = await fetch(
    `${API_BASE}/api/runs/${encodeURIComponent(runId)}`,
  );

  if (!response.ok) {
    throw new Error(
      await response.text(),
    );
  }

  return response.json();
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