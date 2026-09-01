package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"harnais/agent"
)

type OpenAI struct {
	APIKey string

	Model string

	Client *http.Client

	PreviousResponseID string
}

func NewOpenAI(
	apiKey string,
	model string,
) *OpenAI {

	if apiKey == "" {
		apiKey =
			os.Getenv("OPENAI_API_KEY")
	}

	if model == "" {
		model =
			os.Getenv("OPENAI_MODEL")
	}

	return &OpenAI{
		APIKey: apiKey,

		Model: model,

		Client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (o *OpenAI) Validate() error {

	if o.APIKey == "" {
		return fmt.Errorf(
			"OPENAI_API_KEY is not configured",
		)
	}

	if o.Model == "" {
		return fmt.Errorf(
			"OPENAI_MODEL is not configured",
		)
	}

	return nil
}

// ------------------------------------------------------------
// Responses API request
// ------------------------------------------------------------

type responseRequest struct {
	Model string `json:"model"`

	Input []responseInput `json:"input"`

	Tools []responseTool `json:"tools,omitempty"`

	PreviousResponseID string `json:"previous_response_id,omitempty"`
}

type responseInput struct {
	Type string `json:"type,omitempty"`

	Role string `json:"role,omitempty"`

	Content string `json:"content,omitempty"`

	CallID string `json:"call_id,omitempty"`

	Output string `json:"output,omitempty"`
}

type responseTool struct {
	Type string `json:"type"`

	Name string `json:"name"`

	Description string `json:"description"`

	Parameters map[string]any `json:"parameters"`

	Strict bool `json:"strict"`
}

// ------------------------------------------------------------
// Responses API response
// ------------------------------------------------------------

type responseResponse struct {
	ID string `json:"id"`

	// Some providers expose a pre-joined text convenience
	// field. Kept as a fallback only; the canonical source
	// is output[].content[].text.
	OutputText string `json:"output_text"`

	Output []responseOutput `json:"output"`
}

type responseOutput struct {
	Type string `json:"type"`

	ID string `json:"id"`

	CallID string `json:"call_id"`

	Name string `json:"name"`

	Arguments string `json:"arguments"`

	Content []responseContent `json:"content"`
}

type responseContent struct {
	Type string `json:"type"`

	Text string `json:"text"`
}

// ------------------------------------------------------------
// Generate
// ------------------------------------------------------------

func (o *OpenAI) Generate(
	ctx context.Context,
	messages []agent.Message,
	tools []agent.ToolDefinition,
) (agent.LLMResponse, error) {

	if err := o.Validate(); err != nil {
		return agent.LLMResponse{}, err
	}

	request := responseRequest{
		Model: o.Model,

		Tools: make(
			[]responseTool,
			0,
			len(tools),
		),

		Input: make(
			[]responseInput,
			0,
		),

		PreviousResponseID: o.PreviousResponseID,
	}

	// --------------------------------------------------
	// Initial request
	// --------------------------------------------------

	if o.PreviousResponseID == "" {

		if len(messages) == 0 {
			return agent.LLMResponse{}, fmt.Errorf(
				"no messages provided",
			)
		}

		message :=
			messages[0]

		request.Input =
			append(
				request.Input,
				responseInput{
					Type: "message",

					Role: message.Role,

					Content: message.Content,
				},
			)

	} else {

		// --------------------------------------------------
		// Continuation
		//
		// We expect the last message to represent the
		// result of the function/tool call.
		// --------------------------------------------------

		if len(messages) == 0 {
			return agent.LLMResponse{}, fmt.Errorf(
				"no messages provided for continuation",
			)
		}

		message :=
			messages[len(messages)-1]

		if message.CallID == "" {
			return agent.LLMResponse{}, fmt.Errorf(
				"tool result is missing call ID",
			)
		}

		request.Input =
			append(
				request.Input,
				responseInput{
					Type: "function_call_output",

					CallID: message.CallID,

					Output: message.Content,
				},
			)
	}

	// --------------------------------------------------
	// Tools
	// --------------------------------------------------

	for _, tool := range tools {

		request.Tools =
			append(
				request.Tools,
				responseTool{
					Type: "function",

					Name: tool.Name,

					Description: tool.Description,

					Parameters: tool.Parameters,

					Strict: true,
				},
			)
	}

	body, err :=
		json.Marshal(request)

	if err != nil {
		return agent.LLMResponse{}, err
	}

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"https://api.openai.com/v1/responses",
			bytes.NewReader(body),
		)

	if err != nil {
		return agent.LLMResponse{}, err
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+o.APIKey,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err :=
		o.Client.Do(req)

	if err != nil {
		return agent.LLMResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 ||
		resp.StatusCode >= 300 {

		var errorBody struct {
			Error any `json:"error"`
		}

		_ = json.NewDecoder(
			resp.Body,
		).Decode(
			&errorBody,
		)

		return agent.LLMResponse{}, fmt.Errorf(
			"openai API returned %s: %v",
			resp.Status,
			errorBody.Error,
		)
	}

	var result responseResponse

	if err :=
		json.NewDecoder(
			resp.Body,
		).Decode(
			&result,
		); err != nil {

		return agent.LLMResponse{}, err
	}

	// --------------------------------------------------
	// Save response ID.
	// --------------------------------------------------

	o.PreviousResponseID =
		result.ID

	// --------------------------------------------------
	// Function call
	// --------------------------------------------------

	for _, item := range result.Output {

		if item.Type !=
			"function_call" {
			continue
		}

		var arguments map[string]any

		if err :=
			json.Unmarshal(
				[]byte(item.Arguments),
				&arguments,
			); err != nil {

			return agent.LLMResponse{}, fmt.Errorf(
				"invalid tool arguments: %w",
				err,
			)
		}

		return agent.LLMResponse{
			ResponseID: result.ID,

			ToolCall: &agent.ToolCall{
				Name: item.Name,

				Input: arguments,

				CallID: item.CallID,
			},
		}, nil
	}

	return agent.LLMResponse{
		ResponseID: result.ID,

		Text: resultText(result),
	}, nil
}

// resultText assembles the assistant's text from the response.
//
// The Responses API returns text in message output items as
// content parts (type "output_text"), not in a top-level field.
// output_text is only used as a fallback for providers that
// expose it.
func resultText(
	result responseResponse,
) string {

	if result.OutputText != "" {
		return result.OutputText
	}

	var text string

	for _, item := range result.Output {

		if item.Type != "message" {
			continue
		}

		for _, content := range item.Content {

			if content.Type !=
				"output_text" {
				continue
			}

			text +=
				content.Text
		}
	}

	return text
}
