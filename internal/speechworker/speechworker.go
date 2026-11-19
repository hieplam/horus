// Package speechworker is the adapter for the speech worker: the Python
// process that holds the speech model (VieNeu) and the speech recognition
// model in memory (ADR 0022).
//
// The program holds the Python file of the worker (worker.py, go:embed).
// Before each start, the adapter writes that file to Config.Script and
// starts it with the Python of the speech venv:
//
//	<Config.Python> -I <Config.Script> --models <Config.Models>
//
// The adapter and the worker talk through the standard input and output of
// the worker: one JSON line for each request and one for each answer. The
// docstring of worker.py gives the lines. The first line of the worker
// says that the models are loaded (or why not).
//
// A Worker starts the process at the first request, keeps it for the next
// requests, and sends one request at a time: a second call waits. The
// process ends when the context of New ends, when Close is called, or when
// the context of a request ends before its answer. A worker that stops is
// an error of the request (ErrStopped). The next request starts a new
// process.
//
// The fake worker of the tests is the program in the folder fakeworker.
package speechworker

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed worker.py
var script []byte

// Script gives the text of the Python file of the speech worker.
func Script() []byte { return bytes.Clone(script) }

// Config says how to start the speech worker.
type Config struct {
	Python string    // the Python program of the speech venv
	Script string    // the path of the worker file; the adapter writes the file there before each start
	Models string    // the folder with vieneu/, codec/ and whisper-small/
	Env    []string  // more environment of the worker, KEY=value, after the environment of this process
	Log    io.Writer // gets the error output of the worker; nil: the adapter keeps only its last lines
}

// The failures of a request. An *Error wraps ErrStopped or ErrReported.
var (
	// ErrStopped: the worker process ended before it answered, or it did
	// not load the models.
	ErrStopped = errors.New("the speech worker stopped")
	// ErrReported: the worker answered the request with an error. The
	// worker still runs.
	ErrReported = errors.New("the speech worker reported an error")
	// ErrClosed: the context of New ended, or Close was called. The adapter
	// starts no new worker.
	ErrClosed = errors.New("the speech worker is closed")
)

// Error is a failure of the worker, with what the worker said.
type Error struct {
	Kind    error  // ErrStopped or ErrReported
	Message string // the error of the answer, or the exit status of the process
	Stderr  string // the last lines of the error output of the worker
}

func (e *Error) Error() string {
	s := e.Kind.Error() + ": " + e.Message
	if e.Stderr != "" {
		s += ". The last lines of its error output: " + e.Stderr
	}
	return s
}

func (e *Error) Unwrap() error { return e.Kind }

// Voice is one preset voice of the speech model.
type Voice struct {
	ID    string `json:"id"`    // the name that a request gives
	Label string `json:"label"` // the name that the model package shows
}

// Voices is the answer of the request voices.
type Voices struct {
	Voices  []Voice `json:"voices"`
	Default string  `json:"default"`
	Started bool    `json:"-"` // this call started the worker
}

// Text is one request to speak.
type Text struct {
	Text   string // sent to the model as written
	Voice  string // a Voice.ID; empty: the default voice
	Out    string // the absolute path of the audio file
	Format string // "wav" or "mp3"
}

// Speech is the answer of the request speak.
type Speech struct {
	Path         string  `json:"path"`
	Bytes        int64   `json:"bytes"`
	Format       string  `json:"format"`
	AudioSeconds float64 `json:"audio_seconds"`
	LUFS         float64 `json:"lufs"`
	Transcript   string  `json:"transcript"`
	SpeakSeconds float64 `json:"speak_seconds"`
	ReadSeconds  float64 `json:"read_seconds"`
	Started      bool    `json:"-"` // this call started the worker
}

// stopWait is how long the adapter waits for a worker to end after it
// closes its standard input. Then it ends the process.
const stopWait = 5 * time.Second

// tailLines is the count of lines of the error output that an Error keeps.
const tailLines = 20

// Worker is the adapter of one speech worker.
type Worker struct {
	cfg  Config
	life context.Context
	slot chan struct{} // holds one token while a request runs

	mu     sync.Mutex // guards proc and closed
	proc   *process
	closed bool
	nextID int
}

