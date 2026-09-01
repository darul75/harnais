package workflows

import (
	"testing"

	"harnais/graph"
	"harnais/opencode"
)

func TestDescribeOpenCodeCoder(t *testing.T) {

	g :=
		graph.NewGraph()

	if err :=
		g.AddNode(
			&graph.Node{
				ID: "coder",

				Worker: &opencode.Worker{
					AgentID: "opencode-coder",
					Prompt:  "implement it",
				},
			},
		); err != nil {

		t.Fatalf("AddNode: %v", err)
	}

	info :=
		Describe(g)

	if len(info.Nodes) != 1 {
		t.Fatalf(
			"expected 1 node, got %d",
			len(info.Nodes),
		)
	}

	node :=
		info.Nodes[0]

	if node.Kind != NodeAgent {
		t.Errorf(
			"expected agent kind, got %q",
			node.Kind,
		)
	}

	if node.AgentID != "opencode-coder" {
		t.Errorf(
			"expected opencode-coder, got %q",
			node.AgentID,
		)
	}

	if node.Prompt != "implement it" {
		t.Errorf(
			"expected prompt, got %q",
			node.Prompt,
		)
	}

	if len(node.Tools) != 1 ||
		node.Tools[0] != "opencode" {
		t.Errorf(
			"expected [opencode] tools, got %v",
			node.Tools,
		)
	}
}