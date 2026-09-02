// Package opencode integrates the OpenCode CLI as a graph Worker.
//
// A Worker shells out to `opencode run --format json` scoped to a
// workspace directory, streams the JSON events to capture the final
// assistant text, and returns it as the node output state.
package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"harnais/graph"
)

// permissionConfig scopes what the OpenCode subprocess may do.
//
// Workspace-internal edits/reads are allowed (the workspace is the
// working directory), while external directories, network access,
// subagents, and skills are denied. Shell commands are limited to
// the same development commands harnais exposes to its own agents.
const permissionConfig = `{
  "*": "allow",
  "external_directory": "deny",
  "webfetch": "deny",
  "websearch": "deny",
  "task": "deny",
  "skill": "deny",
  "bash": {
    "go *": "allow",
    "git *": "allow",
    "npm *": "allow",
    "node *": "allow",
    "pnpm *": "allow",
    "yarn *": "allow",
    "grep *": "allow",
    "*": "deny"
  }
}`

// readOnlyPermissionConfig is like permissionConfig but denies all
// file edits, for phases (planning, review) that must not mutate
// the workspace.
const readOnlyPermissionConfig = `{
  "*": "deny",
  "read": "allow",
  "external_directory": "deny",
  "webfetch": "deny",
  "websearch": "deny",
  "task": "deny",
  "skill": "deny",
  "bash": {
    "go *": "allow",
    "git *": "allow",
    "npm *": "allow",
    "node *": "allow",
    "pnpm *": "allow",
    "yarn *": "allow",
    "grep *": "allow",
    "*": "deny"
  }
}`

// Worker is a graph Worker that runs a task through the OpenCode CLI.
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

	// ReadOnly restricts the subprocess to read-only operations
	// (no file edits). Used for planning and review phases.
	ReadOnly bool

	// OutputKey is the state key the final assistant text is stored
	// under. Defaults to "output".
	OutputKey string

	// Binary is the OpenCode executable. Defaults to "opencode".
	Binary string

	// Timeout bounds a single run. Defaults to 10 minutes.
	Timeout time.Duration
}

func (w *Worker) ID() string {

	if w.AgentID != "" {
		return w.AgentID
	}

	return "opencode"
}

func (w *Worker) binary() string {

	if w.Binary != "" {
		return w.Binary
	}

	return "opencode"
}

