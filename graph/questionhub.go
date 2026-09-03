package graph

import (
	"context"
	"sync"
)

// QuestionHub coordinates in-process question/answer delivery
// between a worker that is blocked waiting for a user decision and
// the HTTP handler that delivers the user's answer. Keys are scoped
// to a run so concurrent runs never collide.
type QuestionHub struct {
	mu sync.Mutex

	answers map[string]chan [][]string
}

func NewQuestionHub() *QuestionHub {
	return &QuestionHub{
		answers: make(map[string]chan [][]string),
	}
}

// QuestionKey scopes a pending question to a specific run.
func QuestionKey(
	runID string,
	requestID string,
) string {

	return runID + "/" + requestID
}

// Register creates a delivery channel for a pending question and
// returns it along with a cleanup function. The worker calls Wait on
// the returned channel; the HTTP handler delivers via Reply.
func (h *QuestionHub) Register(
	runID string,
	requestID string,
) (<-chan [][]string, func()) {

	key := QuestionKey(runID, requestID)

	channel := make(chan [][]string, 1)

	h.mu.Lock()
	h.answers[key] = channel
	h.mu.Unlock()

	return channel, func() {
		h.Done(runID, requestID)
	}
}

// Wait blocks until an answer is delivered, the run is aborted, or
// the caller cancels. Returns nil if no answer arrived.
func (h *QuestionHub) Wait(
	ctx context.Context,
	runID string,
	requestID string,
) ([][]string, bool) {

	channel := h.channel(runID, requestID)

	if channel == nil {
		return nil, false
	}

	select {

	case answers, ok :=
		<-channel:

		if !ok {
			return nil, false
		}

		return answers, true

	case <-ctx.Done():
		return nil, false
	}
}

// Reply delivers the user's answer to a blocked worker.
func (h *QuestionHub) Reply(
	runID string,
	requestID string,
	answers [][]string,
) bool {

	channel := h.channel(runID, requestID)

	if channel == nil {
		return false
	}

	select {

	case channel <- answers:
		return true

	default:
		return false
	}
}

// Done removes and closes the channel for a question.
func (h *QuestionHub) Done(
	runID string,
	requestID string,
) {

	key := QuestionKey(runID, requestID)

	h.mu.Lock()
	defer h.mu.Unlock()

	channel, ok :=
		h.answers[key]

	if !ok {
		return
	}

	delete(h.answers, key)

	close(channel)
}

func (h *QuestionHub) channel(
	runID string,
	requestID string,
) chan [][]string {

	key := QuestionKey(runID, requestID)

	h.mu.Lock()
	defer h.mu.Unlock()

	return h.answers[key]
}
