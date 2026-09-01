package workflows

import (
	"sort"

	"harnais/agent"
	"harnais/graph"
	"harnais/opencode"
)

// NodeKind classifies the worker behind a graph node.
type NodeKind string

const (
	NodeAgent    NodeKind = "agent"
	NodeFunction NodeKind = "function"
	NodeWorker   NodeKind = "worker"
)

// NodeInfo describes a single workflow graph node.
type NodeInfo struct {
	ID string

	Kind NodeKind

	// Set when Kind is agent.
	AgentID string

	// Set when Kind is agent and the agent has a prompt.
	Prompt string

	// Tool names, set when Kind is agent.
	Tools []string

	JoinAll bool
}

// EdgeInfo describes a single workflow graph edge.
type EdgeInfo struct {
	ID string

	From string

	To string

	// True when the edge has an associated condition.
	Conditional bool
}

// GraphInfo is a serializable description of a workflow graph.
type GraphInfo struct {
	Nodes []NodeInfo

	Edges []EdgeInfo
}

// Describe builds a serializable description of a workflow graph
// without executing any of its nodes. It reports the node kind and,
// for agent nodes, the agent ID, prompt, and available tools.
func Describe(
	g *graph.Graph,
) GraphInfo {

	info := GraphInfo{
		Nodes: make(
			[]NodeInfo,
			0,
			len(g.Nodes),
		),

		Edges: make(
			[]EdgeInfo,
			0,
			len(g.Edges),
		),
	}

	for id, node := range g.Nodes {

		nodeInfo := NodeInfo{
			ID: id,

			JoinAll: node.JoinAll,
		}

		switch worker := node.Worker.(type) {

		case *agent.LoopAgent:

			nodeInfo.Kind = NodeAgent

			nodeInfo.AgentID = worker.AgentID

			nodeInfo.Prompt = worker.Prompt

			if worker.ToolRegistry != nil {

				definitions :=
					worker.ToolRegistry.Definitions()

				tools := make(
					[]string,
					0,
					len(definitions),
				)

				for _, definition := range definitions {
					tools =
						append(
							tools,
							definition.Name,
						)
				}

				sort.Strings(tools)

				nodeInfo.Tools = tools
			}

		case *opencode.Worker:

			nodeInfo.Kind = NodeAgent

			nodeInfo.AgentID = worker.AgentID

			nodeInfo.Prompt = worker.Prompt

			nodeInfo.Tools = []string{
				"opencode",
			}

		case *graph.FuncWorker:

			nodeInfo.Kind = NodeFunction

			nodeInfo.AgentID = worker.WorkerID

		default:

			nodeInfo.Kind = NodeWorker
		}

		info.Nodes =
			append(
				info.Nodes,
				nodeInfo,
			)
	}

	sort.Slice(
		info.Nodes,
		func(i, j int) bool {
			return info.Nodes[i].ID <
				info.Nodes[j].ID
		},
	)

	for _, edge := range g.Edges {

		info.Edges =
			append(
				info.Edges,
				EdgeInfo{
					ID: edge.ID,

					From: edge.From,

					To: edge.To,

					Conditional: edge.Condition != nil,
				},
			)
	}

	sort.Slice(
		info.Edges,
		func(i, j int) bool {
			return info.Edges[i].ID <
				info.Edges[j].ID
		},
	)

	return info
}
