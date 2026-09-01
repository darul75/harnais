package agent

import "context"

type Tool interface {
	ID() string

	Description() string

	Parameters() map[string]any

	Execute(
		ctx context.Context,
		input map[string]any,
	) (map[string]any, error)
}
