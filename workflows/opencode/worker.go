// Package opencode integrates the OpenCode headless server as a graph
// Worker.
//
// A Worker drives an external `opencode serve` instance over HTTP,
// creates a session scoped to a workspace directory, sends the prompt,
// streams the events to capture the final assistant text, and returns it
// as the node output state. Clarifying questions OpenCode asks mid-run
// are surfaced to the harnais UI and answered live, resuming the run.
package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"harnais/graph"
)

// Worker is a graph Worker that runs a task through a headless
// OpenCode server (`opencode serve`) over HTTP, listening for
// clarifying questions and surfacing them to the run UI.
type Worker struct {
	// AgentID is the agent identifier shown in the run UI.
	AgentID string

	// Prompt holds the standing instructions given to OpenCode.
	Prompt string

	// Dir is the workspace root OpenCode operates in.
	Dir string

	// Model optionally overrides the model in provider/model format.
	// Empty uses OpenCode's default for the configured provider.
	Model string

	// ReadOnly restricts the server session to read-only operations
	// (no file edits). Used for planning and review phases.
	ReadOnly bool

	// OutputKey is the state key the final assistant text is stored
	// under. Defaults to "output".
	OutputKey string

	// ServerURL is the base URL of the external `opencode serve`
	// instance. Defaults to http://127.0.0.1:4096.
	ServerURL string

	// QuestionHub delivers user answers from the HTTP API back to a
	// worker blocked on a clarifying question. If nil, questions are
	// answered with their first option.
	QuestionHub *graph.QuestionHub

	// Timeout bounds a single run (including time spent waiting for
	// user answers). Defaults to 10 minutes.
	Timeout time.Duration

	// Resume sends the prompt to the session already stored under
	// state["sessionId"] instead of creating a new session. Used to
	// send follow-up messages (e.g. plan revisions) to the same agent.
	Resume bool
}

func (w *Worker) ID() string {

	if w.AgentID != "" {
		return w.AgentID
	}

	return "opencode"
}

func (w *Worker) serverURL() string {

	if w.ServerURL != "" {
		return w.ServerURL
	}

	return "http://127.0.0.1:4096"
}

func (w *Worker) timeout() time.Duration {

	if w.Timeout > 0 {
		return w.Timeout
	}

	return 30 * time.Minute
}

func (w *Worker) agent() string {
	return "build"
}

func (w *Worker) grantedPermissions() map[string]any {

	if w.ReadOnly {
		return readOnlyPermissionRules
	}

	return permissionRules
}

// permissionRules scopes what an OpenCode server session may do.
// Workspace-internal edits/reads are allowed (the workspace is the
// working directory), while external directories, network access,
// subagents, and skills are denied. Shell commands are limited to
// the same development commands harnais exposes to its own agents.
var permissionRules = map[string]any{
	"*":                  "allow",
	"external_directory": "deny",
	"webfetch":           "deny",
	"websearch":          "deny",
	"task":               "deny",
	"skill":              "deny",
	"bash": map[string]any{
		"go *":   "allow",
		"git *":  "allow",
		"npm *":  "allow",
		"node *": "allow",
		"pnpm *": "allow",
		"yarn *": "allow",
		"grep *": "allow",
		"*":      "deny",
	},
}

// readOnlyPermissionRules is like permissionRules but denies all
// file edits, for phases (planning, review) that must not mutate
// the workspace.
var readOnlyPermissionRules = map[string]any{
	"*":                  "deny",
	"read":               "allow",
	"external_directory": "deny",
	"webfetch":           "deny",
	"websearch":          "deny",
	"task":               "deny",
	"skill":              "deny",
	"bash": map[string]any{
		"go *":   "allow",
		"git *":  "allow",
		"npm *":  "allow",
		"node *": "allow",
		"pnpm *": "allow",
		"yarn *": "allow",
		"grep *": "allow",
		"*":      "deny",
	},
}

// ------------------------------------------------------------
// graph.Worker
// ------------------------------------------------------------

func (w *Worker) Run(
	ctx context.Context,
	input graph.WorkerInput,
) (graph.WorkerResult, error) {

	executionContext, ok :=
		graph.GetExecutionContext(ctx)

	if !ok {
		return graph.WorkerResult{}, fmt.Errorf(
			"opencode worker %q: missing execution context",
			w.ID(),
		)
	}

	agentExecution :=
		executionContext.Run.StartAgentExecution(
			executionContext.ExecutionID,
			w.ID(),
		)

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentStarted,

			AgentID: w.ID(),

			Data: map[string]any{
				"agentExecutionId": agentExecution.ID,

				"message": w.Prompt,
			},
		},
	)

	result, err :=
		w.runOpenCode(
			ctx,
			executionContext,
			agentExecution.ID,
			input.State,
		)

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentCompleted,

			AgentID: w.ID(),
		},
	)

	executionContext.Run.CompleteAgentExecution(
		agentExecution.ID,
		err,
	)

	if err != nil {
		return graph.WorkerResult{}, err
	}

	return result, nil
}

