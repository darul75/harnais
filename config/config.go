// Package config holds runtime-configurable provider settings
// (API keys, models, ...) and exposes them to the rest of the app.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"harnais/agent"
	"harnais/llm"
)

// FieldType describes how a setting is edited in the UI.
type FieldType string

const (
	// FieldSecret is a value hidden behind a password input.
	FieldSecret FieldType = "secret"

	// FieldString is a plain text value.
	FieldString FieldType = "string"
)

// Field describes one configurable setting of a provider.
type Field struct {
	Key string `json:"key"`

	Label string `json:"label"`

	Type FieldType `json:"type"`

	Placeholder string `json:"placeholder,omitempty"`

	// EnvVar is the environment variable used to seed the value
	// when nothing is persisted yet.
	EnvVar string `json:"envVar,omitempty"`

	// Secret marks a value that should be rendered as a password
	// input. Kept alongside Type for convenience.
	Secret bool `json:"secret,omitempty"`
}

// Provider describes a service provider and its configurable fields.
type Provider struct {
	ID string `json:"id"`

	Label string `json:"label"`

	Fields []Field `json:"fields"`
}

// Values holds the current field values of a provider.
type Values map[string]string

// Providers is the catalog of services the harness can be configured for.
// Add new entries here when new agents/providers are introduced.
var providers = []Provider{
	{
		ID:    "openai",
		Label: "OpenAI",
		Fields: []Field{
			{
				Key:         "apiKey",
				Label:       "API Key",
				Type:        FieldSecret,
				Placeholder: "sk-...",
				EnvVar:      "OPENAI_API_KEY",
				Secret:      true,
			},
			{
				Key:         "model",
				Label:       "Model",
				Type:        FieldString,
				Placeholder: "gpt-4o-mini",
				EnvVar:      "OPENAI_MODEL",
			},
		},
	},
}

// Store holds the current provider settings and persists them to disk.
type Store struct {
	mu sync.RWMutex

	providers []Provider

	byID map[string]Provider

	values map[string]Values

	path string
}

// NewStore builds a store seeded from environment variables and the
// optional persisted settings file at path. An empty path disables
// persistence (in-memory only).
func NewStore(path string) *Store {

	if path == "" {
		path = defaultPath()
	}

	store := &Store{
		providers: providers,
		byID:      map[string]Provider{},
		values:    map[string]Values{},
		path:      path,
	}

	for _, provider := range providers {

		store.byID[provider.ID] =
			provider
	}

	store.seedFromEnv()
	store.load()

	return store
}

func defaultPath() string {

	if dir, err :=
		os.UserHomeDir(); err == nil {

		return filepath.Join(
			dir,
			".harnais",
			"settings.json",
		)
	}

	return ".harnais-settings.json"
}

// ------------------------------------------------------------
// Seeding / loading
// ------------------------------------------------------------

func (s *Store) seedFromEnv() {

	for _, provider := range s.providers {

		values := Values{}

		for _, field := range provider.Fields {

			if field.EnvVar == "" {
				continue
			}

			if value :=
				os.Getenv(field.EnvVar); value != "" {

				values[field.Key] =
					value
			}
		}

		if len(values) > 0 {
			s.values[provider.ID] =
				values
		}
	}
}

func (s *Store) load() {

	data, err :=
		os.ReadFile(s.path)

	if err != nil {
		return
	}

	var file struct {
		Providers map[string]Values `json:"providers"`
	}

	if err :=
		json.Unmarshal(data, &file); err != nil {

		return
	}

	for id, values := range file.Providers {

		if _, ok := s.byID[id]; !ok {
			continue
		}

		s.values[id] =
			values
	}
}

func (s *Store) persist() error {

	if s.path == "" {
		return nil
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(s.path),
			0o700,
		); err != nil {

		return err
	}

	data, err :=
		json.MarshalIndent(
			map[string]any{
				"providers": s.values,
			},
			"",
			"  ",
		)

	if err != nil {
		return err
	}

	tmp :=
		s.path + ".tmp"

	if err :=
		os.WriteFile(
			tmp,
			data,
			0o600,
		); err != nil {

		return err
	}

	return os.Rename(tmp, s.path)
}

// ------------------------------------------------------------
// API views
// ------------------------------------------------------------

