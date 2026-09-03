package graph

import (
	"context"
	"testing"
)

// TestQuestionHubDeliversAnswerAfterRegister covers the delivery
// path behind the OpenCode clarifying-question flow: a worker must
// Register a question before an HTTP reply can be delivered to it.
// Previously the worker waited without registering, so the channel
// was nil and Wait returned immediately with no answer.
func TestQuestionHubDeliversAnswerAfterRegister(t *testing.T) {

	hub := NewQuestionHub()

	ch, cleanup := hub.Register("run-1", "que-1")
	defer cleanup()

	if ch == nil {
		t.Fatalf("Register returned nil channel")
	}

	go func() {
		hub.Reply("run-1", "que-1", [][]string{{"A"}})
	}()

	answers, ok := hub.Wait(context.Background(), "run-1", "que-1")
	if !ok {
		t.Fatalf("Wait returned ok=false, want an answer")
	}

	if len(answers) != 1 ||
		len(answers[0]) != 1 ||
		answers[0][0] != "A" {
		t.Fatalf("unexpected answers: %#v", answers)
	}
}

func TestQuestionHubReplyUnknownReturnsFalse(t *testing.T) {

	hub := NewQuestionHub()

	if ok := hub.Reply("run-x", "que-x", [][]string{{"A"}}); ok {
		t.Fatalf("Reply to an unregistered question should return false")
	}
}