// ------------------------------------------------------------
// OpenCode server driver
// ------------------------------------------------------------

func (w *Worker) runOpenCode(
	ctx context.Context,
	executionContext graph.ExecutionContext,
	agentExecutionID string,
	state graph.State,
) (graph.WorkerResult, error) {

	// Resolve the workspace to an absolute path: the server resolves
	// the `directory` parameter against its own cwd, which differs from
	// harnais's, so a relative path would point at the wrong location.
	dir, err :=
		filepath.Abs(w.Dir)

	if err != nil {
		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: resolve workspace: %w",
			err,
		)
	}

	if err :=
		os.MkdirAll(
			dir,
			0o755,
		); err != nil {

		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: create workspace: %w",
			err,
		)
	}

	timeoutCtx, cancel :=
		context.WithTimeout(
			ctx,
			w.timeout(),
		)

	defer cancel()

	client :=
		NewClient(
			w.serverURL(),
		)

	prompt :=
		w.buildPrompt(state)

	recorder :=
		newActivityRecorder(
			ctx,
			executionContext.Run,
			w.ID(),
			agentExecutionID,
			prompt,
		)

	title :=
		fmt.Sprintf(
			"harnais/%s/%s",
			executionContext.RunID,
			executionContext.NodeID,
		)

	sessionID := ""

	if w.Resume {

		sessionID, _ =
			state["sessionId"].(string)

		if sessionID == "" {
			return graph.WorkerResult{}, fmt.Errorf(
				"opencode: resume requires a sessionId in state",
			)
		}
	} else {

		sessionID, err =
			client.CreateSession(
				timeoutCtx,
				dir,
				title,
				w.agent(),
				w.Model,
			)

		if err != nil {
			return graph.WorkerResult{}, fmt.Errorf(
				"opencode: create session: %w",
				err,
			)
		}
	}

	recorder.setSessionID(sessionID)

	// ------------------------------------------------------
	// Stream server events.
	//
	// The directory stream carries high-level control events
	// (questions, status, errors). Live progress is captured by
	// polling the session message history.
	// ------------------------------------------------------

	eventCtx, cancelEvents :=
		context.WithCancel(ctx)

	defer cancelEvents()

	events, err :=
		client.SubscribeSSE(
			eventCtx,
			dir,
		)

	if err != nil {
		// Non-fatal: the message request still runs.
		events = nil
	}

	// pollHistory fetches the session's message history and reflects
	// it into the activity records (live text, reasoning, tools).
	pollHistory := func() {

		pollCtx, cancelPoll :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)

		defer cancelPoll()

		messages, err :=
			client.SessionMessages(
				pollCtx,
				sessionID,
			)

		if err != nil {
			return
		}

		recorder.syncMessages(messages)
	}

	// ------------------------------------------------------
	// Send the prompt. The server blocks until the run finishes
	// (or needs input such as a question).
	// ------------------------------------------------------

	type outcome struct {
		result *MessageResult

		err error
	}

	done :=
		make(chan outcome, 1)

	go func() {
		result, sendErr :=
			client.SendMessage(
				timeoutCtx,
				sessionID,
				dir,
				w.agent(),
				w.Model,
				prompt,
			)

		done <- outcome{
			result: result,

			err: sendErr,
		}
	}()

	// Track surfaced questions so a replayed event isn't shown twice.
	surfaced :=
		map[string]bool{}

	var (
		result *MessageResult

		runErr error

		clarifications []Clarification
	)

	// handleControl reacts to questions, session errors, and provider
	// retries carried by either stream. It returns a run-ending error
	// (or nil) that breaks the loop.
	handleControl := func(event ServerEvent) error {

		if msg :=
			sessionRetryError(event); msg != "" {

			// The provider failed and opencode is retrying (e.g. a
			// rate limit). Surface it promptly instead of leaving the
			// run silently stuck for the retry duration.
			return fmt.Errorf(
				"%s",
				msg,
			)
		}

		if isSessionError(event.Type) {

			// The server reported a session error (e.g. a provider
			// failure or rate limit). Fail promptly so the run
			// surfaces the message instead of hanging.
			return fmt.Errorf(
				"opencode session error: %s",
				sessionErrorMessage(event),
			)
		}

		if isQuestionEvent(event.Type) {

			if question :=
				questionFromEvent(event); question != nil &&
				!surfaced[question.RequestID] {

				surfaced[question.RequestID] = true

				cls, err :=
					w.handleQuestion(
						timeoutCtx,
						client,
						executionContext.RunID,
						dir,
						question,
					)

				if err != nil {
					return err
				}

				clarifications =
					append(
						clarifications,
						cls...,
					)
			}
		}

		return nil
	}

	pollTicker :=
		time.NewTicker(1500 * time.Millisecond)

	defer pollTicker.Stop()

