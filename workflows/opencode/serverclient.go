package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client talks to an external headless `opencode serve` instance over
// HTTP. The server routes requests to the right per-directory
// instance using the `directory` query parameter, so one server can
// serve many harnais workspaces.
type Client struct {
	baseURL string

	http *http.Client
}

func NewClient(
	baseURL string,
) *Client {

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

// modelRef splits a "providerId/modelId" string into the two parts
// the server API expects.
func modelRef(model string) (providerID string, modelID string) {

	if model == "" {
		return "", ""
	}

	parts := strings.SplitN(model, "/", 2)

	if len(parts) == 1 {
		return parts[0], parts[0]
	}

	return parts[0], parts[1]
}

// CreateSession creates a session bound to a workspace directory and
// returns its ID.
func (c *Client) CreateSession(
	ctx context.Context,
	directory string,
	title string,
	agent string,
	model string,
) (string, error) {

	body := map[string]any{
		"title": title,
	}

	if agent != "" {
		body["agent"] = agent
	}

	if model != "" {

		providerID, modelID :=
			modelRef(model)

		body["model"] = map[string]any{
			"id":         modelID,
			"providerID": providerID,
		}
	}

	var session struct {
		ID string `json:"id"`
	}

	if err :=
		c.doJSON(
			ctx,
			http.MethodPost,
			c.baseURL+"/session?directory="+
				url.QueryEscape(directory),
			body,
			&session,
		); err != nil {

		return "", err
	}

	return session.ID, nil
}

// SendMessage sends a prompt to a session and returns the completed
// assistant message info and its parts. The server blocks until the
// session finishes (or needs input such as a question), at which point
// a concurrent SSE subscription and question flow handle it.
func (c *Client) SendMessage(
	ctx context.Context,
	sessionID string,
	directory string,
	agent string,
	model string,
	prompt string,
) (*MessageResult, error) {

	body := map[string]any{
		"parts": []map[string]any{
			{
				"type": "text",
				"text": prompt,
			},
		},
	}

	if model != "" {

		providerID, modelID :=
			modelRef(model)

		body["model"] = map[string]any{
			"providerID": providerID,
			"modelID":    modelID,
		}
	}

	if agent != "" {
		body["agent"] = agent
	}

	response := &MessageResult{}

	if err :=
		c.doJSON(
			ctx,
			http.MethodPost,
			c.baseURL+"/session/"+sessionID+
				"/message?directory="+
				url.QueryEscape(directory),
			body,
			response,
		); err != nil {

		return nil, err
	}

	return response, nil
}

// SessionMessageInfo identifies a message in a session's history.
type SessionMessageInfo struct {
	ID string `json:"id"`

	Role string `json:"role"`
}

// SessionMessage is a message in a session's history.
type SessionMessage struct {
	Info SessionMessageInfo `json:"info"`

	Parts []json.RawMessage `json:"parts"`
}

// SessionMessages fetches the full message history for a session,
// including all parts (text, reasoning, tool calls), used to render
// the agent's conversation live. Uses the v1 endpoint, which returns
// the message array directly (the v2 /api/... route returns empty for
// directory-scoped sessions).
func (c *Client) SessionMessages(
	ctx context.Context,
	sessionID string,
) ([]SessionMessage, error) {

	var messages []SessionMessage

	if err :=
		c.doJSON(
			ctx,
			http.MethodGet,
			c.baseURL+"/session/"+
				sessionID+"/message",
			nil,
			&messages,
		); err != nil {

		return nil, err
	}

	return messages, nil
}

// ReplyQuestion answers a pending question so the blocked run resumes.
// answers is one []string of selected labels per question, in order.
//
// The reply is posted to the directory-scoped v1 endpoint: the pending
// question lives in the per-directory question store, and the v2
// session-scoped endpoint fails to find it (QuestionNotFoundError).
func (c *Client) ReplyQuestion(
	ctx context.Context,
	directory string,
	requestID string,
	answers [][]string,
) error {

	return c.doNoContent(
		ctx,
		http.MethodPost,
		c.baseURL+"/question/"+requestID+
			"/reply?directory="+url.QueryEscape(directory),
		map[string]any{
			"answers": answers,
		},
	)
}

// RejectQuestion rejects a pending question, failing that step.
func (c *Client) RejectQuestion(
	ctx context.Context,
	directory string,
	requestID string,
) error {

	return c.doNoContent(
		ctx,
		http.MethodPost,
		c.baseURL+"/question/"+requestID+
			"/reject?directory="+url.QueryEscape(directory),
		nil,
	)
}

// Abort cancels a running session.
func (c *Client) Abort(
	ctx context.Context,
	sessionID string,
) error {

	return c.doNoContent(
		ctx,
		http.MethodPost,
		c.baseURL+"/session/"+sessionID+"/abort",
		nil,
	)
}

// MessageResult is the response of POST /session/{id}/message.
type MessageResult struct {
	Info struct {
		ID    string `json:"id"`
		Error *struct {
			Name string `json:"name"`

			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		} `json:"error"`
	} `json:"info"`

	Parts json.RawMessage `json:"parts"`
}

// FinalText aggregates the assistant text parts of a message response.
func (m *MessageResult) FinalText() string {

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	if err :=
		json.Unmarshal(m.Parts, &parts); err != nil {

		return ""
	}

	var builder strings.Builder

	for _, part := range parts {
		if part.Type == "text" {
			builder.WriteString(part.Text)
		}
	}

	return builder.String()
}

// ErrorMessage returns a human-readable message from the message info
// error, matching the error objects returned by the server.
func (m *MessageResult) ErrorMessage() string {

	if m.Info.Error == nil {
		return ""
	}

	message :=
		m.Info.Error.Data.Message

	if message == "" {
		message = m.Info.Error.Name
	}

	return strings.TrimSpace(message)
}

// doJSON performs a request and decodes a JSON response body.
func (c *Client) doJSON(
	ctx context.Context,
	method string,
	url string,
	body any,
	out any,
) error {

	request, err :=
		c.newRequest(
			ctx,
			method,
			url,
			body,
		)

	if err != nil {
		return err
	}

	response, err :=
		c.http.Do(request)

	if err != nil {
		return fmt.Errorf(
			"opencode server: %w",
			err,
		)
	}

	defer response.Body.Close()

	data, err :=
		io.ReadAll(response.Body)

	if err != nil {
		return fmt.Errorf(
			"opencode server: read: %w",
			err,
		)
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		return fmt.Errorf(
			"opencode server: %s %s: %s: %s",
			method,
			url,
			response.Status,
			strings.TrimSpace(string(data)),
		)
	}

	if out != nil &&
		len(bytes.TrimSpace(data)) > 0 {

		if err :=
			json.Unmarshal(data, out); err != nil {

			return fmt.Errorf(
				"opencode server: decode: %w",
				err,
			)
		}
	}

	return nil
}

// doNoContent performs a request that expects an empty success body.
func (c *Client) doNoContent(
	ctx context.Context,
	method string,
	url string,
	body any,
) error {

	request, err :=
		c.newRequest(
			ctx,
			method,
			url,
			body,
		)

	if err != nil {
		return err
	}

	response, err :=
		c.http.Do(request)

	if err != nil {
		return fmt.Errorf(
			"opencode server: %w",
			err,
		)
	}

	defer response.Body.Close()

	bodyBytes, _ := io.ReadAll(response.Body)

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		detail :=
			strings.TrimSpace(
				string(bodyBytes),
			)

		if len(detail) > 400 {
			detail = detail[:400]
		}

		return fmt.Errorf(
			"opencode server: %s %s: %s: %s",
			method,
			url,
			response.Status,
			detail,
		)
	}

	return nil
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	url string,
	body any,
) (*http.Request, error) {

	var reader io.Reader

	if body != nil {

		data, err :=
			json.Marshal(body)

		if err != nil {
			return nil, fmt.Errorf(
				"opencode server: encode: %w",
				err,
			)
		}

		reader = bytes.NewReader(data)
	}

	request, err :=
		http.NewRequestWithContext(
			ctx,
			method,
			url,
			reader,
		)

	if err != nil {
		return nil, err
	}

	if body != nil {
		request.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	return request, nil
}

// ------------------------------------------------------------
// Server-Sent Events
// ------------------------------------------------------------

// ServerEvent is a decoded SSE event from the opencode event stream.
type ServerEvent struct {
	Type string `json:"type"`

	Properties map[string]any `json:"properties"`
}

// SubscribeSSE opens the instance event stream for a directory and
// returns decoded events on the returned channel until the context is
// cancelled. This stream carries high-level events (questions, status,
// errors).
func (c *Client) SubscribeSSE(
	ctx context.Context,
	directory string,
) (<-chan ServerEvent, error) {

	streamURL :=
		c.baseURL + "/event?directory=" +
			url.QueryEscape(directory)

	return c.subscribeStream(
		ctx,
		streamURL,
	)
}

func (c *Client) subscribeStream(
	ctx context.Context,
	streamURL string,
) (<-chan ServerEvent, error) {

	request, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			streamURL,
			nil,
		)

	if err != nil {
		return nil, err
	}

	request.Header.Set(
		"Accept",
		"text/event-stream",
	)

	response, err :=
		c.http.Do(request)

	if err != nil {
		return nil, fmt.Errorf(
			"opencode server: event stream: %w",
			err,
		)
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		response.Body.Close()

		return nil, fmt.Errorf(
			"opencode server: event stream: %s",
			response.Status,
		)
	}

	events := make(
		chan ServerEvent,
		256,
	)

	go func() {
		defer close(events)
		defer response.Body.Close()

		scanner :=
			bufio.NewScanner(response.Body)

		scanner.Buffer(
			make([]byte, 0, 64*1024),
			4*1024*1024,
		)

		var data strings.Builder

		for scanner.Scan() {

			line :=
				scanner.Text()

			if line == "" {
				emit :=
					strings.TrimSpace(data.String())

				data.Reset()

				if emit != "" {
					c.dispatch(events, emit)
				}

				continue
			}

			if strings.HasPrefix(line, "data:") {
				data.WriteString(
					strings.TrimPrefix(line, "data:"),
				)

				data.WriteString("\n")
			}
		}
	}()

	return events, nil
}

func (c *Client) dispatch(
	out chan<- ServerEvent,
	raw string,
) {

	var event ServerEvent

	if err :=
		json.Unmarshal([]byte(raw), &event); err != nil {

		return
	}

	select {

	case out <- event:
	default:
	}
}
