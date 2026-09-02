package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"harnais/config"
	"harnais/graph"
	"harnais/tools"
)

func testSettingsServer(t *testing.T) *httptest.Server {
	t.Helper()

	store := config.NewStore(
		filepath.Join(t.TempDir(), "settings.json"),
	)

	api := NewServer(
		NewEventBus(),
		NewRunManager(),
		store,
		tools.NewWorkspace(
			filepath.Join(t.TempDir(), "ws"),
		),
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

	return server
}

func TestGetSettings(t *testing.T) {

	server := testSettingsServer(t)

	response, err :=
		http.Get(
			server.URL +
				"/api/settings",
		)

	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			response.StatusCode,
		)
	}

	var view config.SettingsView

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(
			&view,
		); err != nil {

		t.Fatalf("decode: %v", err)
	}

	if len(view.Providers) != 3 {
		t.Fatalf(
			"expected 3 providers, got %d",
			len(view.Providers),
		)
	}

	if view.Providers[0].ID != "openai" {
		t.Errorf(
			"expected openai provider, got %q",
			view.Providers[0].ID,
		)
	}
}

func TestUpdateSettings(t *testing.T) {

	server := testSettingsServer(t)

	body, _ :=
		json.Marshal(
			config.SettingsUpdate{
				Providers: map[string]config.Values{
					"openai": {
						"apiKey": "sk-test",
						"model":  "gpt-4o-mini",
					},
				},
			},
		)

	request, _ :=
		http.NewRequest(
			http.MethodPut,
			server.URL+
				"/api/settings",
			bytes.NewReader(body),
		)

	response, err :=
		http.DefaultClient.Do(request)

	if err != nil {
		t.Fatalf("PUT: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			response.StatusCode,
		)
	}

	var view config.SettingsView

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(
			&view,
		); err != nil {

		t.Fatalf("decode: %v", err)
	}

	openai :=
		view.Providers[0]

	if !openai.Configured {
		t.Error("expected configured=true after update")
	}

	if openai.Values["apiKey"] !=
		"sk-test" {
		t.Errorf(
			"expected apiKey, got %q",
			openai.Values["apiKey"],
		)
	}
}

func TestUpdateSettingsRejectsUnknownProvider(t *testing.T) {

	server := testSettingsServer(t)

	body, _ :=
		json.Marshal(
			config.SettingsUpdate{
				Providers: map[string]config.Values{
					"nope": {"apiKey": "x"},
				},
			},
		)

	request, _ :=
		http.NewRequest(
			http.MethodPut,
			server.URL+
				"/api/settings",
			bytes.NewReader(body),
		)

	response, err :=
		http.DefaultClient.Do(request)

	if err != nil {
		t.Fatalf("PUT: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode !=
		http.StatusBadRequest {
		t.Fatalf(
			"expected 400, got %d",
			response.StatusCode,
		)
	}
}

func TestTestSettings(t *testing.T) {

	server := testSettingsServer(t)

	body, _ :=
		json.Marshal(
			config.TestRequest{
				Provider: "openai",
				Values:   map[string]string{},
			},
		)

	request, _ :=
		http.NewRequest(
			http.MethodPost,
			server.URL+
				"/api/settings/test",
			bytes.NewReader(body),
		)

	response, err :=
		http.DefaultClient.Do(request)

	if err != nil {
		t.Fatalf("POST: %v", err)
	}

	defer response.Body.Close()

	var result config.TestResult

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(
			&result,
		); err != nil {

		t.Fatalf("decode: %v", err)
	}

	if result.OK {
		t.Error("expected ok=false for missing key")
	}

	if result.Message == "" {
		t.Error("expected an error message")
	}
}