loop:
	for {

		select {

		case <-timeoutCtx.Done():

			cancelEvents()

			_ = client.Abort(
				context.Background(),
				sessionID,
			)

			return graph.WorkerResult{}, fmt.Errorf(
				"opencode: %w",
				timeoutCtx.Err(),
			)

		case event, ok :=
			<-events:

			// Directory stream: control events only.
			if !ok {
				events = nil

				continue
			}

			if err :=
				handleControl(event); err != nil {

				runErr = err

				break loop
			}

		case <-pollTicker.C:

			// Live progress: poll the session history and record the
			// assistant text, reasoning, and tool calls incrementally.
			pollHistory()

		case sendResult :=
			<-done:

			result = sendResult.result
			runErr = sendResult.err

			break loop
		}
	}

	cancelEvents()

	// Final history poll + finalize the recorded activities.
	pollHistory()

	recorder.finalize()

	// Fallback: if polling captured no activities (e.g. the history
	// fetch failed throughout), derive them from the completed message
	// parts so the UI still shows the LLM response and tool calls.
	if result != nil &&
		!recorder.hasRecorded() {

		recorder.recordParts(
			result.Parts,
		)
	}

	if runErr != nil {
		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: %w",
			runErr,
		)
	}

	// Surface any questions still pending in the final message.
	if result != nil {

		if message :=
			result.ErrorMessage(); message != "" {

			return graph.WorkerResult{}, fmt.Errorf(
				"opencode: %s",
				message,
			)
		}

		outputKey :=
			w.OutputKey

		if outputKey == "" {
			outputKey = "output"
		}

		text :=
			result.FinalText()

		return graph.WorkerResult{
			State: graph.State{
				outputKey: text,

				"sessionId": sessionID,

				"clarifications": clarifications,
			},
		}, nil
	}

	return graph.WorkerResult{}, fmt.Errorf(
		"opencode: no response from server",
	)
}

// handleQuestion surfaces a pending question to the run UI and waits
// for the user's answer, then resumes (or rejects) the opencode run.
// The recorded clarifications are returned so the run can carry them
// forward to downstream agents.
func (w *Worker) handleQuestion(
	ctx context.Context,
	client *Client,
	runID string,
	directory string,
	question *Question,
) ([]Clarification, error) {

	var (
		answers [][]string

		ch      <-chan [][]string
		cleanup func()
	)

	if w.QuestionHub != nil {

		ch, cleanup =
			w.QuestionHub.Register(
				runID,
				question.RequestID,
			)
	}

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentQuestion,

			AgentID: w.ID(),

			Data: map[string]any{
				"requestId": question.RequestID,

				"sessionId": question.SessionID,

				"questions": question.Questions,
			},
		},
	)

	if w.QuestionHub != nil {

		// Register happens before the event is emitted, so an answer
		// that arrives the moment the question appears is not lost.
		select {
		case <-ctx.Done():
		case got, ok := <-ch:
			if ok {
				answers = got
			}
		}

		if cleanup != nil {
			cleanup()
		}
	}

	if len(answers) !=
		len(question.Questions) {

		// Either no hub (auto mode) or Wait was cancelled.
		// Auto-answer by picking the first option for every
		// question that offers one.
		answers =
			make([][]string, 0, len(question.Questions))

		for _, q := range question.Questions {

			if len(q.Options) == 0 {
				continue
			}

			answers =
				append(
					answers,
					[]string{q.Options[0].Label},
				)
		}
	}

	if len(answers) !=
		len(question.Questions) {

		// Not every question can be answered (no options and
		// no user reply) - reject so the step fails cleanly.
		_ = client.RejectQuestion(
			context.Background(),
			directory,
			question.RequestID,
		)

		return nil, fmt.Errorf(
			"opencode question %s timed out",
			question.RequestID,
		)
	}

	graph.EmitEvent(
		ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventAgentQuestionAnswer,

			AgentID: w.ID(),

			Data: map[string]any{
				"requestId": question.RequestID,

				"answers": answers,
			},
		},
	)

	if err := client.ReplyQuestion(
		context.Background(),
		directory,
		question.RequestID,
		answers,
	); err != nil {
		return nil, err
	}

	clarifications :=
		make([]Clarification, 0, len(question.Questions))

	for i, q := range question.Questions {

		if i >= len(answers) {
			break
		}

		clarifications =
			append(
				clarifications,
				Clarification{
					Question: q.Question,

					Header: q.Header,

					Answers: answers[i],
				},
			)
	}

	return clarifications, nil
}

