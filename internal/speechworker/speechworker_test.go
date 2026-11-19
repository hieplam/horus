package speechworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// fakePython is the fake worker (the program in the folder fakeworker),
// built one time by TestMain.
var fakePython string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "horus-fakeworker-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fakePython = filepath.Join(dir, platform.ExeName("python"))
	build := exec.Command("go", "build", "-o", fakePython, "./fakeworker")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build the fake worker: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// fake is one test with the fake worker.
type fake struct {
	t      *testing.T
	dir    string
	record string
	w      *Worker
	cancel context.CancelFunc
}

// newFake makes an adapter for the fake worker in the mode mode. The end of
// the test ends the worker.
func newFake(t *testing.T, mode string) *fake {
	t.Helper()
	dir := t.TempDir()
	f := &fake{t: t, dir: dir, record: filepath.Join(dir, "worker.record")}
	t.Setenv("HORUS_FAKE_SPEECH", mode)
	t.Setenv("HORUS_FAKE_SPEECH_RECORD", f.record)
	life, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.w = New(life, Config{Python: fakePython, Script: filepath.Join(dir, "user", "speech", "worker.py"), Models: filepath.Join(dir, "models")})
	t.Cleanup(func() { f.w.Close(); cancel() })
	return f
}

// mode sets the mode of the next start of the fake worker.
func (f *fake) mode(m string) { f.t.Setenv("HORUS_FAKE_SPEECH", m) }

// text is a request to speak into the file name in the test folder.
func (f *fake) text(name string) Text {
	return Text{Text: "Xin chào các bạn.", Voice: "Hải Đăng", Out: filepath.Join(f.dir, "out", name), Format: "wav"}
}

// lines gives the lines of the record of the fake worker.
func (f *fake) lines() []string {
	b, _ := os.ReadFile(f.record)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// starts gives the count of starts of the fake worker.
func (f *fake) starts() int {
	n := 0
	for _, l := range f.lines() {
		if strings.HasPrefix(l, "start") {
			n++
		}
	}
	return n
}

// gone waits up to 10 s until the process pid ended.
func gone(pid int) bool {
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if !platform.Alive(pid) {
			return true
		}
	}
	return false
}

