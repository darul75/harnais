package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"harnais/config"
	"harnais/graph"
)

type StartRunRequest struct {
	Task string

	WorkflowID string
}

type StartRunFunc func(
	request StartRunRequest,
) *graph.Run

type WorkflowInfo struct {
	ID string `json:"id"`

	Title string `json:"title"`

	Description string `json:"description"`
}

type WorkflowNodeInfo struct {
	ID string `json:"id"`

	Kind string `json:"kind"`

	AgentID string `json:"agentId,omitempty"`

	Prompt string `json:"prompt,omitempty"`

	Tools []string `json:"tools,omitempty"`

	JoinAll bool `json:"joinAll"`
}

type WorkflowEdgeInfo struct {
	ID string `json:"id"`

	From string `json:"from"`

	To string `json:"to"`

	Conditional bool `json:"conditional"`
}

type WorkflowDetail struct {
	ID string `json:"id"`

	Title string `json:"title"`

	Description string `json:"description"`

	Nodes []WorkflowNodeInfo `json:"nodes"`

	Edges []WorkflowEdgeInfo `json:"edges"`
}

type Server struct {
	Bus *EventBus

	Runs *RunManager

	Settings *config.Store

	StartRun StartRunFunc

	Workflows func() []WorkflowInfo

	GetWorkflow func(id string) (*WorkflowDetail, bool)
}

func NewServer(
	bus *EventBus,
	runs *RunManager,
	settings *config.Store,
	startRun StartRunFunc,
	workflows func() []WorkflowInfo,
	getWorkflow func(id string) (*WorkflowDetail, bool),
) *Server {
	return &Server{
		Bus: bus,

		Runs: runs,

		Settings: settings,

		StartRun: startRun,

		Workflows: workflows,

		GetWorkflow: getWorkflow,
	}
}

func (s *Server) Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /api/runs",
		s.createRun,
	)

	mux.HandleFunc(
		"GET /api/runs",
		s.getRuns,
	)

	mux.HandleFunc(
		"GET /api/workflows",
		s.getWorkflows,
	)

	mux.HandleFunc(
		"GET /api/workflows/{workflowID}",
		s.getWorkflow,
	)

	mux.HandleFunc(
		"GET /api/settings",
		s.getSettings,
	)

	mux.HandleFunc(
		"PUT /api/settings",
		s.updateSettings,
	)

	mux.HandleFunc(
		"POST /api/settings/test",
		s.testSettings,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}",
		s.getRun,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/graph",
		s.getGraph,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/state",
		s.getState,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/executions",
		s.getExecutions,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/tree",
		s.getExecutionTree,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/agent-executions",
		s.getAgentExecutions,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/llm-calls",
		s.getLLMCalls,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/tool-calls",
		s.getToolCalls,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/events",
		s.events,
	)

	return withCORS(mux)
}

// ------------------------------------------------------------
// Create
// ------------------------------------------------------------

type createRunRequest struct {
	State graph.State `json:"state"`
}

type createRunResponse struct {
	RunID string `json:"runId"`
}

