package speak

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speechsetup"
)

// The speech workflow of the tests: text of 2 to 40 characters, 2 voices,
// 2 tags, 2 formats, and one model file of 5 bytes.
const talkManifest = `{"name": "talk", "kind": "speech", "title": "Talk", "summary": "A test speech workflow.",
 "text": {"min_chars": 2, "max_chars": 40, "rules": ["Write the words."]},
 "voices": ["alpha", "beta"], "default_voice": "alpha", "tags": ["[laugh]", "[sigh]"], "formats": ["wav", "mp3"]}`

const talkModels = `{"repo": "x/talk", "files": [{"role": "speech", "repo_path": "model.bin", "folder": "tts", "bytes": 5, "sha256": "0"}]}`

// fakeWorker is a speech worker in the test process. By default it writes 4
// bytes to the out path and answers with the text as the transcript.
type fakeWorker struct {
	mu      sync.Mutex
	texts   []Text
	running bool
	closed  bool
	answer  func(ctx context.Context, t Text) (Speech, error) // nil: the default answer
}

func (f *fakeWorker) Speak(ctx context.Context, t Text) (Speech, error) {
	f.mu.Lock()
	f.texts = append(f.texts, t)
	started := !f.running
	f.running = true
	answer := f.answer
	f.mu.Unlock()
	if answer != nil {
		return answer(ctx, t)
	}
	if err := os.MkdirAll(filepath.Dir(t.Out), 0o755); err != nil {
		return Speech{}, err
	}
	if err := os.WriteFile(t.Out, []byte("RIFF"), 0o644); err != nil {
		return Speech{}, err
	}
	return Speech{Path: t.Out, Bytes: 4, Format: t.Format, AudioSeconds: 0.5, Transcript: t.Text, Started: started}, nil
}

func (f *fakeWorker) Running() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.running }

func (f *fakeWorker) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed, f.running = true, false
	return nil
}

// fakePrograms finds each program except the ones in missing.
type fakePrograms struct{ missing map[string]bool }

func (p fakePrograms) LookPath(name string) (string, error) {
	if p.missing[name] {
		return "", fs.ErrNotExist
	}
	return "/bin/" + name, nil
}

// env is one test: a workflows folder with the speech workflow talk and the
// image workflow it-illustration, a speech folder with each part of the
// speech setup, and an output folder.
type env struct {
	t        *testing.T
	root     string
	place    speechsetup.Place
	worker   *fakeWorker
	made     []speechsetup.Place // the places of the workers that the service made
	programs fakePrograms
	svc      *Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "workflows", "talk", "manifest.json"), talkManifest)
	write(filepath.Join(root, "workflows", "talk", "models.json"), talkModels)
	image, err := os.ReadFile(filepath.Join("..", "..", "workflows", "it-illustration", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "workflows", "it-illustration", "manifest.json"), string(image))
	t.Setenv("HORUS_ROOT", root) // a repo root without .env, so no setting comes from the .env of this machine
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(root, "workflows"))
	t.Setenv("HORUS_SPEECH_DIR", filepath.Join(root, "speech"))
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(root, "out"))
	t.Setenv("HORUS_GENERATE_TIMEOUT", "")
	settings.Reset()
	t.Cleanup(settings.Reset)
	e := &env{t: t, root: root, place: speechsetup.At(filepath.Join(root, "speech")), worker: &fakeWorker{}, programs: fakePrograms{missing: map[string]bool{}}}
	write(e.place.Python, "python")
	write(filepath.Join(e.place.Models, "tts", "model.bin"), "12345")
	e.svc = New(func(p speechsetup.Place) Worker { e.made = append(e.made, p); return e.worker }, e.programs)
	return e
}

func (e *env) speak(r Request) (*Result, error) {
	e.t.Helper()
	if r.Workflow == "" {
		r.Workflow = "talk"
	}
	return e.svc.Speak(context.Background(), r, Options{})
}

func ptr(s string) *string { return &s }

// herrOf fails the test unless err is a horus error with the code.
func herrOf(t *testing.T, err error, code string) *herr.Error {
	t.Helper()
	var he *herr.Error
	if !errors.As(err, &he) || he.Code != code {
		t.Fatalf("error %v, want the code %s", err, code)
	}
	return he
}

