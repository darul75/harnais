export type RunStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed";

export type NodeStatus =
  | "pending"
  | "running"
  | "completed"
  | "failed";

export type AgentActivityKind =
  | "llm"
  | "tool";

export type Run = {
  id: string;
  task?: string;
  workflowId?: string;
  status: RunStatus;
  startedAt: string;
  completedAt?: string;
  state?: Record<string, unknown>;
};

export type RunSummary = {
  id: string;
  task?: string;
  workflowId?: string;
  status: RunStatus;
  startedAt: string;
  completedAt?: string;
};

export type Workflow = {
  id: string;
  title: string;
  description: string;
};

export type WorkflowNodeKind =
  | "agent"
  | "function"
  | "worker";

export type WorkflowNode = {
  id: string;
  kind: WorkflowNodeKind;
  agentId?: string;
  prompt?: string;
  tools?: string[];
  joinAll?: boolean;
};

export type WorkflowEdge = {
  id: string;
  from: string;
  to: string;
  conditional?: boolean;
};

export type WorkflowDetail = {
  id: string;
  title: string;
  description: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
};

export type ExecutionNode = {
  id: string;
  nodeId: string;
  workerId: string;
  attempt: number;
  status: NodeStatus;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  triggeredBy?: string[];
  agent?: AgentExecution;
};

export type ExecutionEdge = {
  id: string;
  fromExecutionId: string;
  toExecutionId: string;
  fromNodeId: string;
  toNodeId: string;
  edgeId: string;
};

export type AgentActivity = {
  id: string;
  agentExecutionId: string;
  sequence: number;
  kind: AgentActivityKind;
  llmCallId?: string;
  toolCallId?: string;
  startedAt: string;
  completedAt?: string;
  status: NodeStatus;
};

export type MessageRecord = {
  role: string;
  content: string;
};

export type LLMCall = {
  id: string;
  agentExecutionId: string;
  activityId: string;
  sequence: number;
  status: NodeStatus;
  messages?: MessageRecord[];
  reasoning?: string;
  response?: string;
  requestedTool?: string;
  startedAt: string;
  completedAt?: string;
  error?: string;
};

export type ToolCall = {
  id: string;
  agentExecutionId: string;
  activityId: string;
  sequence: number;
  toolId: string;
  status: NodeStatus;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  startedAt: string;
  completedAt?: string;
  error?: string;
};

export type AgentExecution = {
  id: string;
  nodeExecutionId: string;
  agentId: string;
  status: NodeStatus;
  startedAt: string;
  completedAt?: string;
  error?: string;
  activities: AgentActivity[];
  llmCalls?: LLMCall[];
  toolCalls?: ToolCall[];
};

export type RunTree = {
  runId: string;
  status: RunStatus;
  startedAt: string;
  completedAt?: string;
  nodes: ExecutionNode[];
  edges: ExecutionEdge[];
};

export type RuntimeEvent = {
  id?: number;
  time: string;
  runID: string;
  type: string;
  nodeID?: string;
  executionID?: string;
  workerID?: string;
  agentID?: string;
  toolID?: string;
  message?: string;
  data?: Record<string, unknown>;
};

export type SettingsFieldType =
  | "secret"
  | "string";

export type SettingsField = {
  key: string;
  label: string;
  type: SettingsFieldType;
  placeholder?: string;
  suggestions?: string[];
  envVar?: string;
  secret?: boolean;
};

export type ProviderSettings = {
  id: string;
  label: string;
  fields: SettingsField[];
  values: Record<string, string>;
  configured: boolean;
};

export type Settings = {
  providers: ProviderSettings[];
  path: string;
};

export type SettingsTestResult = {
  ok: boolean;
  message?: string;
};

export type Report = {
  runId: string;
  name: string;
  size: number;
  modified: string;
};

export type AudioFile = {
  runId: string;
  name: string;
  size: number;
  modified: string;
};

export type AnalyticsOverview = {
  totalRuns: number;
  completedRuns: number;
  failedRuns: number;
  runningRuns: number;
  totalLLMCalls: number;
  completedLLM: number;
  failedLLM: number;
  totalToolCalls: number;
  completedTools: number;
  failedTools: number;
  totalAgents: number;
  completedAgents: number;
  totalEvents: number;
  avgRunDuration: number;
  avgLLMDuration: number;
  avgToolDuration: number;
};

export type LLMCallByRun = {
  runId: string;
  task: string;
  count: number;
  failed: number;
  avgSec: number;
  totalSec: number;
};

export type ToolCallStats = {
  toolId: string;
  count: number;
  completed: number;
  failed: number;
  avgSec: number;
};

export type ToolFailure = {
  runId: string;
  task: string;
  toolId: string;
  error: string;
  startedAt: string;
};

export type LLMFailure = {
  runId: string;
  task: string;
  error: string;
  startedAt: string;
};

export type EdgeActivationStats = {
  edgeId: string;
  fromNode: string;
  toNode: string;
  activated: number;
  consumed: number;
};

export type RunDuration = {
  runId: string;
  task: string;
  status: string;
  durationSec: number;
  startedAt: string;
};

export type Analytics = {
  overview: AnalyticsOverview;
  llmByRun: LLMCallByRun[];
  toolStats: ToolCallStats[];
  toolFailures: ToolFailure[];
  llmFailures: LLMFailure[];
  edgeStats: EdgeActivationStats[];
  runDurations: RunDuration[];
};
