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

func TestCleanSearchArtifacts(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unbracketed turn1 search token",
			input: "CNNs are parameter-efficient. (Stanford CS231n). turn1search8",
			want:  "CNNs are parameter-efficient. (Stanford CS231n). ",
		},
		{
			name:  "unbracketed turn1 academia token",
			input: "ResNets underpin vision backbones. turn1academia13",
			want:  "ResNets underpin vision backbones. ",
		},
		{
			name:  "bracketed tokens",
			input: "See [urn0search1] and [turn0search0] for details.",
			want:  "See  and  for details.",
		},
		{
			name:  "cite label",
			input: "Findings cite（ source ）turn1search2 here.",
			want:  "Findings  source  here.",
		},
		{
			name:  "zero-width characters",
			input: "Lead\u200Bing\u200B edge",
			want:  "Leading edge",
		},
		{
			name:  "clean text unchanged",
			input: "A normal sentence with (parens) and numbers 123.",
			want:  "A normal sentence with (parens) and numbers 123.",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := cleanSearchArtifacts(test.input); got != test.want {
				t.Errorf("cleanSearchArtifacts(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}