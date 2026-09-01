package agent

import "context"

type Tool interface {
	ID() string

	Execute(
		ctx context.Context,
		input map[string]any,
	) (map[string]any, error)
}