// isQuestionEvent reports whether an event type represents a pending
// clarifying question from the agent.
func isQuestionEvent(
	eventType string,
) bool {

	switch eventType {
	case "question.v2.asked",
		"question.asked":
		return true

	default:
		return false
	}
}

// sessionRetryError returns a formatted provider-error message when the
// event is a session.status retry event (emitted while opencode retries
// a failed provider call, e.g. a rate limit), otherwise "".
func sessionRetryError(
	event ServerEvent,
) string {

	if event.Type != "session.status" {
		return ""
	}

	status, _ :=
		event.Properties["status"].(map[string]any)

	if status == nil ||
		status["type"] != "retry" {
		return ""
	}

	message, _ :=
		status["message"].(string)

	if message == "" {
		message = "provider error"
	}

	attempt, _ :=
		status["attempt"].(float64)

	return fmt.Sprintf(
		"opencode provider error (retry %d): %s",
		int(attempt),
		message,
	)
}

// isSessionError reports whether an event type signals a failed
// session or step (e.g. a provider error or rate limit).
func isSessionError(
	eventType string,
) bool {

	switch eventType {
	case "session.error",
		"session.next.step.failed":
		return true

	default:
		return false
	}
}

// sessionErrorMessage extracts a readable message from a session
// error event's properties.
func sessionErrorMessage(
	event ServerEvent,
) string {

	raw, ok :=
		event.Properties["error"]

	if !ok {
		return "unknown error"
	}

	switch value := raw.(type) {

	case string:
		if value != "" {
			return value
		}

	case map[string]any:

		if message, ok :=
			value["message"].(string); ok &&
			message != "" {

			return message
		}

		if data, ok :=
			value["data"].(map[string]any); ok {

			if message, ok :=
				data["message"].(string); ok &&
				message != "" {

				return message
			}
		}

		if name, ok :=
			value["name"].(string); ok &&
			name != "" {

			return name
		}
	}

	return "unknown error"
}

// Question is a pending clarifying question extracted from an event.
type Question struct {
	RequestID string

	SessionID string

	Questions []QuestionInfo
}

// Clarification records a user's answer to a clarifying question so a
// downstream agent (e.g. the coder) receives the decision instead of
// re-asking.
type Clarification struct {
	Question string `json:"question"`

	Header string `json:"header"`

	Answers []string `json:"answers"`
}

// QuestionInfo is a single ask within a pending question request.
type QuestionInfo struct {
	Question string `json:"question"`

	Header string `json:"header"`

	Options []QuestionOption `json:"options"`

	Multiple bool `json:"multiple"`

	Custom bool `json:"custom"`
}

// QuestionOption is an answer choice for a question.
type QuestionOption struct {
	Label string `json:"label"`

	Description string `json:"description"`
}

// questionFromEvent extracts a pending question from a server event.
func questionFromEvent(
	event ServerEvent,
) *Question {

	props :=
		event.Properties

	if props == nil {
		return nil
	}

	requestID, ok :=
		props["id"].(string)

	if !ok {
		return nil
	}

	sessionID, _ :=
		props["sessionID"].(string)

	raw, err :=
		json.Marshal(props["questions"])

	if err != nil {
		return nil
	}

	var questions []QuestionInfo

	if err :=
		json.Unmarshal(raw, &questions); err != nil {

		return nil
	}

	return &Question{
		RequestID: requestID,

		SessionID: sessionID,

		Questions: questions,
	}
}

