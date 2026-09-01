package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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

type Server struct {
	Bus *EventBus

	Runs *RunManager

	StartRun StartRunFunc

	Workflows func() []WorkflowInfo
}

func NewServer(
	bus *EventBus,
	runs *RunManager,
	startRun StartRunFunc,
	workflows func() []WorkflowInfo,
) *Server {
	return &Server{
		Bus: bus,

		Runs: runs,

		StartRun: startRun,

		Workflows: workflows,
	}
}

func (s *Server) Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /api/runs",
		s.createRun,
	)

	mux.HandleFunc(
		"GET /api/workflows",
		s.getWorkflows,
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
// Run
// ------------------------------------------------------------

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

	writeJSON(
		w,
		map[string]any{
			"id": snapshot.ID,

			"status": snapshot.Status,

			"startedAt": snapshot.StartedAt,

			"completedAt": snapshot.CompletedAt,
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
				"GET, POST, OPTIONS",
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