func (w *Worker) timeout() time.Duration {

	if w.Timeout > 0 {
		return w.Timeout
	}

	return 10 * time.Minute
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
// OpenCode subprocess
// ------------------------------------------------------------

func (w *Worker) runOpenCode(
	ctx context.Context,
	executionContext graph.ExecutionContext,
	agentExecutionID string,
	state graph.State,
) (graph.WorkerResult, error) {

	if err :=
		os.MkdirAll(
			w.Dir,
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

	args := []string{
		"run",
		"--format", "json",
		"--dir", w.Dir,
		"--title", fmt.Sprintf(
			"harnais/%s/%s",
			executionContext.RunID,
			executionContext.NodeID,
		),
	}

	if w.Model != "" {
		args =
			append(
				args,
				"--model",
				w.Model,
			)
	}

	args =
		append(
			args,
			w.buildPrompt(state),
		)

	cmd :=
		exec.CommandContext(
			timeoutCtx,
			w.binary(),
			args...,
		)

	cmd.Dir = w.Dir

	permission :=
		permissionConfig

	if w.ReadOnly {
		permission =
			readOnlyPermissionConfig
	}

	cmd.Env =
		append(
			os.Environ(),
			"OPENCODE_PERMISSION="+permission,
			"OPENCODE_DISABLE_AUTOUPDATE=1",
		)

	// Make the process its own group leader so a timeout (or a
	// lingering child) can be released by killing the whole group.
	cmd.SysProcAttr =
		&syscall.SysProcAttr{
			Setpgid: true,
		}

	// Use *os.File pipes for both stdout and stderr. This avoids
	// os/exec's internal copier goroutines, which would make
	// Wait() block forever if a child outlives OpenCode while
	// holding the pipe open.
	pr, pw, err :=
		os.Pipe()

	if err != nil {
		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: stdout pipe: %w",
			err,
		)
	}

	defer pr.Close()

	stderrFile, err :=
		os.CreateTemp(
			"",
			"harnais-opencode-stderr-*",
		)

	if err != nil {
		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: stderr file: %w",
			err,
		)
	}

	defer os.Remove(
		stderrFile.Name(),
	)

	defer stderrFile.Close()

	cmd.Stdout = pw
	cmd.Stderr = stderrFile

	if err :=
		cmd.Start(); err != nil {

		pw.Close()

		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: start: %w",
			err,
		)
	}

	// The parent no longer needs its copy of the write end, so
	// close it to let EOF propagate once OpenCode (and any child
	// still writing) is gone.
	pw.Close()

	recorder :=
		newActivityRecorder(
			ctx,
			executionContext.Run,
			w.ID(),
			agentExecutionID,
			w.buildPrompt(state),
		)

	// --------------------------------------------------------
	// Stream stdout lines while the process runs. Reading in a
	// goroutine lets us stop even if a lingering child keeps the
	// pipe open after OpenCode exits.
	// --------------------------------------------------------

	lines :=
		make(
			chan []byte,
			256,
		)

	go func() {
		defer close(lines)

		scanner :=
			bufio.NewScanner(pr)

		scanner.Buffer(
			make(
				[]byte,
				0,
				64*1024,
			),
			4*1024*1024,
		)

		for scanner.Scan() {

			line :=
				append(
					[]byte(nil),
					scanner.Bytes()...,
				)

			lines <- line
		}
	}()

	// Wait() has no copier goroutines to wait on (both output
	// targets are *os.File), so it returns as soon as OpenCode's
	// process exits.
	waitResult :=
		make(chan error, 1)

	go func() {
		waitResult <- cmd.Wait()
	}()

	waitDone := false

	var waitErr error

	for {
		select {

		case line, ok :=
			<-lines:

			if !ok {
				lines = nil

				continue
			}

			recorder.process(line)

		case err :=
			<-waitResult:

			waitErr = err
			waitDone = true

			if lines == nil {
				break
			}

			// OpenCode exited but the pipe is still open
			// (a grandchild inherited it). Release the group.
			if cmd.Process != nil {
				_ = syscall.Kill(
					-cmd.Process.Pid,
					syscall.SIGKILL,
				)
			}
		}

		if waitDone &&
			lines == nil {
			break
		}
	}

	if waitErr != nil {

		message :=
			strings.TrimSpace(
				readAll(stderrFile),
			)

		return graph.WorkerResult{}, fmt.Errorf(
			"opencode: %v%s",
			waitErr,
			suffixMessage(message),
		)
	}

	sessionID, text :=
		recorder.output()

	outputKey :=
		w.OutputKey

	if outputKey == "" {
		outputKey = "output"
	}

	return graph.WorkerResult{
		State: graph.State{
			outputKey: text,

			"sessionId": sessionID,
		},
	}, nil
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
	stepStarted   bool
	stepActivityID string
	stepLLMCallID  string
	stepText       strings.Builder
	stepTool       string
	steps          int

	tools map[string]toolRecord

	sessionID string

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

func (r *activityRecorder) startStep() {

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

func suffixMessage(
	message string,
) string {

	if message == "" {
		return ""
	}

	return ": " + message
}

// readAll reads the full content of a file starting at offset zero.
func readAll(
	file *os.File,
) string {

	if _, err :=
		file.Seek(
			0,
			io.SeekStart,
		); err != nil {

		return ""
	}

	data, err :=
		io.ReadAll(file)

	if err != nil {
		return ""
	}

	return string(data)
}