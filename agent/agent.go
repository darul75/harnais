package agent

import (
	"context"

	"harnais/graph"
)

type Input struct {
	Message string

	State graph.State
}

type Result struct {
	Output string

	State graph.State
}

type Agent interface {
	ID() string

	Run(
		ctx context.Context,
		input Input,
	) (Result, error)
}
