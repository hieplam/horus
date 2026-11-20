// The tool generate_speech.
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/speak"
)

// Speech is what the tool generate_speech needs from the speak service
// internal/speak. cmd/horus gives the server a *speak.Service. A test gives
// it a fake.
type Speech interface {
	// Speak makes one speech file.
	Speak(ctx context.Context, r speak.Request, o speak.Options) (*speak.Result, error)
	// StopWorkers ends each speech worker of this server. uninstall_workflow
	// calls it before it deletes the model files of a speech workflow.
	StopWorkers()
}

// SpeechResult is the result of generate_speech, in the key order that the tool shows.
type SpeechResult = speak.Result

// speechInStack is the error of generate_speech in the stack: the stack runs
// only image jobs, and the API process never runs a backend for a job (ADR
// 0025, ADR 0038).
func speechInStack() *herr.Error {
	e := herr.New("BACKEND_UNAVAILABLE", "horus in the Docker stack makes images only: generate_speech does not run in the stack",
		map[string]any{"tool": "generate_speech", "stack": true})
	e.Retryable = ptr(false)
	e.NextStep = "Tell the user that horus in the Docker stack cannot make speech. For speech, the user registers the horus " +
		"program of this machine (horus register) and sets up the speech workflow (horus setup <workflow> -yes)."
	return e
}

// generateSpeech is the tool. In the stack (inStack), each call gets
// speechInStack.
func generateSpeech(speech Speech, inStack bool) tool {
	return tool{
		name:  "generate_speech",
		title: "Generate speech",
		description: "Speak one text with the given speech workflow and save the audio file on this machine. The call waits " +
			"until the file is done (a few seconds for a sentence; the first call of a session also loads the models, " +
			"5 to 20 s) and reports progress. The text is the whole input: horus sends it as written and adds no words. " +
			"The result has the absolute path of the audio file and its transcript: the words that horus read back from " +
			"the file. Compare the transcript with your text. horus does not compare them.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(false), IdempotentHint: false, OpenWorldHint: ptr(false)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows of the kind speech, for example 'vi-speech'."),
			f("text", tString).d("The words to speak, sent as written. Follow text.rules of describe_workflow. " +
				"A [tag] in the text must be one of tags of describe_workflow."),
			f("voice", tString).null().d("A name from voices of describe_workflow. Default: default_voice of describe_workflow.").dflt(nil),
			f("out_path", tString).null().d("Absolute path of the audio file to write. It ends with .wav or .mp3, and the end " +
				"gives the format. Default: a new .wav file in the horus output folder.").dflt(nil),
			f("overwrite", tBool).d("Replace out_path when it exists.").dflt(false),
		},
		output: envelope("SpeechResult", obj{
			"SpeechResult": model("SpeechResult",
				f("workflow", tString),
				f("voice", tString).d("The voice that the call used."),
				f("path", tString).d("Absolute path of the audio file on this machine."),
				f("bytes", tInt),
				f("format", tEnum("wav", "mp3")).d("wav: 48 kHz, 16-bit, mono. mp3: 44.1 kHz, 128 kbps, mono."),
				f("audio_seconds", tNumber).d("The length of the audio."),
				f("transcript", tString).d("The words that horus read back from the audio file. Compare them with your text."),
				f("seconds", tNumber).d("Wall time of this call."),
				f("started_worker", tBool).d("True: this call had to start the speech worker first."),
			),
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			if inStack {
				return nil, speechInStack()
			}
			res, err := speech.Speak(ctx, speak.Request{
				Workflow:  c.args.str("workflow"),
				Text:      c.args.str("text"),
				Voice:     c.args.strPtr("voice"),
				OutPath:   c.args.strPtr("out_path"),
				Overwrite: c.args.boolean("overwrite"),
			}, speak.Options{Report: speak.Report(c.report)})
			if err != nil {
				return nil, err
			}
			return res, nil
		},
	}
}