// buildPrompt combines the standing instructions with the run task.
func (w *Worker) buildPrompt(
	state graph.State,
) string {

	var builder strings.Builder

	builder.WriteString(
		w.Prompt,
	)

	builder.WriteString(
		"\n\n",
	)

	if task, ok :=
		state["task"].(string); ok &&
		task != "" {

		fmt.Fprintf(
			&builder,
			"User request: %s\n",
			task,
		)
	}

	if plan, ok :=
		state["plan"].(string); ok &&
		plan != "" {

		fmt.Fprintf(
			&builder,
			"Plan: %s\n",
			plan,
		)
	}

	if testOutput, ok :=
		state["test_output"].(string); ok &&
		testOutput != "" {

		fmt.Fprintf(
			&builder,
			"Test output: %s\n",
			testOutput,
		)
	}

	if feedback, ok :=
		state["review_feedback"].(string); ok &&
		feedback != "" {

		fmt.Fprintf(
			&builder,
			"Previous review feedback: %s\n",
			feedback,
		)
	}

	if feedback, ok :=
		state["plan_feedback"].(string); ok &&
		feedback != "" {

		fmt.Fprintf(
			&builder,
			"User requested changes to the plan: %s\n",
			feedback,
		)
	}

	if raw, ok :=
		state["clarifications"].([]Clarification); ok &&
		len(raw) > 0 {

		builder.WriteString(
			"\nUser clarifications (already answered - do not ask again):\n",
		)

		for _, c := range raw {

			builder.WriteString("- ")

			if c.Header != "" {
				fmt.Fprintf(
					&builder,
					"%s: ",
					c.Header,
				)
			}

			if c.Question != "" {
				builder.WriteString(
					c.Question,
				)
			}

			builder.WriteString(
				" -> ",
			)

			builder.WriteString(
				strings.Join(
					c.Answers,
					", ",
				),
			)

			builder.WriteString(
				"\n",
			)
		}
	}

	return builder.String()
}

// ------------------------------------------------------------
// Activity recorder
// ------------------------------------------------------------
//
// activityRecorder mirrors the OpenCode JSON event stream into the
// graph's activity/LLM/tool records so the run UI shows the coder's
// steps, exactly like the built-in LoopAgent does.

type toolRecord struct {
	activityID string

	toolCallID string

	name string
}

type activityRecorder struct {
	ctx context.Context

	run *graph.Run

	agentID string

	agentExecutionID string

	prompt string

	// Shared activity sequence across LLM and tool activities.
	activitySeq int

	llmSeq int

	toolSeq int

	// Current LLM step (one per OpenCode step).
	stepStarted    bool
	stepActivityID string
	stepLLMCallID  string
	stepText       strings.Builder
	stepTool       string
	steps          int

	tools map[string]toolRecord

	// partsSeen tracks message part IDs already reflected into the
	// activity records so history polling is incremental.
	partsSeen map[string]bool

	// llmByMessage maps assistant message IDs to their live LLM state.
	llmByMessage map[string]*llmState

	// sessionID is the OpenCode session reported by the process.
	sessionID string

	// errorMessage captures the most recent OpenCode error event so
	// it can be surfaced when the process exits non-zero (OpenCode
	// writes API failures to stdout as JSON, not stderr).
	errorMessage string

	// serverText accumulates assistant text deltas from the headless
	// server's SSE stream.
	serverText strings.Builder

	textByPart map[string]string

	textOrder []string
}

func newActivityRecorder(
	ctx context.Context,
	run *graph.Run,
	agentID string,
	agentExecutionID string,
	prompt string,
) *activityRecorder {

	return &activityRecorder{
		ctx:              ctx,
		run:              run,
		agentID:          agentID,
		agentExecutionID: agentExecutionID,
		prompt:           prompt,
		tools:            map[string]toolRecord{},
		partsSeen:        map[string]bool{},
		llmByMessage:     map[string]*llmState{},
		textByPart:       map[string]string{},
		textOrder:        []string{},
	}
}

// process handles a single JSON event line from `opencode run`.
func (r *activityRecorder) process(
	line []byte,
) {

	var event struct {
		Type      string `json:"type"`
		SessionID string `json:"sessionID"`

		Error *struct {
			Message string `json:"message"`
			Data    struct {
				Message string `json:"message"`
			} `json:"data"`
		} `json:"error"`

		Part struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Text   string `json:"text"`
			Tool   string `json:"tool"`
			CallID string `json:"callID"`

			State *struct {
				Status string         `json:"status"`
				Input  map[string]any `json:"input"`
				Output string         `json:"output"`
			} `json:"state"`
		} `json:"part"`
	}

	if err :=
		json.Unmarshal(
			line,
			&event,
		); err != nil {

		return
	}

	if event.SessionID != "" {
		r.sessionID = event.SessionID
	}

	switch event.Type {

	case "error":
		if event.Error == nil {
			return
		}

		message :=
			event.Error.Data.Message

		if message == "" {
			message =
				event.Error.Message
		}

		if message != "" {
			r.errorMessage =
				strings.TrimSpace(message)
		}

	case "step_start":
		r.startStep()

	case "step_finish":
		r.finishStep()

	case "text":
		if event.Part.Type != "text" {
			return
		}

		r.appendText(event.Part.Text)
		r.accumulatePart(
			event.Part.ID,
			event.Part.Text,
		)

	case "tool_use":
		if event.Part.State == nil {
			return
		}

		r.toolEvent(
			event.Part.Tool,
			event.Part.CallID,
			event.Part.State,
		)
	}
}

