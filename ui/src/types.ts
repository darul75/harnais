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

export interface Run {
  id: string;
  status: RunStatus;
  startedAt: string;
  completedAt?: string | null;
}

export interface GraphNode {
  id: string;
}

export interface GraphEdge {
  id: string;
  from: string;
  to: string;
}

export interface GraphDefinition {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface NodeExecution {
  id: string;
  nodeId: string;
  attempt: number;
  status: NodeStatus;
  input: Record<string, unknown>;
  output: Record<string, unknown>;
  error?: string;
  startedAt: string;
  completedAt?: string | null;
}

export interface GraphEvent {
  id: number;
  time: string;
  runID: string;
  type: string;
  nodeID?: string;
  executionID?: string;
  message?: string;
  data?: Record<string, unknown>;
}

export type WorkflowState = Record<string, unknown>;