func (s *Server) createRun(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request struct {
		Task       string `json:"task"`
		WorkflowID string `json:"workflowId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(
			w,
			"invalid JSON",
			http.StatusBadRequest,
		)
		return
	}

	fmt.Printf(
		"[HTTP] create run task=%q workflow=%q\n",
		request.Task,
		request.WorkflowID,
	)

	if request.Task == "" {
		http.Error(
			w,
			"task is required",
			http.StatusBadRequest,
		)
		return
	}

	run := s.StartRun(
		StartRunRequest{
			Task:       request.Task,
			WorkflowID: request.WorkflowID,
		},
	)

	writeJSON(
		w,
		map[string]any{
			"runId": run.ID,
			"task":  request.Task,
		},
	)
}

// ------------------------------------------------------------
// Workflows
// ------------------------------------------------------------

func (s *Server) getWorkflows(
	w http.ResponseWriter,
	r *http.Request,
) {

	if s.Workflows == nil {
		http.Error(
			w,
			"workflows not configured",
			http.StatusInternalServerError,
		)
		return
	}

	writeJSON(
		w,
		s.Workflows(),
	)
}

// ------------------------------------------------------------
// Workflow detail
// ------------------------------------------------------------

func (s *Server) getWorkflow(
	w http.ResponseWriter,
	r *http.Request,
) {

	workflowID :=
		r.PathValue("workflowID")

	if s.GetWorkflow == nil {
		http.Error(
			w,
			"workflows not configured",
			http.StatusInternalServerError,
		)
		return
	}

	workflow, ok :=
		s.GetWorkflow(workflowID)

	if !ok {
		http.Error(
			w,
			"workflow not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(
		w,
		workflow,
	)
}

// ------------------------------------------------------------
// Settings
// ------------------------------------------------------------

func (s *Server) getSettings(
	w http.ResponseWriter,
	r *http.Request,
) {

	if s.Settings == nil {
		http.Error(
			w,
			"settings not configured",
			http.StatusInternalServerError,
		)
		return
	}

	writeJSON(
		w,
		s.Settings.View(),
	)
}

func (s *Server) updateSettings(
	w http.ResponseWriter,
	r *http.Request,
) {

	if s.Settings == nil {
		http.Error(
			w,
			"settings not configured",
			http.StatusInternalServerError,
		)
		return
	}

	var update config.SettingsUpdate

	if err :=
		json.NewDecoder(
			r.Body,
		).Decode(
			&update,
		); err != nil {

		http.Error(
			w,
			"invalid JSON",
			http.StatusBadRequest,
		)
		return
	}

	if err :=
		s.Settings.Update(update); err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	writeJSON(
		w,
		s.Settings.View(),
	)
}

func (s *Server) testSettings(
	w http.ResponseWriter,
	r *http.Request,
) {

	var request config.TestRequest

	if err :=
		json.NewDecoder(
			r.Body,
		).Decode(
			&request,
		); err != nil {

		http.Error(
			w,
			"invalid JSON",
			http.StatusBadRequest,
		)
		return
	}

	result :=
		config.TestResult{
			OK: true,
		}

	if err :=
		config.Test(
			request.Provider,
			request.Values,
		); err != nil {

		result.OK = false
		result.Message =
			err.Error()
	}

	writeJSON(
		w,
		result,
	)
}

type runSummary struct {
	ID string `json:"id"`

	Task string `json:"task"`

	WorkflowID string `json:"workflowId"`

	Status graph.Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`

	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

func (s *Server) getRuns(
	w http.ResponseWriter,
	r *http.Request,
) {

	records :=
		s.Runs.List()

	summaries :=
		make(
			[]runSummary,
			0,
			len(records),
		)

	for _, record := range records {

		summaries =
			append(
				summaries,
				runSummary{
					ID: record.Run.ID,

					Task: record.Meta.Task,

					WorkflowID: record.Meta.WorkflowID,

					Status: record.Run.Status,

					StartedAt: record.Run.StartedAt,

					CompletedAt: record.Run.CompletedAt,
				},
			)
	}

	writeJSON(
		w,
		summaries,
	)
}

func (s *Server) getRun(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	meta, _ :=
		s.Runs.Meta(runID)

	writeJSON(
		w,
		map[string]any{
			"id": snapshot.ID,

			"task": meta.Task,

			"workflowId": meta.WorkflowID,

			"status": snapshot.Status,

			"startedAt": snapshot.StartedAt,

			"completedAt": snapshot.CompletedAt,

			"state": snapshot.State,
		},
	)
}

// ------------------------------------------------------------
// Graph
// ------------------------------------------------------------

func (s *Server) getGraph(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	type NodeDTO struct {
		ID string `json:"id"`
	}

	type EdgeDTO struct {
		ID string `json:"id"`

		From string `json:"from"`

		To string `json:"to"`
	}

	nodes :=
		make(
			[]NodeDTO,
			0,
			len(run.Graph.Nodes),
		)

	for id := range run.Graph.Nodes {

		nodes =
			append(
				nodes,
				NodeDTO{
					ID: id,
				},
			)
	}

	edges :=
		make(
			[]EdgeDTO,
			0,
			len(run.Graph.Edges),
		)

	for _, edge := range run.Graph.Edges {

		edges =
			append(
				edges,
				EdgeDTO{
					ID: edge.ID,

					From: edge.From,

					To: edge.To,
				},
			)
	}

	writeJSON(
		w,
		map[string]any{
			"nodes": nodes,

			"edges": edges,
		},
	)
}

// ------------------------------------------------------------
// State
// ------------------------------------------------------------

func (s *Server) getState(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	writeJSON(
		w,
		run.StateSnapshot(),
	)
}

// ------------------------------------------------------------
// Node executions
// ------------------------------------------------------------

func (s *Server) getExecutions(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	writeJSON(
		w,
		snapshot.Executions,
	)
}

// ------------------------------------------------------------
// Agent executions
// ------------------------------------------------------------

func (s *Server) getAgentExecutions(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	writeJSON(
		w,
		snapshot.AgentExecutions,
	)
}

// ------------------------------------------------------------
// LLM calls
// ------------------------------------------------------------

func (s *Server) getLLMCalls(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	writeJSON(
		w,
		snapshot.LLMCalls,
	)
}

// ------------------------------------------------------------
// Tool calls
// ------------------------------------------------------------

func (s *Server) getToolCalls(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	run, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot :=
		run.Snapshot()

	writeJSON(
		w,
		snapshot.ToolCalls,
	)
}

// ------------------------------------------------------------
// SSE
// ------------------------------------------------------------

func (s *Server) events(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID :=
		r.PathValue("runID")

	_, exists :=
		s.Runs.Get(runID)

	if !exists {

		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	w.Header().Set(
		"Content-Type",
		"text/event-stream",
	)

	w.Header().Set(
		"Cache-Control",
		"no-cache",
	)

	w.Header().Set(
		"Connection",
		"keep-alive",
	)

	w.Header().Set(
		"X-Accel-Buffering",
		"no",
	)

	flusher, ok :=
		w.(http.Flusher)

	if !ok {

		http.Error(
			w,
			"SSE not supported",
			http.StatusInternalServerError,
		)

		return
	}

	lastEventID :=
		uint64(0)

	if value :=
		r.Header.Get("Last-Event-ID"); value != "" {

		lastEventID, _ =
			strconv.ParseUint(
				value,
				10,
				64,
			)
	}

	for _, event := range s.Bus.History(runID) {

		if event.ID <= lastEventID {
			continue
		}

		writeSSE(
			w,
			event,
		)
	}

	flusher.Flush()

	eventChannel, unsubscribe :=
		s.Bus.Subscribe(runID)

	defer unsubscribe()

	fmt.Fprint(
		w,
		": connected\n\n",
	)

	flusher.Flush()

	for {

		select {

		case <-r.Context().Done():
			return

		case event, ok :=
			<-eventChannel:

			if !ok {
				return
			}

			writeSSE(
				w,
				event,
			)

			flusher.Flush()
		}
	}
}

// ------------------------------------------------------------
// SSE writer
// ------------------------------------------------------------

func writeSSE(
	w http.ResponseWriter,
	event graph.Event,
) {
	data, err :=
		json.Marshal(event)

	if err != nil {
		return
	}

	fmt.Fprintf(
		w,
		"id: %d\n",
		event.ID,
	)

	fmt.Fprintf(
		w,
		"event: %s\n",
		event.Type,
	)

	fmt.Fprintf(
		w,
		"data: %s\n\n",
		data,
	)
}

// ------------------------------------------------------------
// JSON
// ------------------------------------------------------------

func writeJSON(
	w http.ResponseWriter,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	_ = json.NewEncoder(
		w,
	).Encode(value)
}

// ------------------------------------------------------------
// CORS
// ------------------------------------------------------------

func withCORS(
	next http.Handler,
) http.Handler {

	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {

			w.Header().Set(
				"Access-Control-Allow-Origin",
				"http://localhost:5173",
			)

			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type, Last-Event-ID",
			)

			w.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, OPTIONS",
			)

			if r.Method ==
				http.MethodOptions {

				w.WriteHeader(
					http.StatusNoContent,
				)

				return
			}

			next.ServeHTTP(
				w,
				r,
			)
		},
	)
}
