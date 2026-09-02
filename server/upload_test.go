package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

func newUploadServer(t *testing.T) (*httptest.Server, *tools.Workspace) {
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

func uploadMultipart(
	t *testing.T,
	serverURL string,
	filename string,
	content []byte,
) (string, int) {
	t.Helper()

	var body bytes.Buffer

	writer :=
		multipart.NewWriter(&body)

	part, err :=
		writer.CreateFormFile("file", filename)

	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}

	if _, err :=
		part.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err :=
		writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	request, err :=
		http.NewRequest(
			http.MethodPost,
			serverURL+"/api/upload",
			&body,
		)

	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	response, err :=
		http.DefaultClient.Do(request)

	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", response.StatusCode
	}

	var result struct {
		Path string `json:"path"`
	}

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(&result); err != nil {

		t.Fatalf("Decode: %v", err)
	}

	return result.Path, response.StatusCode
}

func TestUploadSameNameDistinctPaths(t *testing.T) {

	server, workspace :=
		newUploadServer(t)

	path1, status1 :=
		uploadMultipart(
			t,
			server.URL,
			"test.pdf",
			[]byte("%PDF-1.4 fake one"),
		)

	if status1 != http.StatusOK {
		t.Fatalf("first upload status = %d", status1)
	}

	path2, status2 :=
		uploadMultipart(
			t,
			server.URL,
			"test.pdf",
			[]byte("%PDF-1.4 fake two"),
		)

	if status2 != http.StatusOK {
		t.Fatalf("second upload status = %d", status2)
	}

	if path1 == path2 {
		t.Fatalf("expected distinct paths, both = %q", path1)
	}

	resolved1, err :=
		workspace.Resolve(path1)

	if err != nil {
		t.Fatalf("Resolve path1: %v", err)
	}

	resolved2, err :=
		workspace.Resolve(path2)

	if err != nil {
		t.Fatalf("Resolve path2: %v", err)
	}

	for _, resolved := range []string{
		resolved1,
		resolved2,
	} {

		info, err :=
			os.Stat(resolved)

		if err != nil {
			t.Fatalf("Stat %s: %v", resolved, err)
		}

		if !info.Mode().IsRegular() {
			t.Errorf("expected regular file, got %s", info.Mode())
		}
	}
}

func TestUploadRejectsNonPDF(t *testing.T) {

	server, _ :=
		newUploadServer(t)

	_, status :=
		uploadMultipart(
			t,
			server.URL,
			"notes.txt",
			[]byte("not a pdf"),
		)

	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
}

func TestGetUploadServesPDF(t *testing.T) {

	server, _ :=
		newUploadServer(t)

	content := []byte("%PDF-1.4 fake content")

	path, status :=
		uploadMultipart(
			t,
			server.URL,
			"doc.pdf",
			content,
		)

	if status != http.StatusOK {
		t.Fatalf("upload status = %d", status)
	}

	name :=
		path[strings.LastIndex(path, "/")+1:]

	response, err :=
		http.Get(
			server.URL + "/api/uploads/" + name,
		)

	if err != nil {
		t.Fatalf("Get upload: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET upload status = %d", response.StatusCode)
	}

	if got :=
		response.Header.Get("Content-Type"); !strings.HasPrefix(
			got,
			"application/pdf",
		) {

		t.Errorf("expected application/pdf, got %q", got)
	}

	buf :=
		new(bytes.Buffer)

	if _, err :=
		buf.ReadFrom(response.Body); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), content) {
		t.Errorf("served bytes differ from uploaded bytes")
	}
}