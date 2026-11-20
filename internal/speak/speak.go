// Package speak makes one speech file with one speech workflow (the tool
// generate_speech).
//
// The steps:
//  1. Find the workflow. A workflow of the kind image is INVALID_ARGUMENT, and
//     its next_step names generate_image.
//  2. Check the arguments against the manifest (check.go): the length of the
//     text, the [tag] markers in it, the voice, out_path and overwrite.
//  3. Check that the speech setup is there (internal/speechsetup): the Python
//     of the speech venv, ffmpeg, ffprobe and each model file. A missing part
//     is WORKFLOW_NOT_INSTALLED.
//  4. Send the text, as written, to the speech worker. The service makes the
//     worker at the first call and keeps it until Close. The worker makes one
//     file at a time, so a second call waits.
//  5. Report progress every ReportInterval while the worker runs.
//     HORUS_GENERATE_TIMEOUT limits the call.
//  6. Return the file, the length of the audio and the transcript. horus does
//     not compare the transcript with the text: the caller does (ADR 0024).
//
// The service declares the interface of the speech worker (Worker) and of the
// programs of this machine (Programs). cmd/horus gives it the adapter
// internal/speechworker and platform.Programs. A test gives it fakes.
package speak

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speechsetup"
)

// Worker is what the service needs from one speech worker. The adapter owns
// the process and its protocol. It starts the process at the first request
// and keeps it, it sends one request at a time, and it ends the process when
// the context of a request ends before the answer.
type Worker interface {
	// Speak makes one audio file. An error of the worker is a *WorkerError.
	// A Python program that does not exist gives an error that
	// errors.Is(err, fs.ErrNotExist) finds.
	Speak(ctx context.Context, t Text) (Speech, error)
	// Running tells if the worker process runs now.
	Running() bool
	// Close ends the worker and waits until it ended.
	Close() error
}

// NewWorker makes the worker of the speech setup at p. It starts no process.
type NewWorker func(p speechsetup.Place) Worker

// Programs is what the service needs to find a program on the PATH.
// cmd/horus gives it platform.Programs.
type Programs interface {
	LookPath(name string) (string, error)
}

// Text is one request to the speech worker.
type Text struct {
	Text   string // the text of the caller, as written
	Voice  string // a voice of the workflow
	Out    string // the absolute path of the audio file
	Format string // "wav" or "mp3"
}

// Speech is the answer of the speech worker.
type Speech struct {
	Path         string
	Bytes        int64
	Format       string
	AudioSeconds float64
	Transcript   string
	Started      bool // this request started the worker
}

// WorkerError is a failure of the speech worker. cmd/horus makes it from the
// error of the adapter.
type WorkerError struct {
	// Stopped is true when the worker process ended before it answered, did
	// not load the models, or was closed. It is false when the worker
	// answered the request with an error and still runs.
	Stopped bool
	Message string // what the worker said, or the exit status of its process
	Output  string // the last lines of the error output of the worker
}

func (e *WorkerError) Error() string {
	if e.Stopped {
		return "the speech worker stopped: " + e.Message
	}
	return "the speech worker reported an error: " + e.Message
}

// Report sends one progress report. total 0 means unknown.
type Report func(ctx context.Context, progress, total float64, message string)

// ReportInterval is the time between two progress reports while the worker
// runs. Tests make it short.
var ReportInterval = 10 * time.Second

// Request holds the arguments of generate_speech. A nil Voice or OutPath
// takes the default.
type Request struct {
	Workflow  string
	Text      string
	Voice     *string
	OutPath   *string
	Overwrite bool
}

// Options are the parts of a call that the server or a test sets.
type Options struct {
	Report Report
	// Timeout is the time limit of the call. 0 means HORUS_GENERATE_TIMEOUT.
	Timeout time.Duration
}

// Result is the result of generate_speech, in the key order that the tool shows.
type Result struct {
	Workflow      string  `json:"workflow"`
	Voice         string  `json:"voice"`
	Path          string  `json:"path"`
	Bytes         int64   `json:"bytes"`
	Format        string  `json:"format"`
	AudioSeconds  float64 `json:"audio_seconds"`
	Transcript    string  `json:"transcript"`
	Seconds       float64 `json:"seconds"`
	StartedWorker bool    `json:"started_worker"`
}