func (r *activityRecorder) setSessionID(
	sessionID string,
) {

	if sessionID != "" {
		r.sessionID = sessionID
	}
}

// processServerEvent mirrors a headless-server SSE event into the
// graph activity records.
func (r *activityRecorder) processServerEvent(
	event ServerEvent,
) {

	switch event.Type {

	case "session.next.step.started":
		r.startStep()

	case "session.next.step.ended":
		r.finishStep()

	case "session.next.text.delta":
		delta, _ :=
			event.Properties["delta"].(string)

		if delta == "" {
			return
		}

		r.appendText(delta)
		r.serverText.WriteString(delta)

	case "session.next.tool.called":
		name, _ :=
			event.Properties["tool"].(string)

		callID, _ :=
			event.Properties["callID"].(string)

		input, _ :=
			event.Properties["input"].(map[string]any)

		r.startTool(
			name,
			callID,
			input,
		)

	case "session.next.tool.success":
		callID, _ :=
			event.Properties["callID"].(string)

		r.finishTool(
			callID,
			toolOutput(event.Properties),
			nil,
		)

	case "session.next.tool.failed":
		callID, _ :=
			event.Properties["callID"].(string)

		r.finishTool(
			callID,
			"",
			fmt.Errorf(
				"opencode tool failed",
			),
		)
	}
}

// toolOutput extracts a readable output string from a tool success
// event.
func toolOutput(
	props map[string]any,
) string {

	if props == nil {
		return ""
	}

	switch value :=
		props["content"].(type) {

	case []any:
		return fmt.Sprintf(
			"%v",
			value,
		)
	}

	if result, ok :=
		props["result"]; ok {

		return fmt.Sprintf(
			"%v",
			result,
		)
	}

	return ""
}

func (r *activityRecorder) startStep() {

	// Guard against a duplicate step-started (e.g. the same event
	// delivered on both the directory and session streams).
	if r.stepStarted {
		return
	}

	r.steps++

	messages :=
		[]graph.MessageRecord(nil)

	if r.steps == 1 &&
		r.prompt != "" {

		messages =
			[]graph.MessageRecord{
				{
					Role:    "user",
					Content: r.prompt,
				},
			}
	}

	r.activitySeq++

	activity :=
		r.run.StartAgentActivity(
			r.agentExecutionID,
			r.activitySeq,
			graph.ActivityLLM,
		)

	sequence := r.llmSeq
	r.llmSeq++

	call :=
		r.run.StartLLMCall(
			r.agentExecutionID,
			activity.ID,
			sequence,
			messages,
		)

	r.stepStarted = true
	r.stepActivityID = activity.ID
	r.stepLLMCallID = call.ID
	r.stepText.Reset()
	r.stepTool = ""

	graph.EmitEvent(
		r.ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventLLMStarted,

			AgentID: r.agentID,

			Data: map[string]any{
				"agentExecutionId": r.agentExecutionID,

				"activityId": activity.ID,

				"llmCallId": call.ID,

				"sequence": sequence,
			},
		},
	)
}

func (r *activityRecorder) appendText(
	text string,
) {

	if !r.stepStarted {
		return
	}

	r.stepText.WriteString(text)
}

func (r *activityRecorder) finishStep() {

	if !r.stepStarted {
		return
	}

	r.run.CompleteLLMCall(
		r.stepLLMCallID,
		r.stepText.String(),
		r.stepTool,
		nil,
	)

	r.run.CompleteAgentActivity(
		r.stepActivityID,
		nil,
	)

	graph.EmitEvent(
		r.ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventLLMCompleted,

			AgentID: r.agentID,
		},
	)

	r.stepStarted = false
}

