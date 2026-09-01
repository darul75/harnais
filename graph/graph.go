package graph

import "fmt"

type Graph struct {
	Nodes map[string]*Node
	Edges []Edge
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

func (g *Graph) AddNode(node *Node) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	if node.Worker == nil {
		return fmt.Errorf(
			"node %q has no worker",
			node.ID,
		)
	}

	if _, exists := g.Nodes[node.ID]; exists {
		return fmt.Errorf(
			"node %q already exists",
			node.ID,
		)
	}

	g.Nodes[node.ID] = node

	return nil
}

func (g *Graph) AddEdge(
	from string,
	to string,
) error {
	return g.AddConditionalEdge(
		from,
		to,
		nil,
	)
}

func (g *Graph) AddConditionalEdge(
	from string,
	to string,
	condition func(State) bool,
) error {

	if _, exists := g.Nodes[from]; !exists {
		return fmt.Errorf(
			"source node %q doesn't exist",
			from,
		)
	}

	if _, exists := g.Nodes[to]; !exists {
		return fmt.Errorf(
			"destination node %q doesn't exist",
			to,
		)
	}

	g.Edges = append(
		g.Edges,
		Edge{
			ID: fmt.Sprintf(
				"%s->%s",
				from,
				to,
			),

			From: from,

			To: to,

			Condition: condition,
		},
	)

	return nil
}

func (g *Graph) Outgoing(
	nodeID string,
) []Edge {

	var result []Edge

	for _, edge := range g.Edges {

		if edge.From == nodeID {
			result = append(
				result,
				edge,
			)
		}
	}

	return result
}

func (g *Graph) Incoming(
	nodeID string,
) []Edge {

	var result []Edge

	for _, edge := range g.Edges {

		if edge.To == nodeID {
			result = append(
				result,
				edge,
			)
		}
	}

	return result
}
