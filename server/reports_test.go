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

func newReportServer(t *testing.T) (*httptest.Server, *tools.Workspace) {
	t.Helper()

	workspace := tools.NewWorkspace(
		filepath.Join(t.TempDir(), "ws"),
	)

	api := NewServer(
		NewEventBus(),
		NewRunManager(),
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
	)

	server := httptest.NewServer(api.Handler())

	t.Cleanup(server.Close)

	return server, workspace
}

func TestListAndGetReports(t *testing.T) {

	server, workspace :=
		newReportServer(t)

	reportPath, err :=
		workspace.Resolve(
			"reports/test.md",
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
			[]byte("# Hello\n\nSome **markdown**."),
			0o644,
		); err != nil {

		t.Fatalf("WriteFile: %v", err)
	}

	listResponse, err :=
		http.Get(
			server.URL +
				"/api/reports",
		)

	if err != nil {
		t.Fatalf("GET reports: %v", err)
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

	if payload.Reports[0].Name != "test.md" {
		t.Errorf(
			"expected test.md, got %q",
			payload.Reports[0].Name,
		)
	}

	contentResponse, err :=
		http.Get(
			server.URL +
				"/api/reports/test.md",
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

func TestGetReportNotFound(t *testing.T) {

	server, _ :=
		newReportServer(t)

	response, err :=
		http.Get(
			server.URL +
				"/api/reports/nope.md",
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