// New makes the adapter. It starts no process: the first request does. The
// end of life ends the worker.
func New(life context.Context, cfg Config) *Worker {
	return &Worker{cfg: cfg, life: life, slot: make(chan struct{}, 1)}
}

// Voices gives the preset voices of the speech model.
func (w *Worker) Voices(ctx context.Context) (Voices, error) {
	var v Voices
	started, err := w.call(ctx, map[string]any{"op": "voices"}, &v)
	v.Started = started
	return v, err
}

// Speak makes one audio file.
func (w *Worker) Speak(ctx context.Context, t Text) (Speech, error) {
	var s Speech
	started, err := w.call(ctx, map[string]any{"op": "speak", "text": t.Text, "voice": t.Voice, "out": t.Out, "format": t.Format}, &s)
	s.Started = started
	return s, err
}

// PID gives the process id of the running worker, or 0.
func (w *Worker) PID() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.proc == nil {
		return 0
	}
	return w.proc.cmd.Process.Pid
}

// Close ends the worker and waits until it ended. Then each request answers
// ErrClosed. A request that runs gets ErrStopped.
func (w *Worker) Close() error {
	w.mu.Lock()
	w.closed = true
	p := w.proc
	w.proc = nil
	w.mu.Unlock()
	if p != nil {
		p.stop()
	}
	return nil
}

// call sends one request and decodes the answer into out. It starts the
// worker when none runs, and tells if it did.
func (w *Worker) call(ctx context.Context, req map[string]any, out any) (started bool, err error) {
	select {
	case w.slot <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-w.slot }()

	p, started, err := w.running(ctx)
	if err != nil {
		return started, err
	}
	w.nextID++
	id := w.nextID
	req["id"] = id
	line, err := json.Marshal(req)
	if err != nil {
		return started, err
	}
	if _, err := p.stdin.Write(append(line, '\n')); err != nil {
		return started, w.forget(p, p.stopped())
	}
	for {
		select {
		case l, ok := <-p.lines:
			if !ok {
				return started, w.forget(p, p.stopped())
			}
			var head struct {
				ID    *int   `json:"id"`
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			if json.Unmarshal(l, &head) != nil || head.ID == nil || *head.ID != id {
				p.note("not the answer of request %d: %s", id, l)
				continue
			}
			if !head.OK {
				return started, &Error{Kind: ErrReported, Message: head.Error, Stderr: p.tail()}
			}
			if err := json.Unmarshal(l, out); err != nil {
				return started, &Error{Kind: ErrReported, Message: "an answer that horus cannot read: " + err.Error(), Stderr: p.tail()}
			}
			return started, nil
		case <-ctx.Done():
			// The worker cannot stop a request half way, so it ends: the next
			// request starts a new worker.
			w.forget(p, nil)
			p.kill()
			return started, ctx.Err()
		}
	}
}

// running gives the running worker, and starts one when none runs.
func (w *Worker) running(ctx context.Context) (*process, bool, error) {
	w.mu.Lock()
	closed, p := w.closed || w.life.Err() != nil, w.proc
	w.mu.Unlock()
	if closed {
		return nil, false, ErrClosed
	}
	if p != nil && !p.ended() {
		return p, false, nil
	}
	p, err := w.start(ctx)
	if err != nil {
		return nil, true, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		go p.stop()
		return nil, true, ErrClosed
	}
	w.proc = p
	return p, true, nil
}

// forget drops p as the running worker and returns err.
func (w *Worker) forget(p *process, err error) error {
	w.mu.Lock()
	if w.proc == p {
		w.proc = nil
	}
	w.mu.Unlock()
	return err
}

// start writes the worker file, starts the worker, and waits for its first
// line: the models are loaded.
func (w *Worker) start(ctx context.Context) (*process, error) {
	if err := writeScript(w.cfg.Script); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(w.life, w.cfg.Python, "-I", w.cfg.Script, "--models", w.cfg.Models)
	cmd.Env = append(os.Environ(), w.cfg.Env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// The worker ends at the end of its standard input. Without an end in
	// stopWait, the process ends.
	cmd.Cancel = stdin.Close
	cmd.WaitDelay = stopWait
	r, wr, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = wr
	p := &process{cmd: cmd, stdin: stdin, lines: make(chan []byte, 16), done: make(chan struct{}), errTail: &tail{}}
	cmd.Stderr = p.errTail
	if w.cfg.Log != nil {
		cmd.Stderr = io.MultiWriter(p.errTail, w.cfg.Log)
	}
	err = cmd.Start()
	wr.Close()
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("start the speech worker %s: %w", w.cfg.Python, err)
	}
	go p.read(r)
	go func() { p.err = cmd.Wait(); close(p.done) }()

	for {
		select {
		case l, ok := <-p.lines:
			if !ok {
				return nil, p.stopped()
			}
			var ready struct {
				Ready *bool  `json:"ready"`
				Error string `json:"error"`
			}
			if json.Unmarshal(l, &ready) != nil || ready.Ready == nil {
				p.note("not the first line of the worker: %s", l)
				continue
			}
			if !*ready.Ready {
				p.kill()
				return nil, &Error{Kind: ErrStopped, Message: "the worker did not load the models: " + ready.Error, Stderr: p.tail()}
			}
			return p, nil
		case <-ctx.Done():
			p.kill()
			return nil, ctx.Err()
		}
	}
}

// writeScript writes the worker file to path, when the file there is not
// the same.
func writeScript(path string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, script) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write the speech worker file: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, script, 0o644); err != nil {
		return fmt.Errorf("write the speech worker file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("write the speech worker file: %w", err)
	}
	return nil
}