// SettingsView is the API payload exposing provider schema + values.
type SettingsView struct {
	Providers []ProviderView `json:"providers"`
}

type ProviderView struct {
	ID string `json:"id"`

	Label string `json:"label"`

	Fields []Field `json:"fields"`

	Values map[string]string `json:"values"`

	Configured bool `json:"configured"`
}

// View returns the current settings plus the provider schema.
func (s *Store) View() SettingsView {

	s.mu.RLock()
	defer s.mu.RUnlock()

	view := SettingsView{
		Providers: make(
			[]ProviderView,
			0,
			len(s.providers),
		),
	}

	for _, provider := range s.providers {

		values :=
			s.values[provider.ID]

		if values == nil {
			values = Values{}
		}

		view.Providers =
			append(
				view.Providers,
				ProviderView{
					ID:     provider.ID,
					Label:  provider.Label,
					Fields: provider.Fields,
					Values: values,
					Configured: providerConfigured(
						provider,
						values,
					),
				},
			)
	}

	return view
}

func providerConfigured(
	provider Provider,
	values Values,
) bool {

	for _, field := range provider.Fields {

		if field.Type !=
			FieldSecret {
			continue
		}

		if values[field.Key] != "" {
			return true
		}
	}

	return false
}

// SettingsUpdate is the API payload for updating settings.
type SettingsUpdate struct {
	Providers map[string]Values `json:"providers"`
}

// Update merges the provided values into the store and persists them.
func (s *Store) Update(
	update SettingsUpdate,
) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	next :=
		map[string]Values{}

	for _, provider := range s.providers {

		next[provider.ID] =
			cloneValues(
				s.values[provider.ID],
			)
	}

	for id, values := range update.Providers {

		provider, ok :=
			s.byID[id]

		if !ok {
			return fmt.Errorf(
				"unknown provider %q",
				id,
			)
		}

		for key := range values {

			if !providerHasField(
				provider,
				key,
			) {
				return fmt.Errorf(
					"unknown field %q for provider %q",
					key,
					id,
				)
			}
		}

		next[id] =
			values
	}

	s.values = next

	return s.persist()
}

// ------------------------------------------------------------
// Read access
// ------------------------------------------------------------

// Get returns the current value for a provider field.
func (s *Store) Get(
	providerID string,
	field string,
) string {

	s.mu.RLock()
	defer s.mu.RUnlock()

	if values, ok :=
		s.values[providerID]; ok {

		return values[field]
	}

	return ""
}

// LLMFactory returns a factory building an LLM from the current
// stored settings. The factory is evaluated lazily so updates take
// effect on the next run without a restart.
func (s *Store) LLMFactory(
	kind string,
) func() agent.LLM {

	return func() agent.LLM {

		switch kind {

		case "openai":
			return llm.NewOpenAI(
				s.Get(
					"openai",
					"apiKey",
				),
				s.Get(
					"openai",
					"model",
				),
			)

		default:
			return llm.NewOpenAI(
				s.Get(
					"openai",
					"apiKey",
				),
				s.Get(
					"openai",
					"model",
				),
			)
		}
	}
}

// ------------------------------------------------------------
// Testing
// ------------------------------------------------------------

// TestRequest is the API payload for validating provider values.
type TestRequest struct {
	Provider string `json:"provider"`

	Values map[string]string `json:"values"`
}

// TestResult is the API response for a provider validation.
type TestResult struct {
	OK bool `json:"ok"`

	Message string `json:"message,omitempty"`
}

// Test validates provider credentials/values without a full run.
func Test(
	providerID string,
	values map[string]string,
) error {

	switch providerID {

	case "openai":
		return llm.TestOpenAI(
			values["apiKey"],
			values["model"],
		)

	default:
		return fmt.Errorf(
			"unknown provider %q",
			providerID,
		)
	}
}

// ------------------------------------------------------------
// Helpers
// ------------------------------------------------------------

func cloneValues(
	values Values,
) Values {

	result :=
		make(
			Values,
			len(values),
		)

	for key, value := range values {
		result[key] = value
	}

	return result
}

func providerHasField(
	provider Provider,
	key string,
) bool {

	for _, field := range provider.Fields {

		if field.Key == key {
			return true
		}
	}

	return false
}