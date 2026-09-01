package server

import (
	"net/http"
	"time"

	"harnais/graph"
)

type executionTreeResponse struct {
	RunID string `json:"runId"`

	Status graph.Status `json:"status"`

	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Nodes []executionTreeNode `json:"nodes"`

	Edges []executionTreeEdge `json:"edges"`
}

type executionTreeNode struct {
	ID string `json:"id"`

	NodeID string `json:"nodeId"`

	WorkerID string `json:"workerId"`

	Attempt int `json:"attempt"`

	Status graph.Status `json:"status"`

	Input graph.State `json:"input"`

	Output graph.State `json:"output"`

	TriggeredBy []string `json:"triggeredBy"`

	Agent *agentTree `json:"agent,omitempty"`
}

type agentTree struct {
	ID string `json:"id"`

	AgentID string `json:"agentId"`

	Status graph.Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`

	Activities []*graph.AgentActivity `json:"activities"`

	LLMCalls []*graph.LLMCall `json:"llmCalls"`

	ToolCalls []*graph.ToolCall `json:"toolCalls"`
}

type executionTreeEdge struct {
	ID string `json:"id"`

	FromExecutionID string `json:"fromExecutionId"`

	ToExecutionID string `json:"toExecutionId"`

	FromNodeID string `json:"fromNodeId"`

	ToNodeID string `json:"toNodeId"`

	EdgeID string `json:"edgeId"`
}

func (s *Server) getExecutionTree(
	w http.ResponseWriter,
	r *http.Request,
) {

	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	// ------------------------------------------------------------
	// Index nested execution data.
	// ------------------------------------------------------------

	agentsByNodeExecution :=
		make(
			map[string][]*graph.AgentExecution,
		)

	for _, agent := range snapshot.AgentExecutions {

		agentsByNodeExecution[agent.NodeExecutionID] =
			append(
				agentsByNodeExecution[agent.NodeExecutionID],
				agent,
			)
	}

	llmByAgent :=
		make(
			map[string][]*graph.LLMCall,
		)

	for _, call := range snapshot.LLMCalls {

		llmByAgent[call.AgentExecutionID] =
			append(
				llmByAgent[call.AgentExecutionID],
				call,
			)
	}

	toolsByAgent :=
		make(
			map[string][]*graph.ToolCall,
		)

	for _, call := range snapshot.ToolCalls {

		toolsByAgent[call.AgentExecutionID] =
			append(
				toolsByAgent[call.AgentExecutionID],
				call,
			)
	}

	// ------------------------------------------------------------
	// Execution nodes.
	// ------------------------------------------------------------

	nodes :=
		make(
			[]executionTreeNode,
			0,
			len(snapshot.Executions),
		)

	for _, execution := range snapshot.Executions {

		node :=
			executionTreeNode{
				ID: execution.ID,

				NodeID: execution.NodeID,

				WorkerID: execution.WorkerID,

				Attempt: execution.Attempt,

				Status: execution.Status,

				Input: execution.Input,

				Output: execution.Output,

				TriggeredBy: append(
					[]string(nil),
					execution.TriggeredBy...,
				),
			}

		agentExecutions :=
			agentsByNodeExecution[execution.ID]

		if len(agentExecutions) > 0 {

			// A node can currently have one agent
			// execution. We keep the array internally
			// but expose the first one for the UI.

			agentExecution :=
				agentExecutions[0]

			node.Agent = &agentTree{
				ID: agentExecution.ID,

				AgentID: agentExecution.AgentID,

				Status: agentExecution.Status,

				StartedAt: agentExecution.StartedAt,

				CompletedAt: agentExecution.CompletedAt,

				Activities: agentExecution.Activities,

				LLMCalls: llmByAgent[agentExecution.ID],

				ToolCalls: toolsByAgent[agentExecution.ID],
			}

			if agentExecution.CompletedAt != nil {

				node.Agent.CompletedAt =
					agentExecution.CompletedAt
			}
		}

		nodes =
			append(
				nodes,
				node,
			)
	}

	// ------------------------------------------------------------
	// Runtime edges.
	// ------------------------------------------------------------

	edges :=
		make(
			[]executionTreeEdge,
			0,
			len(snapshot.EdgeActivations),
		)

	for _, activation := range snapshot.EdgeActivations {

		if activation.ToExecutionID == nil {
			continue
		}

		edges =
			append(
				edges,
				executionTreeEdge{
					ID: activation.ID,

					FromExecutionID: activation.FromExecutionID,

					ToExecutionID: *activation.ToExecutionID,

					FromNodeID: activation.FromNodeID,

					ToNodeID: activation.ToNodeID,

					EdgeID: activation.EdgeID,
				},
			)
	}

	response :=
		executionTreeResponse{
			RunID: snapshot.ID,

			Status: snapshot.Status,

			StartedAt: snapshot.StartedAt,

			Nodes: nodes,

			Edges: edges,
		}

	if snapshot.CompletedAt != nil {

		response.CompletedAt =
			snapshot.CompletedAt
	}

	writeJSON(
		w,
		response,
	)
}
