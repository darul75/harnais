package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

func newReportServer(t *testing.T) (*httptest.Server, *tools.Workspace, *RunManager) {
	t.Helper()

	workspace := tools.NewWorkspace(
		filepath.Join(t.TempDir(), "ws"),
	)

	runs := NewRunManager()

	api := NewServer(
		NewEventBus(),
		runs,
		config.NewStore(
			filepath.Join(t.TempDir(), "settings.json"),
		),
		workspace,
		func(request StartRunRequest) *graph.Run {
			return nil
		},
		func() []WorkflowInfo {
			return nil
		},
		func(id string) (*WorkflowDetail, bool) {
			return nil, false
		},
		graph.NewQuestionHub(),
	)

	server := httptest.NewServer(api.Handler())

	t.Cleanup(server.Close)

	return server, workspace, runs
}

func writeReport(
	t *testing.T,
	workspace *tools.Workspace,
	runID string,
	name string,
	content string,
) {
	t.Helper()

	reportPath, err :=
		workspace.Resolve(
			filepath.Join(
				"reports",
				runID,
				name,
			),
		)

	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(reportPath),
			0o755,
		); err != nil {

		t.Fatalf("MkdirAll: %v", err)
	}

	if err :=
		os.WriteFile(
			reportPath,
			[]byte(content),
			0o644,
		); err != nil {

		t.Fatalf("WriteFile: %v", err)
	}
}

func TestListAndGetRunReports(t *testing.T) {

	server, workspace, runs :=
		newReportServer(t)

	runID := "run-123"

	runs.Add(
		graph.NewRun(runID, nil, graph.State{}),
		RunMeta{Task: "t"},
	)

	writeReport(
		t,
		workspace,
		runID,
		"test.md",
		"# Hello\n\nSome **markdown**.",
	)

	// Run-scoped listing.
	listResponse, err :=
		http.Get(
			server.URL +
				"/api/runs/" +
				runID +
				"/reports",
		)

	if err != nil {
		t.Fatalf("GET run reports: %v", err)
	}

	defer listResponse.Body.Close()

	if listResponse.StatusCode !=
		http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			listResponse.StatusCode,
		)
	}

	var payload struct {
		Reports []reportInfo `json:"reports"`
	}

	if err :=
		json.NewDecoder(
			listResponse.Body,
		).Decode(
			&payload,
		); err != nil {

		t.Fatalf("decode: %v", err)
	}

	if len(payload.Reports) != 1 {
		t.Fatalf(
			"expected 1 report, got %d",
			len(payload.Reports),
		)
	}

	if payload.Reports[0].RunID != runID ||
		payload.Reports[0].Name != "test.md" {
		t.Errorf(
			"unexpected report %+v",
			payload.Reports[0],
		)
	}

	// Report content.
	contentResponse, err :=
		http.Get(
			server.URL +
				"/api/runs/" +
				runID +
				"/reports/test.md",
		)

	if err != nil {
		t.Fatalf("GET report: %v", err)
	}

	defer contentResponse.Body.Close()

	if contentResponse.StatusCode !=
		http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			contentResponse.StatusCode,
		)
	}

	if ct :=
		contentResponse.Header.Get(
			"Content-Type",
		); ct !=
		"text/markdown; charset=utf-8" {

		t.Errorf(
			"unexpected content type %q",
			ct,
		)
	}
}

func TestListAllReportsGroupsByRun(t *testing.T) {

	server, workspace, runs :=
		newReportServer(t)

	for _, runID := range []string{"run-a", "run-b"} {

		runs.Add(
			graph.NewRun(runID, nil, graph.State{}),
			RunMeta{Task: "t"},
		)

		writeReport(
			t,
			workspace,
			runID,
			"brief.md",
			"# brief",
		)
	}

	response, err :=
		http.Get(
			server.URL +
				"/api/reports",
		)

	if err != nil {
		t.Fatalf("GET all reports: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			response.StatusCode,
		)
	}

	var payload struct {
		Reports []reportInfo `json:"reports"`
	}

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(
			&payload,
		); err != nil {

		t.Fatalf("decode: %v", err)
	}

	if len(payload.Reports) != 2 {
		t.Fatalf(
			"expected 2 reports, got %d",
			len(payload.Reports),
		)
	}

	seen := map[string]bool{}

	for _, report := range payload.Reports {
		seen[report.RunID] = true
	}

	if !seen["run-a"] || !seen["run-b"] {
		t.Errorf(
			"expected both runs, got %v",
			payload.Reports,
		)
	}
}

func TestRunReportsRequireExistingRun(t *testing.T) {

	server, _, _ :=
		newReportServer(t)

	response, err :=
		http.Get(
			server.URL +
				"/api/runs/nope/reports",
		)

	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusNotFound {
		t.Fatalf(
			"expected 404, got %d",
			response.StatusCode,
		)
	}
}

func TestGetRunReportNotFound(t *testing.T) {

	server, _, runs :=
		newReportServer(t)

	runID := "run-404"

	runs.Add(
		graph.NewRun(runID, nil, graph.State{}),
		RunMeta{Task: "t"},
	)

	response, err :=
		http.Get(
			server.URL +
				"/api/runs/" +
				runID +
				"/reports/nope.md",
		)

	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusNotFound {
		t.Fatalf(
			"expected 404, got %d",
			response.StatusCode,
		)
	}
}
