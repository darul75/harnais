package graph

import (
	"context"
	"fmt"
)

// FuncWorker adapts an ordinary function into a Worker.
//
// This lets us keep simple nodes such as planner and reviewer,
// while the executor only knows about Worker.
type FuncWorker struct {
	WorkerID string

	Fn func(
		ctx context.Context,
		state State,
	) (State, error)
}

func NewFuncWorker(
	id string,
	fn func(
		ctx context.Context,
		state State,
	) (State, error),
) *FuncWorker {
	return &FuncWorker{
		WorkerID: id,
		Fn:       fn,
	}
}

func (w *FuncWorker) ID() string {
	return w.WorkerID
}

func (w *FuncWorker) Run(
	ctx context.Context,
	input WorkerInput,
) (WorkerResult, error) {

	if w.Fn == nil {
		return WorkerResult{}, fmt.Errorf(
			"worker %q has no function",
			w.WorkerID,
		)
	}

	output, err := w.Fn(
		ctx,
		input.State,
	)

	if err != nil {
		return WorkerResult{}, err
	}

	return WorkerResult{
		State: output,
	}, nil
}
