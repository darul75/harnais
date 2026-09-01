package agent

import "harnais/graph"

type Input struct {
	Message string

	State graph.State
}

type Result struct {
	Output string

	State graph.State
}
