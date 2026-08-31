package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"harnais/graph"
)

type StartRunFunc func(
	initial graph.State,
) *graph.Run

type Server struct {
	Bus  *EventBus
	Runs *RunManager

	StartRun StartRunFunc
}

func NewServer(
	bus *EventBus,
	runs *RunManager,
	startRun StartRunFunc,
) *Server {
	return &Server{
		Bus:      bus,
		Runs:     runs,
		StartRun: startRun,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /api/runs",
		s.createRun,
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
		"GET /api/runs/{runID}/events",
		s.events,
	)

	return withCORS(mux)
}

// ------------------------------------------------------------
// POST /api/runs
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
	var request createRunRequest

	if r.ContentLength != 0 {

		if err := json.NewDecoder(
			r.Body,
		).Decode(&request); err != nil {

			http.Error(
				w,
				"invalid JSON",
				http.StatusBadRequest,
			)

			return
		}
	}

	if request.State == nil {
		request.State = make(graph.State)
	}

	if s.StartRun == nil {
		http.Error(
			w,
			"run starter is not configured",
			http.StatusInternalServerError,
		)

		return
	}

	run := s.StartRun(
		request.State,
	)

	writeJSON(
		w,
		createRunResponse{
			RunID: run.ID,
		},
	)
}

// ------------------------------------------------------------
// GET /api/runs/:runID
// ------------------------------------------------------------

func (s *Server) getRun(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID := r.PathValue("runID")

	run, exists := s.Runs.Get(runID)

	if !exists {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot := run.Snapshot()

	writeJSON(
		w,
		map[string]any{
			"id":          snapshot.ID,
			"status":      snapshot.Status,
			"startedAt":   snapshot.StartedAt,
			"completedAt": snapshot.CompletedAt,
		},
	)
}

// ------------------------------------------------------------
// GET /api/runs/:runID/graph
// ------------------------------------------------------------

func (s *Server) getGraph(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID := r.PathValue("runID")

	run, exists := s.Runs.Get(runID)

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
		ID   string `json:"id"`
		From string `json:"from"`
		To   string `json:"to"`
	}

	nodes := make(
		[]NodeDTO,
		0,
		len(run.Graph.Nodes),
	)

	for id := range run.Graph.Nodes {
		nodes = append(
			nodes,
			NodeDTO{
				ID: id,
			},
		)
	}

	edges := make(
		[]EdgeDTO,
		0,
		len(run.Graph.Edges),
	)

	for _, edge := range run.Graph.Edges {
		edges = append(
			edges,
			EdgeDTO{
				ID:   edge.ID,
				From: edge.From,
				To:   edge.To,
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
// GET /api/runs/:runID/state
// ------------------------------------------------------------

func (s *Server) getState(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID := r.PathValue("runID")

	run, exists := s.Runs.Get(runID)

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
// GET /api/runs/:runID/executions
// ------------------------------------------------------------

func (s *Server) getExecutions(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID := r.PathValue("runID")

	run, exists := s.Runs.Get(runID)

	if !exists {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)

		return
	}

	snapshot := run.Snapshot()

	writeJSON(
		w,
		snapshot.Executions,
	)
}

// ------------------------------------------------------------
// GET /api/runs/:runID/events
// ------------------------------------------------------------

func (s *Server) events(
	w http.ResponseWriter,
	r *http.Request,
) {
	runID := r.PathValue("runID")

	_, exists := s.Runs.Get(runID)

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

	flusher, ok := w.(http.Flusher)

	if !ok {
		http.Error(
			w,
			"SSE not supported",
			http.StatusInternalServerError,
		)

		return
	}

	// --------------------------------------------------
	// Replay history
	// --------------------------------------------------

	lastEventID := uint64(0)

	if value := r.Header.Get("Last-Event-ID"); value != "" {
		lastEventID, _ = strconv.ParseUint(
			value,
			10,
			64,
		)
	}

	history := s.Bus.History(runID)

	for _, event := range history {

		if event.ID <= lastEventID {
			continue
		}

		writeSSE(
			w,
			event,
		)
	}

	flusher.Flush()

	// --------------------------------------------------
	// Subscribe
	// --------------------------------------------------

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

		case event, ok := <-eventChannel:

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
// SSE
// ------------------------------------------------------------

func writeSSE(
	w http.ResponseWriter,
	event graph.Event,
) {
	data, err := json.Marshal(event)

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

	_ = json.NewEncoder(w).Encode(value)
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

			if r.Method == http.MethodOptions {
				w.WriteHeader(
					http.StatusNoContent,
				)

				return
			}

			next.ServeHTTP(w, r)
		},
	)
}