// Service makes speech files. It keeps one worker for each place of the
// speech setup.
type Service struct {
	newWorker NewWorker
	programs  Programs

	mu      sync.Mutex
	workers map[speechsetup.Place]Worker
	closed  bool
}

// New makes the service. cmd/horus gives it a function that makes the
// adapter internal/speechworker, and platform.Programs.
func New(newWorker NewWorker, programs Programs) *Service {
	return &Service{newWorker: newWorker, programs: programs, workers: map[speechsetup.Place]Worker{}}
}

// Close ends each speech worker that the service started. A call after Close
// answers GENERATION_FAILED.
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	workers := s.workers
	s.workers = map[speechsetup.Place]Worker{}
	s.mu.Unlock()
	for _, w := range workers {
		_ = w.Close()
	}
	return nil
}

// StopWorkers ends each speech worker that the service started, and waits
// until each one ended. The next call starts a new worker. uninstall_workflow
// calls it before it deletes the model files of a speech workflow: Windows
// does not delete a file that a process holds open, and a worker must not
// keep the models of deleted files. A call that runs in a worker that stops
// answers GENERATION_FAILED.
func (s *Service) StopWorkers() {
	s.mu.Lock()
	workers := s.workers
	s.workers = map[speechsetup.Place]Worker{}
	s.mu.Unlock()
	for _, w := range workers {
		_ = w.Close()
	}
}

// worker gives the worker of the place p, and makes it at the first call.
func (s *Service) worker(p speechsetup.Place) (Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, &WorkerError{Stopped: true, Message: "the speech worker is closed: horus mcp ends"}
	}
	if w, ok := s.workers[p]; ok {
		return w, nil
	}
	w := s.newWorker(p)
	s.workers[p] = w
	return w, nil
}

// Speak makes one speech file.
func (s *Service) Speak(ctx context.Context, r Request, o Options) (*Result, error) {
	t0 := time.Now()
	w, err := catalog.Get(r.Workflow)
	if err != nil {
		return nil, err
	}
	if k := w.Kind(); k != catalog.KindSpeech {
		e := invalid(fmt.Sprintf("workflow %s is of the kind %s: generate_speech makes speech only", quote(w.Name), quote(k)),
			map[string]any{"field": "workflow", "kind": k})
		e.NextStep = fmt.Sprintf("Call generate_image with workflow=%s, or choose a workflow of the kind speech from list_workflows.", quote(w.Name))
		return nil, e
	}
	c, err := check(w, r)
	if err != nil {
		return nil, err
	}
	place := speechsetup.Here()
	if err := s.installed(w, place); err != nil {
		return nil, err
	}
	timeout := o.Timeout
	if timeout == 0 {
		secs, err := settings.GenerateTimeout()
		if err != nil {
			return nil, err
		}
		timeout = time.Duration(secs) * time.Second
	}
	report := o.Report
	if report == nil {
		report = func(context.Context, float64, float64, string) {}
	}
	wk, err := s.worker(place)
	if err != nil {
		return nil, s.failed(w, err)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if wk.Running() {
		report(ctx, 0, 0, "making the speech file")
	} else {
		report(ctx, 0, 0, "starting the speech worker: the first call loads the models (5 to 20 s)")
	}
	type answer struct {
		sp  Speech
		err error
	}
	done := make(chan answer, 1)
	go func() {
		sp, err := wk.Speak(cctx, Text{Text: r.Text, Voice: c.voice, Out: c.path, Format: c.format})
		done <- answer{sp, err}
	}()
	tick := time.NewTicker(ReportInterval)
	defer tick.Stop()
	var a answer
	for wait := true; wait; {
		select {
		case a = <-done:
			wait = false
		case <-tick.C:
			elapsed := math.RoundToEven(time.Since(t0).Seconds())
			report(ctx, elapsed, 0, fmt.Sprintf("making the speech file, %d s", int64(elapsed)))
		}
	}
	if a.err != nil {
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case errors.Is(cctx.Err(), context.DeadlineExceeded):
			e := herr.New("GENERATION_TIMEOUT", fmt.Sprintf("the speech call did not finish in %s s", seconds(timeout)),
				map[string]any{"workflow": w.Name, "timeout_seconds": timeout.Seconds()})
			e.NextStep = "The call took longer than HORUS_GENERATE_TIMEOUT, and horus stopped the speech worker. " +
				"Call generate_speech again with a shorter text, or when the machine is less busy."
			return nil, e
		}
		return nil, s.failed(w, a.err)
	}
	fi, err := os.Stat(c.path)
	if err != nil || !fi.Mode().IsRegular() {
		e := herr.New("GENERATION_FAILED", "the speech worker answered, but no audio file is at "+c.path,
			map[string]any{"workflow": w.Name, "path": c.path, "worker_message": "no file"})
		e.NextStep = failedNextStep
		return nil, e
	}
	return &Result{
		Workflow:      w.Name,
		Voice:         c.voice,
		Path:          c.path,
		Bytes:         fi.Size(),
		Format:        c.format,
		AudioSeconds:  a.sp.AudioSeconds,
		Transcript:    a.sp.Transcript,
		Seconds:       math.Round(time.Since(t0).Seconds()*10) / 10,
		StartedWorker: a.sp.Started,
	}, nil
}

