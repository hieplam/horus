// Tests of the tool generate_speech as a handler, with a fake of the speak
// service. The service itself is tested in internal/speak. The whole path
// with the fake speech worker is tested in cmd/horus (speech_test.go) and
// in tests/mcp_contract.py.
package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/speak"
)

// fakeSpeech is a fake of the speak service. Speak records the request,
// sends one progress report and returns result or err.
type fakeSpeech struct {
	mu     sync.Mutex
	reqs   []speak.Request
	result *speak.Result
	err    error
	stops  int // the calls of StopWorkers
}

func (f *fakeSpeech) StopWorkers() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
}

func (f *fakeSpeech) Speak(ctx context.Context, r speak.Request, o speak.Options) (*speak.Result, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	o.Report(ctx, 0, 0, "making the speech file")
	return f.result, nil
}

func TestGenerateSpeechToolCallsTheService(t *testing.T) {
	useRepoWorkflows(t)
	speech := &fakeSpeech{result: &speak.Result{Workflow: "vi-speech", Voice: "Mai Anh", Path: "/x/a.mp3", Bytes: 9, Format: "mp3",
		AudioSeconds: 3.2, Transcript: "Xin chào.", Seconds: 6.1, StartedWorker: true}}
	got := make(chan string, 16)
	cs := connect(t, New(&fakeImages{}, nil, speech), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		got <- r.Params.Message
	}})
	params := &mcp.CallToolParams{Name: "generate_speech", Arguments: json.RawMessage(
		`{"workflow": "vi-speech", "text": "Xin chào [cười] bạn.", "voice": "Mai Anh", "out_path": "/x/a.mp3", "overwrite": true}`)}
	params.SetProgressToken("tok-speech")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	env := envelopeOf(t, res)
	r, _ := env["result"].(map[string]any)
	if res.IsError || r["transcript"] != "Xin chào." || r["path"] != "/x/a.mp3" || r["started_worker"] != true || r["audio_seconds"] != 3.2 {
		t.Fatalf("got %v", env)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	order := []string{`"workflow"`, `"voice"`, `"path"`, `"bytes"`, `"format"`, `"audio_seconds"`, `"transcript"`, `"seconds"`, `"started_worker"`}
	last := -1
	for _, k := range order {
		i := strings.Index(text, k)
		if i < last {
			t.Fatalf("the key %s is not in the order of the spec: %s", k, text)
		}
		last = i
	}
	if len(speech.reqs) != 1 {
		t.Fatalf("requests %v", speech.reqs)
	}
	q := speech.reqs[0]
	if q.Workflow != "vi-speech" || q.Text != "Xin chào [cười] bạn." || q.Voice == nil || *q.Voice != "Mai Anh" ||
		q.OutPath == nil || *q.OutPath != "/x/a.mp3" || !q.Overwrite {
		t.Fatalf("request %+v", q)
	}
	if m := nextProgress(t, got); m != "making the speech file" {
		t.Fatalf("progress: %q", m)
	}

	// The optional arguments are nil without a value, and a horus error is the envelope.
	speech.err = herr.New("WORKFLOW_NOT_INSTALLED", "parts of the speech setup are missing: ffmpeg", map[string]any{"missing": []string{"ffmpeg"}})
	env = envelopeOf(t, callTool(t, cs, "generate_speech", map[string]any{"workflow": "vi-speech", "text": "Xin chào."}))
	if e, _ := env["error"].(map[string]any); env["ok"] != false || e["code"] != "WORKFLOW_NOT_INSTALLED" {
		t.Fatalf("got %v", env)
	}
	if q := speech.reqs[1]; q.Voice != nil || q.OutPath != nil || q.Overwrite {
		t.Fatalf("request without the optional arguments %+v", q)
	}
}

// TestGenerateSpeechArgumentsNameNoSpeechSetup: the caller gives a text and a
// voice. No argument names an engine, Python, ffmpeg or the loudness (the
// goal row speech-hidden).
func TestGenerateSpeechArgumentsNameNoSpeechSetup(t *testing.T) {
	tl := generateSpeech(nil, false)
	names := []string{}
	for _, a := range tl.args {
		names = append(names, a.name)
		text := strings.ToLower(a.name + " " + a.desc)
		for _, word := range []string{"engine", "python", "ffmpeg", "ffprobe", "loudness", "lufs", "venv", "vieneu", "whisper"} {
			if strings.Contains(text, word) {
				t.Errorf("the argument %s names %q: %s", a.name, word, a.desc)
			}
		}
	}
	if strings.Join(names, ",") != "workflow,text,voice,out_path,overwrite" {
		t.Fatalf("arguments %v", names)
	}
}
