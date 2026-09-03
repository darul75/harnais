package config

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestOpenCodeModels(t *testing.T) {

	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				w.Write(
					[]byte(`{"data":[
						{"providerID":"opencode","id":"big-pickle"},
						{"providerID":"opencode","id":"deepseek-v4-flash"},
						{"providerID":"deepseek","id":"deepseek-v4-flash"}
					]}`),
				)
			},
		),
	)

	defer server.Close()

	models :=
		OpenCodeModels(
			server.URL,
		)

	want := []string{
		"deepseek/deepseek-v4-flash",
		"opencode/big-pickle",
		"opencode/deepseek-v4-flash",
	}

	if len(models) != len(want) {
		t.Fatalf("got %v, want %v", models, want)
	}

	for i := range want {

		if models[i] != want[i] {
			t.Fatalf("got %v, want %v", models, want)
		}
	}
}

func TestTestOpenCodeModel(t *testing.T) {

	server := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				w.Write(
					[]byte(`{"data":[
						{"providerID":"opencode","id":"deepseek-v4-flash"}
					]}`),
				)
			},
		),
	)

	defer server.Close()

	if err := testOpenCodeModel(
		"opencode/deepseek-v4-flash",
		server.URL,
	); err != nil {
		t.Fatalf("full ref should be accepted: %v", err)
	}

	if err := testOpenCodeModel(
		"deepseek-v4-flash",
		server.URL,
	); err != nil {
		t.Fatalf("bare id should be accepted: %v", err)
	}

	if err := testOpenCodeModel(
		"opencode/nope",
		server.URL,
	); err == nil {
		t.Fatal("unknown model should be rejected")
	}
}

func TestStoreUpdateAndView(t *testing.T) {

	dir := t.TempDir()

	store := NewStore(
		filepath.Join(dir, "settings.json"),
	)

	if err := store.Update(
		SettingsUpdate{
			Providers: map[string]Values{
				"openai": {
					"apiKey": "sk-test-123",
					"model":  "gpt-4o-mini",
				},
			},
		},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}

	view := store.View()

	if len(view.Providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(view.Providers))
	}

	openai := view.Providers[0]

	if openai.ID != "openai" {
		t.Fatalf("expected openai first, got %q", openai.ID)
	}

	if !openai.Configured {
		t.Error("expected openai to be configured")
	}

	if openai.Values["apiKey"] != "sk-test-123" {
		t.Errorf("expected apiKey, got %q", openai.Values["apiKey"])
	}

	if store.Get("openai", "model") != "gpt-4o-mini" {
		t.Errorf("expected model via Get, got %q", store.Get("openai", "model"))
	}
}

func TestStoreOpenCodeProvider(t *testing.T) {

	store := NewStore(
		filepath.Join(
			t.TempDir(),
			"settings.json",
		),
	)

	view := store.View()

	var openCode ProviderView

	for _, provider := range view.Providers {

		if provider.ID == "opencode" {
			openCode = provider
		}
	}

	if openCode.ID != "opencode" {
		t.Fatal("expected opencode provider")
	}

	if openCode.Label != "OpenCode Zen" {
		t.Errorf(
			"expected OpenCode Zen label, got %q",
			openCode.Label,
		)
	}

	if len(openCode.Fields) != 4 ||
		openCode.Fields[0].Key != "serverUrl" ||
		openCode.Fields[1].Key != "model" {
		t.Errorf(
			"expected server url + model + planner + reviewer fields, got %+v",
			openCode.Fields,
		)
	}

	if len(openCode.Fields[1].Suggestions) == 0 {
		t.Error("expected model suggestions")
	}

	if err := store.Update(
		SettingsUpdate{
			Providers: map[string]Values{
				"opencode": {
					"model": "opencode/deepseek-v4-flash",
				},
			},
		},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := store.Get("opencode", "model"); got != "opencode/deepseek-v4-flash" {
		t.Errorf("expected stored model, got %q", got)
	}

	view = store.View()

	for _, provider := range view.Providers {

		if provider.ID == "opencode" && !provider.Configured {
			t.Error("expected opencode configured after setting model")
		}
	}
}

func TestTestOpenCodeEmptyModel(t *testing.T) {

	err := Test(
		"opencode",
		map[string]string{},
	)

	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestStorePersistsAcrossInstances(t *testing.T) {

	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	first := NewStore(path)

	if err := first.Update(
		SettingsUpdate{
			Providers: map[string]Values{
				"openai": {
					"apiKey": "sk-persisted",
					"model":  "gpt-4o",
				},
			},
		},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}

	second := NewStore(path)

	if got := second.Get("openai", "apiKey"); got != "sk-persisted" {
		t.Errorf("expected persisted apiKey, got %q", got)
	}
}

func TestStoreSeedsFromEnv(t *testing.T) {

	t.Setenv("OPENAI_API_KEY", "sk-env")
	t.Setenv("OPENAI_MODEL", "gpt-env")

	store := NewStore(
		filepath.Join(
			t.TempDir(),
			"settings.json",
		),
	)

	if got := store.Get("openai", "apiKey"); got != "sk-env" {
		t.Errorf("expected env apiKey, got %q", got)
	}

	if got := store.Get("openai", "model"); got != "gpt-env" {
		t.Errorf("expected env model, got %q", got)
	}
}

func TestStoreRejectsUnknownProvider(t *testing.T) {

	store := NewStore("")

	err := store.Update(
		SettingsUpdate{
			Providers: map[string]Values{
				"not-a-provider": {"apiKey": "x"},
			},
		},
	)

	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestStoreDefaultPath(t *testing.T) {

	path := defaultPath()

	if path == "" {
		t.Fatal("expected non-empty default path")
	}
}