// failedNextStep is the next_step of GENERATION_FAILED of a speech call.
const failedNextStep = "Call generate_speech again with the same arguments: horus starts a new speech worker when the last one stopped. " +
	"If it fails two times, report details.worker_message to the user."

// tailLines is the count of the last lines of the error output of the
// worker that go into details.worker_output_tail.
const tailLines = 8

// failed maps an error of the worker to its horus error: a missing Python is
// WORKFLOW_NOT_INSTALLED, a *WorkerError is GENERATION_FAILED. Another error
// passes through.
func (s *Service) failed(w *catalog.Workflow, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		e := herr.New("WORKFLOW_NOT_INSTALLED", "the Python of the speech venv does not start: "+err.Error(),
			map[string]any{"workflow": w.Name, "missing": []string{"speech venv"}})
		e.NextStep = installNextStep(w)
		return e
	}
	var we *WorkerError
	if !errors.As(err, &we) {
		return err
	}
	e := herr.New("GENERATION_FAILED", we.Error(), map[string]any{
		"workflow":           w.Name,
		"worker_message":     we.Message,
		"worker_stopped":     we.Stopped,
		"worker_output_tail": tail(we.Output, tailLines),
	})
	e.NextStep = failedNextStep
	return e
}

// installed checks the parts of the speech setup of w at the place p.
func (s *Service) installed(w *catalog.Workflow, p speechsetup.Place) error {
	parts, err := speechsetup.Parts(w, p)
	if err != nil {
		return err
	}
	found := func(name string) bool { _, err := s.programs.LookPath(name); return err == nil }
	missing := speechsetup.Missing(parts, fileSize, found)
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, len(missing))
	for i, m := range missing {
		names[i] = m.Name
	}
	e := herr.New("WORKFLOW_NOT_INSTALLED", "parts of the speech setup are missing: "+strings.Join(names, ", "),
		map[string]any{"workflow": w.Name, "missing": names, "parts": missing, "speech_dir": p.Dir})
	e.NextStep = installNextStep(w)
	return e
}

func installNextStep(w *catalog.Workflow) string {
	return fmt.Sprintf("Call install_workflow with workflow=%s, then call generate_speech again.", quote(w.Name))
}

// fileSize gives the size of the file at path, or -1 when no file is there.
func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return -1
	}
	return fi.Size()
}

// tail gives the last n lines of text that are not empty.
func tail(text string, n int) []string {
	lines := []string{}
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimRight(l, "\r"); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// seconds writes a duration in seconds, without a fraction when it has none.
func seconds(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprint(int64(d / time.Second))
	}
	return fmt.Sprint(d.Seconds())
}
