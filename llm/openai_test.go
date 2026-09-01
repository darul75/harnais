package llm

import "testing"

func TestResultTextFromMessageContent(t *testing.T) {
	result := responseResponse{
		ID: "resp_123",

		Output: []responseOutput{
			{
				Type: "function_call",
				Name: "read_file",
			},
			{
				Type: "message",

				Content: []responseContent{
					{
						Type: "output_text",

						Text: "The weather in Paris is sunny.",
					},
				},
			},
		},
	}

	if got := resultText(result); got != "The weather in Paris is sunny." {
		t.Errorf("expected response text, got %q", got)
	}
}

func TestResultTextMultiPart(t *testing.T) {
	result := responseResponse{
		Output: []responseOutput{
			{
				Type: "message",

				Content: []responseContent{
					{
						Type: "output_text",
						Text: "Hello",
					},
					{
						Type: "output_text",
						Text: " world",
					},
				},
			},
		},
	}

	if got := resultText(result); got != "Hello world" {
		t.Errorf("expected joined text, got %q", got)
	}
}

func TestResultTextPrefersOutputText(t *testing.T) {
	result := responseResponse{
		OutputText: "shortcut text",

		Output: []responseOutput{
			{
				Type: "message",

				Content: []responseContent{
					{
						Type: "output_text",
						Text: "content text",
					},
				},
			},
		},
	}

	if got := resultText(result); got != "shortcut text" {
		t.Errorf("expected output_text fallback, got %q", got)
	}
}

func TestResultTextEmpty(t *testing.T) {
	result := responseResponse{}

	if got := resultText(result); got != "" {
		t.Errorf("expected empty text, got %q", got)
	}
}