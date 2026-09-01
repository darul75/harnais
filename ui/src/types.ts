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
  status: RunStatus;
  startedAt: string;
  completedAt?: string;
  state?: Record<string, unknown>;
};

export type Workflow = {
  id: string;
  title: string;
  description: string;
};

export type ExecutionNode = {
  id: string;
  nodeId: string;
  workerId: string;
  attempt: number;
  status: NodeStatus;
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