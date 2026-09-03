package opencode

import "testing"

func TestIsSessionError(t *testing.T) {

	if !isSessionError("session.error") {
		t.Fatal("session.error should be treated as a session error")
	}

	if !isSessionError("session.next.step.failed") {
		t.Fatal("session.next.step.failed should be treated as a session error")
	}

	if isSessionError("session.next.text.delta") {
		t.Fatal("session.next.text.delta should not be a session error")
	}
}

func TestSessionErrorMessage(t *testing.T) {

	event := ServerEvent{
		Type: "session.error",
		Properties: map[string]any{
			"error": map[string]any{
				"name": "AI_APICallError",
				"data": map[string]any{
					"message": "Rate limit exceeded. Please try again later.",
				},
			},
		},
	}

	if got := sessionErrorMessage(event); got != "Rate limit exceeded. Please try again later." {
		t.Fatalf("unexpected message %q", got)
	}
}

func TestSessionErrorMessageString(t *testing.T) {

	event := ServerEvent{
		Type: "session.error",
		Properties: map[string]any{
			"error": "boom",
		},
	}

	if got := sessionErrorMessage(event); got != "boom" {
		t.Fatalf("unexpected message %q", got)
	}
}

func TestSessionRetryError(t *testing.T) {

	event := ServerEvent{
		Type: "session.status",
		Properties: map[string]any{
			"status": map[string]any{
				"type":    "retry",
				"attempt": float64(2),
				"message": "Rate limit exceeded. Please try again later.",
				"next":    float64(0),
			},
		},
	}

	want := "opencode provider error (retry 2): Rate limit exceeded. Please try again later."

	if got := sessionRetryError(event); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSessionRetryErrorNonRetryStatus(t *testing.T) {

	event := ServerEvent{
		Type: "session.status",
		Properties: map[string]any{
			"status": map[string]any{
				"type": "busy",
			},
		},
	}

	if got := sessionRetryError(event); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
