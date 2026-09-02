package workflows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"harnais/graph"
	"harnais/llm"
)

const TTSWorkflowID = "tts"

// TTSWorkflow converts the user's typed text into spoken audio
// (MP3) using OpenAI's audio/speech API. The text is synthesized
// as-is; no LLM transforms it first.
func TTSWorkflow(
	s *Shared,
) *Workflow {

	return &Workflow{
		ID: TTSWorkflowID,

		Title: "Text-to-Speech",

		Description: "Convert your typed text into spoken audio (MP3) via OpenAI TTS, then listen to it in the run view.",

		Keywords: []string{
			"speech",
			"audio",
			"tts",
			"voice",
			"listen",
			"read aloud",
			"narration",
			"say",
			"speak",
		},

		Build: func() *graph.Graph {

			g :=
				graph.NewGraph()

			addNode(
				g,
				&graph.Node{
					ID: "tts",

					Worker: s.TTSWorker(),
				},
			)

			addNode(
				g,
				&graph.Node{
					ID: "write_audio",

					Worker: s.WriteAudio(),
				},
			)

			addEdge(g, "tts", "write_audio")

			return g
		},
	}
}

// TTSWorker generates speech for the task text via OpenAI and
// hands the raw audio bytes to the next node through state.
func (s *Shared) TTSWorker() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"tts_speech",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			text, ok :=
				state["task"].(string)

			if !ok || strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf(
					"tts: task (text to speak) is missing",
				)
			}

			client :=
				llm.NewOpenAI(
					s.store.Get(
						"openai",
						"apiKey",
					),
					"",
				)

			data, err :=
				client.TTS(
					ctx,
					llm.TTSRequest{
						Text: text,

						Voice: s.store.Get(
							"openai",
							"ttsVoice",
						),

						Model: s.store.Get(
							"openai",
							"ttsModel",
						),
					},
				)

			if err != nil {
				return nil, err
			}

			return graph.State{
				"audio_bytes": data,

				"audio_text": text,
			}, nil
		},
	)
}

// WriteAudio persists the synthesized audio bytes into the run's
// reports directory and records the saved path in state.
func (s *Shared) WriteAudio() *graph.FuncWorker {

	return graph.NewFuncWorker(
		"write_audio",

		func(
			ctx context.Context,
			state graph.State,
		) (graph.State, error) {

			data, ok :=
				state["audio_bytes"].([]byte)

			if !ok || len(data) == 0 {
				return nil, fmt.Errorf(
					"write_audio: no audio bytes in state",
				)
			}

			executionContext, ok :=
				graph.GetExecutionContext(
					ctx,
				)

			if !ok {
				return nil, fmt.Errorf(
					"write_audio: missing execution context",
				)
			}

			name := "speech.mp3"

			relative :=
				filepath.Join(
					"reports",
					executionContext.RunID,
					name,
				)

			resolved, err :=
				s.workspace.Resolve(
					relative,
				)

			if err != nil {
				return nil, err
			}

			if err :=
				os.MkdirAll(
					filepath.Dir(resolved),
					0o755,
				); err != nil {

				return nil, err
			}

			if err :=
				os.WriteFile(
					resolved,
					data,
					0o644,
				); err != nil {

				return nil, err
			}

			return graph.State{
				"audio_path": relative,

				"audio_name": name,
			}, nil
		},
	)
}