// process is one running speech worker.
type process struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan []byte   // the lines of the standard output; closed at its end
	done    chan struct{} // closed when the process ended
	err     error         // the result of Wait, after done
	errTail *tail
}

// read sends each line of the standard output to lines.
func (p *process) read(r io.ReadCloser) {
	defer close(p.lines)
	defer r.Close()
	br := bufio.NewReader(r)
	for {
		l, err := br.ReadBytes('\n')
		if l = bytes.TrimSpace(l); len(l) > 0 {
			p.lines <- l
		}
		if err != nil {
			return
		}
	}
}

func (p *process) ended() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// stopped waits until the process ended and gives the error of a worker
// that stopped.
func (p *process) stopped() error {
	select {
	case <-p.done:
	case <-time.After(stopWait + time.Second):
		p.kill()
		<-p.done
	}
	status := "it ended"
	if p.err != nil {
		status = p.err.Error()
	} else if p.cmd.ProcessState != nil {
		status = p.cmd.ProcessState.String()
	}
	return &Error{Kind: ErrStopped, Message: status, Stderr: p.tail()}
}

// stop closes the standard input, waits up to stopWait for the end, then
// ends the process.
func (p *process) stop() {
	p.stdin.Close()
	select {
	case <-p.done:
	case <-time.After(stopWait):
		p.kill()
	}
}

// kill ends the process and waits until it ended.
func (p *process) kill() {
	p.cmd.Process.Kill()
	<-p.done
}

// note adds a line of the adapter to the kept error output.
func (p *process) note(format string, args ...any) {
	fmt.Fprintf(p.errTail, "horus: "+format+"\n", args...)
}

func (p *process) tail() string { return p.errTail.String() }

// tail keeps the last tailLines lines that it gets.
type tail struct {
	mu    sync.Mutex
	lines []string
	part  string
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.part + string(b)
	parts := strings.Split(s, "\n")
	t.part = parts[len(parts)-1]
	for _, l := range parts[:len(parts)-1] {
		if l = strings.TrimRight(l, "\r"); strings.TrimSpace(l) != "" {
			t.lines = append(t.lines, l)
		}
	}
	if n := len(t.lines) - tailLines; n > 0 {
		t.lines = t.lines[n:]
	}
	if len(t.part) > 4096 {
		t.part = t.part[len(t.part)-4096:]
	}
	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := t.lines
	if strings.TrimSpace(t.part) != "" {
		lines = append(lines[:len(lines):len(lines)], t.part)
	}
	return strings.Join(lines, "\n")
}
