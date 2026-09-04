package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

type StartRunRequest struct {
	Task string

	WorkflowID string

	// PDFPath is a workspace-relative path to an uploaded PDF to
	// process (used by the PDF summary workflow).
	PDFPath string
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

	Workspace *tools.Workspace

	StartRun StartRunFunc

	Workflows func() []WorkflowInfo

	GetWorkflow func(id string) (*WorkflowDetail, bool)

	// Questions dispatches user answers to workers blocked on an
	// OpenCode clarifying question.
	Questions *graph.QuestionHub
}

func NewServer(
	bus *EventBus,
	runs *RunManager,
	settings *config.Store,
	workspace *tools.Workspace,
	startRun StartRunFunc,
	workflows func() []WorkflowInfo,
	getWorkflow func(id string) (*WorkflowDetail, bool),
	questionHub *graph.QuestionHub,
) *Server {
	return &Server{
		Bus: bus,

		Runs: runs,

		Settings: settings,

		Workspace: workspace,

		StartRun: startRun,

		Workflows: workflows,

		GetWorkflow: getWorkflow,

		Questions: questionHub,
	}
}

func (s *Server) Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /api/runs",
		s.createRun,
	)

	mux.HandleFunc(
		"POST /api/upload",
		s.uploadFile,
	)

	mux.HandleFunc(
		"GET /api/uploads/{name}",
		s.getUpload,
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
		"GET /api/reports",
		s.listAllReports,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/reports",
		s.listRunReports,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/reports/{name}",
		s.getRunReport,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/audio",
		s.listRunAudio,
	)

	mux.HandleFunc(
		"GET /api/runs/{runID}/audio/{name}",
		s.getRunAudio,
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

	mux.HandleFunc(
		"POST /api/runs/{runID}/question/{requestID}/reply",
		s.answerQuestion,
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
		PDFPath    string `json:"pdfPath"`
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
			PDFPath:    request.PDFPath,
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

// answerQuestion delivers a user's answer to a worker blocked on an
// OpenCode clarifying question, resuming that run's opencode session.
func (s *Server) answerQuestion(
	w http.ResponseWriter,
	r *http.Request,
) {

	var request struct {
		Answers [][]string `json:"answers"`
	}

	if err :=
		json.NewDecoder(
			r.Body,
		).Decode(&request); err != nil {

		http.Error(
			w,
			"invalid JSON",
			http.StatusBadRequest,
		)

		return
	}

	if s.Questions == nil {
		http.Error(
			w,
			"question hub not configured",
			http.StatusInternalServerError,
		)

		return
	}

	delivered :=
		s.Questions.Reply(
			r.PathValue("runID"),
			r.PathValue("requestID"),
			request.Answers,
		)

	if !delivered {
		http.Error(
			w,
			"question not found or already answered",
			http.StatusNotFound,
		)

		return
	}

	w.WriteHeader(
		http.StatusNoContent,
	)
}

// maxUploadBytes caps uploaded file sizes (e.g. PDFs).
const maxUploadBytes = 25 << 20 // 25 MB

// uploadFile accepts a multipart file upload, saves it under the
// workspace uploads/ directory, and returns its workspace-relative
// path so a run can reference it.
func (s *Server) uploadFile(
	w http.ResponseWriter,
	r *http.Request,
) {

	if s.Workspace == nil {
		http.Error(
			w,
			"workspace not configured",
			http.StatusInternalServerError,
		)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxUploadBytes,
	)

	if err :=
		r.ParseMultipartForm(
			maxUploadBytes,
		); err != nil {
		http.Error(
			w,
			"failed to parse upload: "+err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	file, header, err :=
		r.FormFile("file")

	if err != nil {
		http.Error(
			w,
			"missing file field",
			http.StatusBadRequest,
		)
		return
	}

	defer file.Close()

	if !strings.HasSuffix(
		strings.ToLower(header.Filename),
		".pdf",
	) {
		http.Error(
			w,
			"only .pdf files are supported",
			http.StatusBadRequest,
		)
		return
	}

	name :=
		filepath.Base(header.Filename)

	if name == "." ||
		name == "/" ||
		name == "" {
		name = "upload.pdf"
	}

	// Keep the readable base name but make every upload unique so
	// re-uploading a same-named PDF cannot overwrite an existing
	// file a run may still reference.
	ext :=
		filepath.Ext(name)

	base :=
		strings.TrimSuffix(
			name,
			ext,
		)

	name =
		fmt.Sprintf(
			"%s-%d%s",
			base,
			time.Now().UnixNano(),
			ext,
		)

	relative :=
		filepath.Join(
			"uploads",
			name,
		)

	resolved, err :=
		s.Workspace.Resolve(relative)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(resolved),
			0o755,
		); err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	dst, err :=
		os.Create(resolved)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	defer dst.Close()

	if _, err :=
		io.Copy(dst, file); err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	writeJSON(
		w,
		map[string]any{
			"path": relative,
		},
	)
}

// getUpload serves a previously uploaded file (e.g. a PDF) so the UI
// can link to it. Only PDFs are served, matching what uploadFile
// accepts.
func (s *Server) getUpload(
	w http.ResponseWriter,
	r *http.Request,
) {

	if s.Workspace == nil {
		http.Error(
			w,
			"workspace not configured",
			http.StatusInternalServerError,
		)
		return
	}

	name :=
		r.PathValue("name")

	if name == "" ||
		!strings.HasSuffix(
			strings.ToLower(name),
			".pdf",
		) {
		http.Error(
			w,
			"not found",
			http.StatusNotFound,
		)
		return
	}

	resolved, err :=
		s.Workspace.Resolve(
			filepath.Join(
				"uploads",
				name,
			),
		)

	if err != nil {
		http.Error(
			w,
			"not found",
			http.StatusNotFound,
		)
		return
	}

	file, err :=
		os.Open(resolved)

	if err != nil {
		http.Error(
			w,
			"not found",
			http.StatusNotFound,
		)
		return
	}

	defer file.Close()

	info, err :=
		file.Stat()

	if err != nil {
		http.Error(
			w,
			"not found",
			http.StatusNotFound,
		)
		return
	}

	if contentType :=
		mime.TypeByExtension(
			filepath.Ext(name),
		); contentType != "" {

		w.Header().Set(
			"Content-Type",
			contentType,
		)
	}

	http.ServeContent(
		w,
		r,
		name,
		info.ModTime(),
		file,
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
	view :=
		s.Settings.View()

	// Offer the models the running OpenCode server exposes for the
	// opencode provider's model fields.
	serverURL :=
		s.Settings.Get(
			"opencode",
			"serverUrl",
		)

	if models :=
		config.OpenCodeModels(
			serverURL,
		); len(models) > 0 {

		for i := range view.Providers {

			if view.Providers[i].ID !=
				"opencode" {
				continue
			}

			for j := range view.Providers[i].Fields {

				switch view.
					Providers[i].Fields[j].Key {

				case "model",
					"plannerModel",
					"reviewerModel":

					view.Providers[i].
						Fields[j].Suggestions =
						models
				}
			}
		}
	}

	writeJSON(
		w,
		view,
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

// ------------------------------------------------------------
// Reports
// ------------------------------------------------------------

type reportInfo struct {
	RunID string `json:"runId"`

	Name string `json:"name"`

	Size int64 `json:"size"`

	Modified time.Time `json:"modified"`
}

// reportRoot is the directory (relative to the workspace) where
// generated markdown reports are stored, one subdirectory per run.
const reportRoot = "reports"

func (s *Server) reportsRoot() (string, error) {

	if s.Workspace == nil {
		return "", fmt.Errorf(
			"workspace not configured",
		)
	}

	resolved, err :=
		s.Workspace.Resolve(
			reportRoot,
		)

	if err != nil {
		return "", err
	}

	return resolved, nil
}

func (s *Server) requireRun(
	w http.ResponseWriter,
	runID string,
) bool {

	if _, exists :=
		s.Runs.Get(runID); exists {

		return true
	}

	if s.Runs.Store() != nil {
		if _, err := s.Runs.Store().GetRun(runID); err == nil {
			return true
		}
	}

	http.Error(
		w,
		"run not found",
		http.StatusNotFound,
	)

	return false
}

func (s *Server) resolveRunReport(
	w http.ResponseWriter,
	runID string,
	name string,
) (string, bool) {

	if name == "" {
		http.Error(
			w,
			"report name is required",
			http.StatusBadRequest,
		)

		return "", false
	}

	resolved, err :=
		s.Workspace.Resolve(
			filepath.Join(
				reportRoot,
				runID,
				name,
			),
		)

	if err != nil {
		http.Error(
			w,
			"invalid report path",
			http.StatusBadRequest,
		)

		return "", false
	}

	return resolved, true
}

// collectReports walks a directory and returns the markdown files
// inside it, tagged with the runID they belong to.
func collectReports(
	root string,
	runID string,
) []reportInfo {

	var reports []reportInfo

	_ = filepath.WalkDir(
		root,
		func(
			path string,
			entry os.DirEntry,
			err error,
		) error {

			if err != nil ||
				entry.IsDir() {
				return nil
			}

			if !strings.HasSuffix(
				strings.ToLower(
					entry.Name(),
				),
				".md",
			) {
				return nil
			}

			info, infoErr :=
				entry.Info()

			if infoErr != nil {
				return nil
			}

			relative, relErr :=
				filepath.Rel(
					root,
					path,
				)

			if relErr != nil {
				return nil
			}

			reports =
				append(
					reports,
					reportInfo{
						RunID: runID,

						Name: relative,

						Size: info.Size(),

						Modified: info.ModTime(),
					},
				)

			return nil
		},
	)

	return reports
}

// listRunReports lists the markdown reports for one run.
func (s *Server) listRunReports(
	w http.ResponseWriter,
	r *http.Request,
) {

	runID :=
		r.PathValue("runID")

	if !s.requireRun(w, runID) {
		return
	}

	root, err :=
		s.reportsRoot()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	dir :=
		filepath.Join(
			root,
			runID,
		)

	reports :=
		collectReports(
			dir,
			runID,
		)

	sort.Slice(
		reports,
		func(i, j int) bool {
			return reports[i].Name <
				reports[j].Name
		},
	)

	writeJSON(
		w,
		map[string]any{
			"reports": reports,
		},
	)
}

// listAllReports lists every run's markdown reports, grouped by run.
func (s *Server) listAllReports(
	w http.ResponseWriter,
	r *http.Request,
) {

	root, err :=
		s.reportsRoot()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	var reports []reportInfo

	entries, err :=
		os.ReadDir(root)

	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(
				w,
				map[string]any{
					"reports": []reportInfo{},
				},
			)
			return
		}

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	for _, entry := range entries {

		if !entry.IsDir() {
			continue
		}

		reports =
			append(
				reports,
				collectReports(
					filepath.Join(
						root,
						entry.Name(),
					),
					entry.Name(),
				)...,
			)
	}

	sort.Slice(
		reports,
		func(i, j int) bool {

			if reports[i].RunID !=
				reports[j].RunID {

				return reports[i].RunID <
					reports[j].RunID
			}

			return reports[i].Name <
				reports[j].Name
		},
	)

	writeJSON(
		w,
		map[string]any{
			"reports": reports,
		},
	)
}

// getRunReport returns a single run's report content.
func (s *Server) getRunReport(
	w http.ResponseWriter,
	r *http.Request,
) {

	runID :=
		r.PathValue("runID")

	if !s.requireRun(w, runID) {
		return
	}

	name :=
		r.PathValue("name")

	resolved, ok :=
		s.resolveRunReport(
			w,
			runID,
			name,
		)

	if !ok {
		return
	}

	content, err :=
		os.ReadFile(resolved)

	if err != nil {
		http.Error(
			w,
			"report not found",
			http.StatusNotFound,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"text/markdown; charset=utf-8",
	)

	_, _ =
		w.Write(content)
}

// ------------------------------------------------------------
// Audio
// ------------------------------------------------------------

// isAudioName reports whether a file name is an audio file we serve
// for playback in the run view.
func isAudioName(name string) bool {

	ext :=
		strings.ToLower(
			filepath.Ext(name),
		)

	switch ext {

	case ".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac":
		return true
	}

	return false
}

// collectAudio walks a directory and returns the audio files inside
// it, tagged with the runID they belong to.
func collectAudio(
	root string,
	runID string,
) []reportInfo {

	var items []reportInfo

	_ = filepath.WalkDir(
		root,
		func(
			path string,
			entry os.DirEntry,
			err error,
		) error {

			if err != nil ||
				entry.IsDir() {
				return nil
			}

			if !isAudioName(
				entry.Name(),
			) {
				return nil
			}

			info, infoErr :=
				entry.Info()

			if infoErr != nil {
				return nil
			}

			relative, relErr :=
				filepath.Rel(
					root,
					path,
				)

			if relErr != nil {
				return nil
			}

			items =
				append(
					items,
					reportInfo{
						RunID: runID,

						Name: relative,

						Size: info.Size(),

						Modified: info.ModTime(),
					},
				)

			return nil
		},
	)

	return items
}

// listRunAudio lists the audio files for one run.
func (s *Server) listRunAudio(
	w http.ResponseWriter,
	r *http.Request,
) {

	runID :=
		r.PathValue("runID")

	if !s.requireRun(w, runID) {
		return
	}

	root, err :=
		s.reportsRoot()

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	dir :=
		filepath.Join(
			root,
			runID,
		)

	items :=
		collectAudio(
			dir,
			runID,
		)

	sort.Slice(
		items,
		func(i, j int) bool {
			return items[i].Name <
				items[j].Name
		},
	)

	writeJSON(
		w,
		map[string]any{
			"audio": items,
		},
	)
}

// getRunAudio serves a single run's audio file bytes for browser
// playback. ServeContent supports range requests so seeking works.
func (s *Server) getRunAudio(
	w http.ResponseWriter,
	r *http.Request,
) {

	runID :=
		r.PathValue("runID")

	if !s.requireRun(w, runID) {
		return
	}

	name :=
		r.PathValue("name")

	if name == "" {
		http.Error(
			w,
			"audio name is required",
			http.StatusBadRequest,
		)
		return
	}

	resolved, err :=
		s.Workspace.Resolve(
			filepath.Join(
				reportRoot,
				runID,
				name,
			),
		)

	if err != nil {
		http.Error(
			w,
			"invalid audio path",
			http.StatusBadRequest,
		)
		return
	}

	file, err :=
		os.Open(resolved)

	if err != nil {
		http.Error(
			w,
			"audio not found",
			http.StatusNotFound,
		)
		return
	}

	defer file.Close()

	info, err :=
		file.Stat()

	if err != nil {
		http.Error(
			w,
			"audio not found",
			http.StatusNotFound,
		)
		return
	}

	if contentType :=
		mime.TypeByExtension(
			filepath.Ext(name),
		); contentType != "" {

		w.Header().Set(
			"Content-Type",
			contentType,
		)
	}

	http.ServeContent(
		w,
		r,
		name,
		info.ModTime(),
		file,
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

	var summaries []runSummary

	storeRecords, err := s.Runs.ListFromStore()
	if err == nil && len(storeRecords) > 0 {
		summaries = make([]runSummary, 0, len(storeRecords))
		for _, rec := range storeRecords {
			summaries = append(summaries, runSummary{
				ID:          rec.RunID,
				Task:        rec.Task,
				WorkflowID:  rec.WorkflowID,
				Status:      rec.Status,
				StartedAt:   rec.StartedAt,
				CompletedAt: rec.CompletedAt,
			})
		}
	} else {
		records := s.Runs.List()
		summaries = make([]runSummary, 0, len(records))
		for _, record := range records {
			summaries = append(summaries, runSummary{
				ID: record.Run.ID,

				Task: record.Meta.Task,

				WorkflowID: record.Meta.WorkflowID,

				Status: record.Run.Status,

				StartedAt: record.Run.StartedAt,

				CompletedAt: record.Run.CompletedAt,
			})
		}
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

	if exists {
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
		return
	}

	detail, err := s.Runs.Store().GetRunDetail(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(
		w,
		map[string]any{
			"id":          detail.RunID,
			"task":        detail.Task,
			"workflowId":  detail.WorkflowID,
			"status":      detail.Status,
			"startedAt":   detail.StartedAt,
			"completedAt": detail.CompletedAt,
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

	if exists {
		writeJSON(
			w,
			run.StateSnapshot(),
		)
		return
	}

	detail, err := s.Runs.Store().GetRunDetail(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	state := make(graph.State)
	for _, e := range detail.NodeExecutions {
		if e.Output != nil {
			for k, v := range e.Output {
				state[k] = v
			}
		}
	}

	writeJSON(w, state)
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

	if exists {
		snapshot :=
			run.Snapshot()

		writeJSON(
			w,
			snapshot.Executions,
		)
		return
	}

	execs, err := s.Runs.Store().GetNodeExecutions(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(w, execs)
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

	if exists {
		snapshot :=
			run.Snapshot()

		writeJSON(
			w,
			snapshot.AgentExecutions,
		)
		return
	}

	agents, err := s.Runs.Store().GetAgentExecutions(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(w, agents)
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

	if exists {
		snapshot :=
			run.Snapshot()

		writeJSON(
			w,
			snapshot.LLMCalls,
		)
		return
	}

	calls, err := s.Runs.Store().GetLLMCalls(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(
		w,
		calls,
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

	if exists {
		snapshot :=
			run.Snapshot()

		writeJSON(
			w,
			snapshot.ToolCalls,
		)
		return
	}

	calls, err := s.Runs.Store().GetToolCalls(runID)
	if err != nil {
		http.Error(
			w,
			"run not found",
			http.StatusNotFound,
		)
		return
	}

	writeJSON(
		w,
		calls,
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
		if s.Runs.Store() != nil {
			if _, err := s.Runs.Store().GetRun(runID); err == nil {
				events, _ := s.Runs.Store().GetEvents(runID)
				w.Header().Set("Content-Type", "text/event-stream")
				for _, e := range events {
					data, _ := json.Marshal(e)
					fmt.Fprintf(w, "data: %s\n\n", data)
				}
				return
			}
		}

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