func TestOneRequestAndItsAnswer(t *testing.T) {
	f := newFake(t, "")
	req := f.text("one.wav")
	got, err := f.w.Speak(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(req.Out)
	if err != nil {
		t.Fatalf("the audio file: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) || got.Bytes != int64(len(data)) {
		t.Errorf("the file has %d bytes and starts with %q; the answer says %d bytes", len(data), data[:4], got.Bytes)
	}
	want := Speech{Path: req.Out, Bytes: int64(len(data)), Format: "wav", AudioSeconds: 0.5, LUFS: -18, Transcript: req.Text, Started: true}
	if got != want {
		t.Errorf("Speak = %+v, want %+v", got, want)
	}
	if script, err := os.ReadFile(f.w.cfg.Script); err != nil || !bytes.Equal(script, Script()) {
		t.Errorf("the worker file %s is not the embedded worker.py (err %v)", f.w.cfg.Script, err)
	}
	wantStart := "start -I " + f.w.cfg.Script + " --models " + f.w.cfg.Models
	if b, _ := os.ReadFile(f.record); !strings.HasPrefix(string(b), wantStart+"\n") {
		t.Errorf("the fake worker got %q, want the arguments %q", b, wantStart)
	}
}

func TestTwoRequestsStartTheWorkerOneTime(t *testing.T) {
	f := newFake(t, "")
	first, err := f.w.Speak(context.Background(), f.text("first.wav"))
	if err != nil {
		t.Fatal(err)
	}
	pid := f.w.PID()
	second, err := f.w.Speak(context.Background(), f.text("second.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Started || second.Started {
		t.Errorf("Started: first %v, second %v; want true, false", first.Started, second.Started)
	}
	if f.starts() != 1 || f.w.PID() != pid {
		t.Errorf("starts %d, pid %d then %d; want 1 start and one pid. Record: %v", f.starts(), pid, f.w.PID(), f.lines())
	}
}

func TestAWorkerThatStopsInARequest(t *testing.T) {
	f := newFake(t, "die")
	_, err := f.w.Speak(context.Background(), f.text("die.wav"))
	var we *Error
	if !errors.Is(err, ErrStopped) || !errors.As(err, &we) {
		t.Fatalf("Speak = %v, want ErrStopped", err)
	}
	if !strings.Contains(we.Message, "exit status 3") || !strings.Contains(we.Stderr, "fake worker: stops in the request") {
		t.Errorf("the error has no exit status 3 or no error output of the worker: %v", err)
	}
	// The next request starts a new worker.
	f.mode("")
	got, err := f.w.Speak(context.Background(), f.text("after.wav"))
	if err != nil || !got.Started || f.starts() != 2 {
		t.Errorf("after the stop: Speak = %+v, %v, starts %d; want a new worker", got, err, f.starts())
	}
}

func TestTheEndOfTheServerContextStopsTheWorker(t *testing.T) {
	f := newFake(t, "")
	if _, err := f.w.Voices(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := f.w.PID()
	if pid <= 0 || !platform.Alive(pid) {
		t.Fatalf("no running worker: pid %d", pid)
	}
	f.cancel()
	if !gone(pid) {
		t.Fatalf("the worker %d still runs 10 s after the end of the server context", pid)
	}
	if _, err := f.w.Speak(context.Background(), f.text("late.wav")); !errors.Is(err, ErrClosed) {
		t.Errorf("Speak after the end = %v, want ErrClosed", err)
	}
	if f.starts() != 1 {
		t.Errorf("starts %d, want 1: the adapter starts no worker after the end", f.starts())
	}
}

func TestCloseStopsTheWorker(t *testing.T) {
	f := newFake(t, "")
	if _, err := f.w.Voices(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := f.w.PID()
	f.w.Close()
	if platform.Alive(pid) {
		t.Errorf("the worker %d runs after Close", pid)
	}
	if _, err := f.w.Voices(context.Background()); !errors.Is(err, ErrClosed) {
		t.Errorf("Voices after Close = %v, want ErrClosed", err)
	}
}

func TestTheEndOfARequestContextStopsTheWorker(t *testing.T) {
	f := newFake(t, "slow")
	if _, err := f.w.Voices(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := f.w.PID()
	// The fake worker waits 2 minutes in the request, so the context ends first.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := f.w.Speak(ctx, f.text("slow.wav"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Speak = %v, want the end of the context", err)
	}
	if f.w.PID() != 0 || platform.Alive(pid) {
		t.Errorf("the worker %d still runs after the end of the request (the adapter holds pid %d)", pid, f.w.PID())
	}
	f.mode("")
	if got, err := f.w.Speak(context.Background(), f.text("next.wav")); err != nil || !got.Started {
		t.Errorf("the next request: %+v, %v; want a new worker", got, err)
	}
}

func TestAReportedErrorKeepsTheWorker(t *testing.T) {
	f := newFake(t, "error")
	_, err := f.w.Speak(context.Background(), f.text("error.wav"))
	var we *Error
	if !errors.Is(err, ErrReported) || !errors.As(err, &we) || we.Message != "fake worker: the request failed" {
		t.Fatalf("Speak = %v, want ErrReported with the text of the worker", err)
	}
	pid := f.w.PID()
	v, err := f.w.Voices(context.Background())
	if err != nil || v.Started || f.w.PID() != pid {
		t.Errorf("Voices after the error = %+v, %v (pid %d then %d); want the same worker", v, err, pid, f.w.PID())
	}
}

func TestVoices(t *testing.T) {
	f := newFake(t, "")
	v, err := f.w.Voices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Default != "Hải Đăng" || len(v.Voices) != 2 || v.Voices[1] != (Voice{ID: "Mai Anh", Label: "Mai Anh"}) || !v.Started {
		t.Errorf("Voices = %+v", v)
	}
}

func TestAWorkerThatCannotLoadTheModels(t *testing.T) {
	f := newFake(t, "load")
	_, err := f.w.Speak(context.Background(), f.text("load.wav"))
	var we *Error
	if !errors.Is(err, ErrStopped) || !errors.As(err, &we) || !strings.Contains(we.Message, "no model file") {
		t.Fatalf("Speak = %v, want ErrStopped with the reason of the worker", err)
	}
	if f.w.PID() != 0 {
		t.Errorf("the adapter keeps the worker %d", f.w.PID())
	}
}

func TestNoPython(t *testing.T) {
	f := newFake(t, "")
	f.w.cfg.Python = filepath.Join(f.dir, "no-venv", platform.ExeName("python"))
	if _, err := f.w.Voices(context.Background()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Voices = %v, want an error of a missing file", err)
	}
}

func TestLinesThatAreNotTheAnswerAreSkipped(t *testing.T) {
	f := newFake(t, "noise")
	got, err := f.w.Speak(context.Background(), f.text("noise.wav"))
	if err != nil || got.Transcript == "" {
		t.Fatalf("Speak = %+v, %v", got, err)
	}
}

func TestASecondCallWaits(t *testing.T) {
	f := newFake(t, "")
	var wg sync.WaitGroup
	got := make([]Speech, 4)
	errs := make([]error, 4)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = f.w.Speak(context.Background(), f.text(fmt.Sprintf("call-%d.wav", i)))
		}()
	}
	wg.Wait()
	started := 0
	for i := range got {
		if errs[i] != nil || got[i].Path != f.text(fmt.Sprintf("call-%d.wav", i)).Out {
			t.Errorf("call %d: %+v, %v; want its own file", i, got[i], errs[i])
		}
		if got[i].Started {
			started++
		}
	}
	if started != 1 || f.starts() != 1 {
		t.Errorf("%d calls say that they started the worker, %d starts; want 1 and 1", started, f.starts())
	}
}

func TestTheWorkerFileIsWrittenAgainWhenItChanged(t *testing.T) {
	f := newFake(t, "")
	path := f.w.cfg.Script
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# an old worker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.w.Voices(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, Script()) {
		t.Errorf("the worker file is not the embedded worker.py after a start")
	}
}