func (r *activityRecorder) toolEvent(
	name string,
	callID string,
	state *struct {
		Status string         `json:"status"`
		Input  map[string]any `json:"input"`
		Output string         `json:"output"`
	},
) {

	if callID == "" {
		return
	}

	if _, seen :=
		r.tools[callID]; !seen {

		r.startTool(
			name,
			callID,
			state.Input,
		)
	}

	switch state.Status {

	case "completed",
		"success",
		"done":

		r.finishTool(
			callID,
			state.Output,
			nil,
		)

	case "error",
		"cancelled",
		"failed":

		r.finishTool(
			callID,
			state.Output,
			fmt.Errorf(
				"opencode tool %s: %s",
				name,
				state.Status,
			),
		)
	}
}

func (r *activityRecorder) startTool(
	name string,
	callID string,
	input map[string]any,
) {

	// Guard against a duplicate tool-started for the same call.
	if _, ok :=
		r.tools[callID]; ok {
		return
	}

	r.activitySeq++

	activity :=
		r.run.StartAgentActivity(
			r.agentExecutionID,
			r.activitySeq,
			graph.ActivityTool,
		)

	sequence := r.toolSeq
	r.toolSeq++

	call :=
		r.run.StartToolCall(
			r.agentExecutionID,
			activity.ID,
			sequence,
			name,
			input,
		)

	r.tools[callID] =
		toolRecord{
			activityID: activity.ID,

			toolCallID: call.ID,

			name: name,
		}

	r.stepTool = name

	graph.EmitEvent(
		r.ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventToolStarted,

			AgentID: r.agentID,

			ToolID: name,

			Data: map[string]any{
				"agentExecutionId": r.agentExecutionID,

				"activityId": activity.ID,

				"toolCallId": call.ID,

				"sequence": sequence,
			},
		},
	)
}

func (r *activityRecorder) finishTool(
	callID string,
	output string,
	err error,
) {

	record, ok :=
		r.tools[callID]

	if !ok {
		return
	}

	result :=
		map[string]any{}

	if output != "" {
		result["output"] = output
	}

	r.run.CompleteToolCall(
		record.toolCallID,
		result,
		err,
	)

	r.run.CompleteAgentActivity(
		record.activityID,
		err,
	)

	delete(r.tools, callID)

	graph.EmitEvent(
		r.ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventToolCompleted,

			AgentID: r.agentID,

			ToolID: record.name,
		},
	)
}

// hasRecorded reports whether any activity was recorded from the live
// SSE stream.
func (r *activityRecorder) hasRecorded() bool {

	return r.activitySeq > 0
}

// messagePart is a decoded part of a completed assistant message.
type messagePart struct {
	ID string `json:"id"`

	Type string `json:"type"`

	Text string `json:"text"`

	Synthetic bool `json:"synthetic"`

	Tool string `json:"tool"`

	CallID string `json:"callID"`

	// State is kept as a generic map so a tool part never fails to
	// decode regardless of how the provider shapes input/output.
	State map[string]any `json:"state"`
}

func partStatus(state map[string]any) string {
	value, _ := state["status"].(string)
	return value
}

func partInput(state map[string]any) map[string]any {
	value, _ := state["input"].(map[string]any)
	return value
}