func TestTheTextGoesToTheWorkerAsWritten(t *testing.T) {
	e := newEnv(t)
	text := "  Xin chào  [laugh] các bạn.\n"
	res, err := e.speak(Request{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.worker.texts) != 1 || e.worker.texts[0].Text != text {
		t.Fatalf("the worker got %q, want the text as written %q", e.worker.texts, text)
	}
	got := e.worker.texts[0]
	if got.Voice != "alpha" || got.Format != "wav" || filepath.Dir(got.Out) != filepath.Join(e.root, "out", "talk") || !strings.HasSuffix(got.Out, ".wav") {
		t.Fatalf("the worker got %+v: want the default voice, wav, and a new file in the output folder", got)
	}
	if res.Path != got.Out || res.Transcript != text || res.Bytes != 4 || res.Format != "wav" || res.Voice != "alpha" ||
		res.Workflow != "talk" || !res.StartedWorker || res.AudioSeconds != 0.5 {
		t.Fatalf("result %+v", res)
	}
	if len(e.made) != 1 || e.made[0] != e.place {
		t.Fatalf("the service made the workers %+v, want one at %+v", e.made, e.place)
	}
}

func TestTheSecondCallKeepsTheWorker(t *testing.T) {
	e := newEnv(t)
	out := filepath.Join(e.root, "mine", "two.mp3")
	first, err := e.speak(Request{Text: "một"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.speak(Request{Text: "hai", Voice: ptr("beta"), OutPath: &out})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.made) != 1 || !first.StartedWorker || second.StartedWorker {
		t.Fatalf("workers made %d, started %v then %v: want 1, true then false", len(e.made), first.StartedWorker, second.StartedWorker)
	}
	if second.Path != out || second.Format != "mp3" || second.Voice != "beta" || e.worker.texts[1].Format != "mp3" {
		t.Fatalf("second call %+v, worker got %+v", second, e.worker.texts[1])
	}
}

// TestFailures has one case for each failure of the spec table "Failures".
// No case before the worker step may reach the worker.
func TestFailures(t *testing.T) {
	long := strings.Repeat("a", 41)
	cases := []struct {
		name    string
		req     Request
		prepare func(e *env, r *Request)
		code    string
		check   func(t *testing.T, e *herr.Error)
		worker  bool // the call reaches the worker
	}{
		{name: "empty text", req: Request{Text: "   "}, code: "INVALID_ARGUMENT", check: field("text", "min_chars", "max_chars", "rules")},
		{name: "too short text", req: Request{Text: "a"}, code: "INVALID_ARGUMENT", check: field("text", "got")},
		{name: "too long text", req: Request{Text: long}, code: "INVALID_ARGUMENT", check: field("text", "max_chars")},
		{name: "a tag that the model does not perform", req: Request{Text: "chào [cough] bạn [laugh]"}, code: "INVALID_ARGUMENT",
			check: func(t *testing.T, e *herr.Error) {
				field("text", "allowed_tags")(t, e)
				if !reflect.DeepEqual(e.Details["found"], []string{"[cough]"}) {
					t.Errorf("found %v, want [cough] only", e.Details["found"])
				}
			}},
		{name: "unknown voice", req: Request{Text: "xin chào", Voice: ptr("gamma")}, code: "INVALID_ARGUMENT",
			check: func(t *testing.T, e *herr.Error) {
				field("voice", "known")(t, e)
				if !reflect.DeepEqual(e.Details["known"], []string{"alpha", "beta"}) {
					t.Errorf("known %v", e.Details["known"])
				}
			}},
		{name: "relative out_path", req: Request{Text: "xin chào", OutPath: ptr(filepath.Join("a", "b.wav"))}, code: "INVALID_ARGUMENT", check: field("out_path", "got")},
		{name: "out_path with another end", req: Request{Text: "xin chào", OutPath: ptr(filepath.Join(os.TempDir(), "b.ogg"))}, code: "INVALID_ARGUMENT", check: field("out_path", "formats")},
		{name: "an image workflow", req: Request{Workflow: "it-illustration", Text: "xin chào"}, code: "INVALID_ARGUMENT",
			check: func(t *testing.T, e *herr.Error) {
				field("workflow", "kind")(t, e)
				if !strings.Contains(e.NextStep, "generate_image") {
					t.Errorf("next_step %q does not name generate_image", e.NextStep)
				}
			}},
		{name: "out_path exists", req: Request{Text: "xin chào"}, code: "OUTPUT_EXISTS",
			prepare: func(e *env, r *Request) {
				p := filepath.Join(e.root, "exists.wav")
				os.WriteFile(p, []byte("old"), 0o644)
				r.OutPath = &p
			}},
		{name: "no speech venv", req: Request{Text: "xin chào"}, code: "WORKFLOW_NOT_INSTALLED",
			prepare: func(e *env, r *Request) { os.Remove(e.place.Python) }, check: missing("speech venv")},
		{name: "no ffmpeg", req: Request{Text: "xin chào"}, code: "WORKFLOW_NOT_INSTALLED",
			prepare: func(e *env, r *Request) { e.programs.missing["ffmpeg"] = true }, check: missing("ffmpeg")},
		{name: "a model file is missing", req: Request{Text: "xin chào"}, code: "WORKFLOW_NOT_INSTALLED",
			prepare: func(e *env, r *Request) { os.Remove(filepath.Join(e.place.Models, "tts", "model.bin")) }, check: missing("model.bin")},
		{name: "the worker stops", req: Request{Text: "xin chào"}, code: "GENERATION_FAILED", worker: true,
			prepare: func(e *env, r *Request) {
				e.worker.answer = func(context.Context, Text) (Speech, error) {
					return Speech{}, &WorkerError{Stopped: true, Message: "exit status 3", Output: "line 1\nfake worker: stops in the request\n"}
				}
			}, check: worker("exit status 3", true)},
		{name: "the worker reports an error", req: Request{Text: "xin chào"}, code: "GENERATION_FAILED", worker: true,
			prepare: func(e *env, r *Request) {
				e.worker.answer = func(context.Context, Text) (Speech, error) {
					return Speech{}, &WorkerError{Message: "fake worker: the request failed"}
				}
			}, check: worker("fake worker: the request failed", false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			req := c.req
			if c.prepare != nil {
				c.prepare(e, &req)
			}
			_, err := e.speak(req)
			he := herrOf(t, err, c.code)
			info := he.Info()
			if info.NextStep == "" || info.ActionBy == "" {
				t.Errorf("next_step %q, action_by %q: want both", info.NextStep, info.ActionBy)
			}
			if c.check != nil {
				c.check(t, he)
			}
			if reached := len(e.worker.texts) > 0; reached != c.worker {
				t.Errorf("the call reached the worker: %v, want %v", reached, c.worker)
			}
		})
	}
}

// field checks details.field and that the details have the keys.
func field(name string, keys ...string) func(t *testing.T, e *herr.Error) {
	return func(t *testing.T, e *herr.Error) {
		t.Helper()
		if e.Details["field"] != name {
			t.Errorf("details.field %v, want %s", e.Details["field"], name)
		}
		for _, k := range keys {
			if _, ok := e.Details[k]; !ok {
				t.Errorf("details has no %s: %v", k, e.Details)
			}
		}
		if e.Info().ActionBy != "caller" {
			t.Errorf("action_by %s, want caller", e.Info().ActionBy)
		}
	}
}

// missing checks that details.missing names the part, and next_step names install_workflow.
func missing(part string) func(t *testing.T, e *herr.Error) {
	return func(t *testing.T, e *herr.Error) {
		t.Helper()
		if !reflect.DeepEqual(e.Details["missing"], []string{part}) {
			t.Errorf("details.missing %v, want [%s]", e.Details["missing"], part)
		}
		if !strings.Contains(e.NextStep, "install_workflow") || !strings.Contains(e.NextStep, "generate_speech") {
			t.Errorf("next_step %q", e.NextStep)
		}
	}
}

// worker checks the details of GENERATION_FAILED.
func worker(message string, stopped bool) func(t *testing.T, e *herr.Error) {
	return func(t *testing.T, e *herr.Error) {
		t.Helper()
		info := e.Info()
		if info.ActionBy != "retry" || !info.Retryable || !strings.Contains(info.NextStep, "generate_speech") {
			t.Errorf("action_by %s, retryable %v, next_step %q", info.ActionBy, info.Retryable, info.NextStep)
		}
		if e.Details["worker_message"] != message || e.Details["worker_stopped"] != stopped {
			t.Errorf("details %v", e.Details)
		}
	}
}

func TestOverwriteReplacesTheFile(t *testing.T) {
	e := newEnv(t)
	p := filepath.Join(e.root, "exists.wav")
	if err := os.WriteFile(p, []byte("old file"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := e.speak(Request{Text: "xin chào", OutPath: &p, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != "RIFF" || res.Bytes != 4 {
		t.Fatalf("file %q, bytes %d", data, res.Bytes)
	}
}

func TestAnAnswerWithoutAFileFails(t *testing.T) {
	e := newEnv(t)
	e.worker.answer = func(_ context.Context, t Text) (Speech, error) {
		return Speech{Path: t.Out, Bytes: 9, Transcript: t.Text}, nil
	}
	_, err := e.speak(Request{Text: "xin chào"})
	herrOf(t, err, "GENERATION_FAILED")
}

func TestAPythonThatDoesNotExistIsNotInstalled(t *testing.T) {
	e := newEnv(t)
	e.worker.answer = func(context.Context, Text) (Speech, error) {
		return Speech{}, fmt.Errorf("start the speech worker: %w", fs.ErrNotExist)
	}
	_, err := e.speak(Request{Text: "xin chào"})
	if he := herrOf(t, err, "WORKFLOW_NOT_INSTALLED"); !strings.Contains(he.NextStep, "install_workflow") {
		t.Fatalf("next_step %q", he.NextStep)
	}
}

func TestTheTimeLimitStopsTheCall(t *testing.T) {
	e := newEnv(t)
	e.worker.answer = func(ctx context.Context, _ Text) (Speech, error) {
		<-ctx.Done() // the adapter ends the worker at the end of the context
		return Speech{}, ctx.Err()
	}
	start := time.Now()
	_, err := e.svc.Speak(context.Background(), Request{Workflow: "talk", Text: "xin chào"}, Options{Timeout: 200 * time.Millisecond})
	he := herrOf(t, err, "GENERATION_TIMEOUT")
	if !strings.Contains(he.NextStep, "generate_speech") || time.Since(start) > 5*time.Second {
		t.Fatalf("next_step %q after %s", he.NextStep, time.Since(start))
	}
}

func TestTheSettingOfTheTimeLimit(t *testing.T) {
	e := newEnv(t)
	t.Setenv("HORUS_GENERATE_TIMEOUT", "1")
	e.worker.answer = func(ctx context.Context, _ Text) (Speech, error) { <-ctx.Done(); return Speech{}, ctx.Err() }
	_, err := e.speak(Request{Text: "xin chào"})
	herrOf(t, err, "GENERATION_TIMEOUT")
}

func TestProgressWhileTheWorkerRuns(t *testing.T) {
	e := newEnv(t)
	old := ReportInterval
	ReportInterval = 50 * time.Millisecond
	t.Cleanup(func() { ReportInterval = old })
	e.worker.answer = func(_ context.Context, t Text) (Speech, error) {
		time.Sleep(300 * time.Millisecond)
		os.MkdirAll(filepath.Dir(t.Out), 0o755)
		return Speech{Transcript: t.Text}, os.WriteFile(t.Out, []byte("RIFF"), 0o644)
	}
	var mu sync.Mutex
	var messages []string
	report := func(_ context.Context, _, _ float64, m string) {
		mu.Lock()
		messages = append(messages, m)
		mu.Unlock()
	}
	if _, err := e.svc.Speak(context.Background(), Request{Workflow: "talk", Text: "xin chào"}, Options{Report: report}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(messages) < 3 || !strings.HasPrefix(messages[0], "starting the speech worker") || !strings.HasPrefix(messages[1], "making the speech file, ") {
		t.Fatalf("progress messages %q: want the start, then a message each interval", messages)
	}
}

func TestCloseEndsTheWorkers(t *testing.T) {
	e := newEnv(t)
	if _, err := e.speak(Request{Text: "xin chào"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Close(); err != nil || !e.worker.closed {
		t.Fatalf("Close: %v, worker closed %v", err, e.worker.closed)
	}
	_, err := e.speak(Request{Text: "xin chào"})
	herrOf(t, err, "GENERATION_FAILED")
}

// TestStopWorkersEndsTheWorkerAndTheNextCallMakesANewOne: StopWorkers
// closes the worker (uninstall_workflow calls it before it deletes model
// files), and the service stays open: the next call makes a new worker.
func TestStopWorkersEndsTheWorkerAndTheNextCallMakesANewOne(t *testing.T) {
	e := newEnv(t)
	if _, err := e.speak(Request{Text: "xin chào"}); err != nil {
		t.Fatal(err)
	}
	e.svc.StopWorkers()
	if !e.worker.closed {
		t.Fatal("StopWorkers did not close the worker")
	}
	res, err := e.speak(Request{Text: "xin chào"})
	if err != nil {
		t.Fatalf("a call after StopWorkers: %v", err)
	}
	if len(e.made) != 2 || !res.StartedWorker {
		t.Fatalf("workers made: %d, started_worker %v; want a new worker", len(e.made), res.StartedWorker)
	}
}

func TestAnotherPlaceGetsAnotherWorker(t *testing.T) {
	e := newEnv(t)
	if _, err := e.speak(Request{Text: "xin chào"}); err != nil {
		t.Fatal(err)
	}
	other := speechsetup.At(filepath.Join(e.root, "speech2"))
	for _, p := range []string{other.Python, filepath.Join(other.Models, "tts", "model.bin")} {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("12345"), 0o644)
	}
	t.Setenv("HORUS_SPEECH_DIR", other.Dir)
	if _, err := e.speak(Request{Text: "xin chào"}); err != nil {
		t.Fatal(err)
	}
	if len(e.made) != 2 || e.made[1] != other {
		t.Fatalf("workers %+v", e.made)
	}
}
