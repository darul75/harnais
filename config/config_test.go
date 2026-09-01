package config

import (
	"path/filepath"
	"testing"
)

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

	if len(view.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(view.Providers))
	}

	openai := view.Providers[0]

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

	store := NewStore("")

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