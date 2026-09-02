package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TTSRequest holds the parameters for an audio/speech synthesis call.
type TTSRequest struct {
	// Text is the source text to turn into speech.
	Text string

	// Voice is the OpenAI voice name (e.g. "alloy").
	Voice string

	// Model is the TTS model (e.g. "gpt-4o-mini-tts" or "tts-1").
	Model string

	// Format is the audio response format ("mp3" by default).
	Format string
}

type ttsRequestBody struct {
	Model          string `json:"model"`
	Voice          string `json:"voice"`
	Input          string `json:"input"`
	ResponseFormat string `json:"response_format"`
}

// TTS synthesizes speech for Text and returns the raw audio bytes.
// It mirrors the API-key/BaseURL behavior of the OpenAI Responses
// client so the same settings drive both text and audio generation.
func (o *OpenAI) TTS(
	ctx context.Context,
	req TTSRequest,
) ([]byte, error) {

	if o.APIKey == "" {
		return nil, fmt.Errorf(
			"OPENAI_API_KEY is not configured",
		)
	}

	if req.Text == "" {
		return nil, fmt.Errorf(
			"tts: text is empty",
		)
	}

	model :=
		req.Model

	if model == "" {
		model = "tts-1"
	}

	voice :=
		req.Voice

	if voice == "" {
		voice = "alloy"
	}

	format :=
		req.Format

	if format == "" {
		format = "mp3"
	}

	body, err :=
		json.Marshal(
			ttsRequestBody{
				Model:          model,
				Voice:          voice,
				Input:          req.Text,
				ResponseFormat: format,
			},
		)

	if err != nil {
		return nil, err
	}

	baseURL :=
		o.BaseURL

	if baseURL == "" {
		baseURL =
			"https://api.openai.com"
	}

	httpReq, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			baseURL+"/v1/audio/speech",
			bytes.NewReader(body),
		)

	if err != nil {
		return nil, err
	}

	httpReq.Header.Set(
		"Authorization",
		"Bearer "+o.APIKey,
	)

	httpReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	client :=
		o.Client

	if client == nil {
		client = http.DefaultClient
	}

	resp, err :=
		client.Do(httpReq)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 ||
		resp.StatusCode >= 300 {

		errorBody, _ :=
			io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"openai audio API returned %s: %s",
			resp.Status,
			string(errorBody),
		)
	}

	return io.ReadAll(resp.Body)
}