func partOutput(state map[string]any) string {
	switch value := state["output"].(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

func partError(state map[string]any) string {
	value, _ := state["error"].(string)
	return value
}

// llmState tracks a live assistant message's LLM activity while its
// text and reasoning grow.
type llmState struct {
	activityID string

	callID string

	response strings.Builder

	reasoning strings.Builder

	finalized bool
}

// recordParts builds activity records from a completed assistant
// message's parts (text + tool calls), used as a fallback when the SSE
// stream did not deliver the live session.next.* events.
func (r *activityRecorder) recordParts(
	parts json.RawMessage,
) {

	if r.hasRecorded() ||
		len(parts) == 0 {
		return
	}

	var list []messagePart

	if err :=
		json.Unmarshal(parts, &list); err != nil {
		return
	}

	var text strings.Builder

	for _, part := range list {

		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}

	r.startStep()

	if text.Len() > 0 {
		r.appendText(text.String())
	}

	r.finishStep()

	for _, part := range list {

		if part.Type != "tool" ||
			part.State == nil {
			continue
		}

		r.startTool(
			part.Tool,
			part.CallID,
			partInput(part.State),
		)

		if partStatus(part.State) == "error" {

			r.finishTool(
				part.CallID,
				"",
				fmt.Errorf(
					"%s",
					partError(part.State),
				),
			)
		} else {

			r.finishTool(
				part.CallID,
				partOutput(part.State),
				nil,
			)
		}
	}
}

// ensureLLM creates the LLM activity and call for an assistant message
// if it does not exist yet.
func (r *activityRecorder) ensureLLM(
	messageID string,
) *llmState {

	if st, ok :=
		r.llmByMessage[messageID]; ok {
		return st
	}

	r.activitySeq++

	activity :=
		r.run.StartAgentActivity(
			r.agentExecutionID,
			r.activitySeq,
			graph.ActivityLLM,
		)

	sequence := r.llmSeq
	r.llmSeq++

	messages :=
		[]graph.MessageRecord(nil)

	if r.prompt != "" {
		messages =
			[]graph.MessageRecord{
				{
					Role:    "user",
					Content: r.prompt,
				},
			}
	}

	call :=
		r.run.StartLLMCall(
			r.agentExecutionID,
			activity.ID,
			sequence,
			messages,
		)

	st := &llmState{
		activityID: activity.ID,
		callID:     call.ID,
	}

	r.llmByMessage[messageID] = st

	graph.EmitEvent(
		r.ctx,
		graph.Event{
			Time: time.Now(),

			Type: graph.EventLLMStarted,

			AgentID: r.agentID,

			Data: map[string]any{
				"agentExecutionId": r.agentExecutionID,

				"activityId": activity.ID,

				"llmCallId": call.ID,

				"sequence": sequence,
			},
		},
	)

	return st
}

// updateLLM pushes the latest response + reasoning to the run so the UI
// shows the assistant text as it streams.
func (r *activityRecorder) updateLLM(
	st *llmState,
) {

	r.run.UpdateLLMCallResponse(
		st.callID,
		st.response.String(),
		st.reasoning.String(),
	)
}

// syncMessages reflects the session message history into the activity
// records incrementally (live text, reasoning, and tool calls).
func (r *activityRecorder) syncMessages(
	messages []SessionMessage,
) {

	for _, message := range messages {

		if message.Info.Role != "assistant" {
			continue
		}

		for _, raw := range message.Parts {

			var part messagePart

			if err :=
				json.Unmarshal(raw, &part); err != nil {
				continue
			}

			if part.ID == "" ||
				r.partsSeen[part.ID] {
				continue
			}

			r.partsSeen[part.ID] = true

			switch part.Type {

			case "text":

				if part.Synthetic {
					continue
				}

				st :=
					r.ensureLLM(message.Info.ID)

				st.response.WriteString(part.Text)

				r.updateLLM(st)

			case "reasoning":

				st :=
					r.ensureLLM(message.Info.ID)

				st.reasoning.WriteString(part.Text)

				r.updateLLM(st)

			case "tool":

				if part.State == nil {
					continue
				}

				r.ensureLLM(message.Info.ID)

				r.startTool(
					part.Tool,
					part.CallID,
					partInput(part.State),
				)

				switch partStatus(part.State) {

				case "error":
					r.finishTool(
						part.CallID,
						"",
						fmt.Errorf(
							"%s",
							partError(part.State),
						),
					)

				case "completed":
					r.finishTool(
						part.CallID,
						partOutput(part.State),
						nil,
					)
				}
			}
		}
	}
}

// finalize completes all open LLM activities and tool calls once the
// message finishes.
func (r *activityRecorder) finalize() {

	for _, st := range r.llmByMessage {

		if st.finalized {
			continue
		}

		st.finalized = true

		r.run.CompleteLLMCall(
			st.callID,
			st.response.String(),
			"",
			nil,
		)

		r.run.CompleteAgentActivity(
			st.activityID,
			nil,
		)

		graph.EmitEvent(
			r.ctx,
			graph.Event{
				Time: time.Now(),

				Type: graph.EventLLMCompleted,

				AgentID: r.agentID,
			},
		)
	}

	// Complete any tool calls still open (a tool reported running and
	// never completed in the history).
	for callID, record := range r.tools {

		_ = record

		r.finishTool(
			callID,
			"",
			nil,
		)
	}
}

// accumulatePart stores the assistant text for the final output,
// accepting either an appended delta or a growing full snapshot.
func (r *activityRecorder) accumulatePart(
	id string,
	text string,
) {

	current, seen :=
		r.textByPart[id]

	if !seen {

		r.textOrder =
			append(
				r.textOrder,
				id,
			)

		r.textByPart[id] =
			text

		return
	}

	if len(text) >=
		len(current) &&
		strings.HasPrefix(
			text,
			current,
		) {

		r.textByPart[id] =
			text
	} else {

		r.textByPart[id] =
			current + text
	}
}

// output returns the session ID and the assembled assistant text.
func (r *activityRecorder) output() (string, string) {

	var builder strings.Builder

	for _, id := range r.textOrder {
		builder.WriteString(
			r.textByPart[id],
		)
	}

	return r.sessionID, builder.String()
}
