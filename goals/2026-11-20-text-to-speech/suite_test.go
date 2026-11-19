//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-20-text-to-speech.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-20-text-to-speech
//
// The rows speech-hidden, speech-errors and speech-contract build horus from
// this repo and run tests/mcp_contract.py against that program one time. They
// read its output and its report (--report): each check, and the answers of
// the server to the speech checks. Two rows also ask the built server
// themselves, over stdio, so that a report with fixed text cannot pass them.
// They need Go, and Python 3.12 with the packages of tests/requirements.txt.
package goal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The task that builds the outcome of a row gives the row its oracle.

func TestSpeechReal(t *testing.T) {
	t.Fatal("not built: generate_speech with the real models, 5 fixed sentences, the loudness and the transcript of each file")
}

func TestSpeechHidden(t *testing.T) {
	run := contract(t)
	row := run.check(t, "speech-hidden")
	if !row.OK {
		t.Errorf("the contract check of the schema failed: %s -- %s", row.Name, row.Detail)
	}
	// The schema as this suite gets it from the built server.
	srv := startServer(t, run.exe, sandboxEnv(t))
	schema := srv.speechSchema(t)
	var props map[string]struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(schema["properties"], &props); err != nil {
		t.Fatalf("the input schema of generate_speech has no properties: %v", err)
	}
	names := []string{}
	shown := 0 // the arguments that show the speech setup (the ratchet: 0, and it stays 0)
	for name, p := range props {
		names = append(names, name)
		text := strings.ToLower(name + " " + p.Title + " " + p.Description)
		for _, w := range setupWords {
			if strings.Contains(text, w) {
				shown++
				t.Errorf("the argument %s names %q: %s", name, w, p.Description)
				break
			}
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "out_path,overwrite,text,voice,workflow" {
		t.Errorf("generate_speech has the arguments %v, want out_path, overwrite, text, voice, workflow", names)
	}
	var required []string
	_ = json.Unmarshal(schema["required"], &required)
	sort.Strings(required)
	if strings.Join(required, ",") != "text,workflow" {
		t.Errorf("the required arguments are %v, want text and workflow", required)
	}
	// The contract test read the same schema from the server that it tested.
	if !sameJSON(run.report.Speech.InputSchema, schema) {
		t.Errorf("the schema in the report of the contract test is not the schema of the built server")
	}
	t.Logf("generate_speech arguments: %v; arguments that show the speech setup: %d", names, shown)
}

func TestSpeechInstall(t *testing.T) {
	t.Fatal("not built: install_workflow of a speech workflow, and the uninstall, in a sandbox home folder")
}

// failureKeys are the keys of the contract checks of the spec table
// "Failures", in the order of its rows.
var failureKeys = []string{"text-length", "text-tag", "voice", "out-path", "kind", "output-exists", "not-installed", "worker-failed"}

func TestSpeechErrors(t *testing.T) {
	rows := failureRows(t)
	if len(rows) != len(failureKeys) {
		t.Fatalf("the spec table Failures has %d rows, the suite knows %d: %v", len(rows), len(failureKeys), rows)
	}
	run := contract(t)
	checked := 0
	for i, want := range rows {
		key := "speech-failure:" + failureKeys[i]
		row := run.check(t, key)
		answers := run.report.Speech.Answers[key]
		ok := row.OK && len(answers) > 0
		for _, a := range answers {
			ok = ok && a.Code == want.code && a.ActionBy == want.actionBy && a.NextStep != "" && a.Details != nil
		}
		if !ok {
			t.Errorf("failure %q: want %s and action_by %s with a next_step; the check %q (%v) got %+v", want.failure, want.code, want.actionBy, row.Name, row.OK, answers)
			continue
		}
		checked++
	}
	// Two failures as this suite gets them from the built server: no speech setup at all.
	srv := startServer(t, run.exe, sandboxEnv(t))
	voices := standInVoices(t)
	e := srv.speechError(t, map[string]any{"workflow": "stand-in-speech", "text": "Xin chào các bạn.", "voice": "no-such-voice"})
	if e.Code != "INVALID_ARGUMENT" || e.ActionBy != "caller" || !sameJSON(e.Details["known"], voices) {
		t.Errorf("an unknown voice: %+v, want INVALID_ARGUMENT with details.known %v", e, voices)
	}
	e = srv.speechError(t, map[string]any{"workflow": "stand-in-speech", "text": "Xin chào các bạn."})
	missing, _ := e.Details["missing"].([]any)
	if e.Code != "WORKFLOW_NOT_INSTALLED" || e.ActionBy != "caller" || !slices.Contains(missing, any("speech venv")) || !strings.Contains(e.NextStep, "install_workflow") {
		t.Errorf("no speech setup: %+v, want WORKFLOW_NOT_INSTALLED naming the speech venv", e)
	}
	t.Logf("speech failures with a check: %d of %d", checked, len(rows))
}

func TestSpeechContract(t *testing.T) {
	run := contract(t)
	if run.code != 0 {
		t.Errorf("tests/mcp_contract.py exit code %d, want 0", run.code)
	}
	m := regexp.MustCompile(`(?m)^contract: (\d+) of (\d+) passed\s*$`).FindAllStringSubmatch(run.out, -1)
	if len(m) != 1 {
		t.Fatalf("the output has %d lines 'contract: N of M passed', want 1", len(m))
	}
	pass := len(regexp.MustCompile(`(?m)^PASS  `).FindAllString(run.out, -1))
	fail := regexp.MustCompile(`(?m)^FAIL  .*$`).FindAllString(run.out, -1)
	var n, total int
	fmt.Sscan(m[0][1], &n)
	fmt.Sscan(m[0][2], &total)
	if n != total || n != pass || len(fail) != 0 {
		t.Errorf("contract: %d of %d passed, %d PASS lines, FAIL lines %q", n, total, pass, fail)
	}
	if total < 47 {
		t.Errorf("the contract test has %d checks, want at least 47", total)
	}
	r := run.report
	if r.Total != total || len(r.Checks) != total || r.Passed != total {
		t.Errorf("the report has %d checks, %d passed, total %d; the output says %d of %d", len(r.Checks), r.Passed, r.Total, n, total)
	}
	speech, image := 0, 0
	for _, c := range r.Checks {
		if !c.OK {
			t.Errorf("FAIL in the report: %s -- %s", c.Name, c.Detail)
		}
		if strings.HasPrefix(c.Key, "speech") {
			speech++
		} else if c.OK {
			image++
		}
	}
	for _, key := range append([]string{"speech-hidden", "speech-file", "speech-progress", "speech-worker-kept"}, prefixed(failureKeys)...) {
		run.check(t, key)
	}
	if image < 37 {
		t.Errorf("%d checks of the image tools passed, want the 37 of the spec", image)
	}
	t.Logf("contract checks: %d of %d passed (%d of the image tools and the setup, %d speech checks)", n, total, image, speech)
}

func TestSpeechMacos(t *testing.T) {
	t.Fatal("not built: a speech check in the macOS job of CI")
}

// ---------------------------------------------------------------------------
// The contract run.
// ---------------------------------------------------------------------------

// root is the repo root. go test starts a test in the folder of its package.
var root = filepath.Join("..", "..")

// setupWords are the words that no argument of generate_speech may have: the
// caller gives only a text and a voice.
var setupWords = []string{"engine", "python", "ffmpeg", "ffprobe", "loudness", "lufs", "venv", "vieneu", "whisper"}

// report is the file of tests/mcp_contract.py --report.
type report struct {
	Total  int        `json:"total"`
	Passed int        `json:"passed"`
	Checks []checkRow `json:"checks"`
	Speech struct {
		InputSchema map[string]json.RawMessage `json:"input_schema"`
		Answers     map[string][]errorInfo     `json:"answers"`
	} `json:"speech"`
}

// checkRow is one check of the report.
type checkRow struct {
	Key    string `json:"key"` // "" for a check without a key
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// errorInfo is the error of an envelope.
type errorInfo struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	ActionBy  string         `json:"action_by"`
	NextStep  string         `json:"next_step"`
	Details   map[string]any `json:"details"`
}

// contractRun is the one run of the contract test.
type contractRun struct {
	exe    string // the horus program that this suite built
	code   int    // the exit code of tests/mcp_contract.py
	out    string // its output
	report report
	fail   string // why the run did not happen
}

var (
	once sync.Once
	run  contractRun
	temp string // the folder of the build and the report
)

func TestMain(m *testing.M) {
	code := m.Run()
	if temp != "" {
		os.RemoveAll(temp)
	}
	os.Exit(code)
}

// contract builds horus and runs the contract test against it, one time for
// the suite.
func contract(t *testing.T) *contractRun {
	t.Helper()
	once.Do(func() {
		var err error
		if temp, err = os.MkdirTemp("", "horus-goal-speech-"); err != nil {
			run.fail = err.Error()
			return
		}
		goexe, err := command(root, nil, "go", "env", "GOEXE").Output() // ".exe" on Windows, "" elsewhere
		if err != nil {
			run.fail = fmt.Sprintf("go env GOEXE: %v", err)
			return
		}
		run.exe = filepath.Join(temp, "horus"+strings.TrimSpace(string(goexe)))
		if out, err := command(root, nil, "go", "build", "-o", run.exe, "./cmd/horus").CombinedOutput(); err != nil {
			run.fail = fmt.Sprintf("go build ./cmd/horus: %v\n%s", err, out)
			return
		}
		py, why := python()
		if py == "" {
			run.fail = why
			return
		}
		file := filepath.Join(temp, "report.json")
		var out bytes.Buffer
		cmd := command(root, nil, py, filepath.Join("tests", "mcp_contract.py"), "--server-cmd", run.exe, "mcp", "--report", file)
		cmd.Stdout, cmd.Stderr = &out, &out
		err = cmd.Run()
		run.out = out.String()
		var exit *exec.ExitError
		switch {
		case errors.As(err, &exit):
			run.code = exit.ExitCode()
		case err != nil:
			run.fail = fmt.Sprintf("tests/mcp_contract.py did not start: %v", err)
			return
		}
		data, err := os.ReadFile(file)
		if err != nil {
			run.fail = fmt.Sprintf("tests/mcp_contract.py wrote no report (exit code %d):\n%s", run.code, tail(run.out))
			return
		}
		if err := json.Unmarshal(data, &run.report); err != nil {
			run.fail = fmt.Sprintf("the report of tests/mcp_contract.py: %v", err)
		}
	})
	if run.fail != "" {
		t.Fatal(run.fail)
	}
	if run.code != 0 {
		t.Logf("the end of the output of tests/mcp_contract.py:\n%s", tail(run.out))
	}
	return &run
}

// check gives the contract check with the key. A missing check fails the row.
func (r *contractRun) check(t *testing.T, key string) checkRow {
	t.Helper()
	for _, c := range r.report.Checks {
		if c.Key == key {
			if !c.OK {
				t.Errorf("the contract check %q failed: %s", c.Name, c.Detail)
			}
			return c
		}
	}
	t.Errorf("the contract test has no check with the key %s", key)
	return checkRow{}
}

func prefixed(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = "speech-failure:" + k
	}
	return out
}

// command is a program in the folder dir, with more environment env.
func command(dir string, env []string, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// python gives a Python program that has the MCP client: the test venv of
// docs/mcp.md (Windows, then macOS and Linux), then python3, then python.
func python() (string, string) {
	win, _ := filepath.Abs(filepath.Join(root, ".scratch", "test-venv", "Scripts", "python.exe"))
	unix, _ := filepath.Abs(filepath.Join(root, ".scratch", "test-venv", "bin", "python"))
	for _, name := range []string{win, unix, "python3", "python"} {
		if exec.Command(name, "-c", "import mcp").Run() == nil {
			return name, ""
		}
	}
	return "", "no Python with the package mcp: run pip install -r tests/requirements.txt (docs/mcp.md, \"Test the server\")"
}

// tail gives the last 40 lines of text.
func tail(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

// sameJSON tells if a and b are the same JSON value.
func sameJSON(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	var va, vb any
	if json.Unmarshal(ja, &va) != nil || json.Unmarshal(jb, &vb) != nil {
		return false
	}
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return bytes.Equal(na, nb)
}

// failure is one row of the spec table "Failures".
type failure struct{ failure, code, actionBy string }

// failureRows reads the table "Failures" of the spec: the failure, the code
// and who acts.
func failureRows(t *testing.T) []failure {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "docs", "specs", "2026-11-20-text-to-speech.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []failure
	in := false
	cell := regexp.MustCompile("^`([A-Z_]+)`$")
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(strings.TrimRight(ln, "\r"))
		if strings.HasPrefix(ln, "#") {
			in = ln == "### Failures"
			continue
		}
		if !in || !strings.HasPrefix(ln, "|") {
			continue
		}
		parts := strings.Split(strings.Trim(ln, "|"), "|")
		if len(parts) != 3 {
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if m := cell.FindStringSubmatch(parts[1]); m != nil {
			rows = append(rows, failure{parts[0], m[1], parts[2]})
		}
	}
	return rows
}

// standInVoices are the voices of the stand-in speech manifest.
func standInVoices(t *testing.T) []string {
	t.Helper()
	var m struct {
		Voices []string `json:"voices"`
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "catalog", "testdata", "stand-in-speech", "manifest.json"))
	if err != nil || json.Unmarshal(data, &m) != nil || len(m.Voices) == 0 {
		t.Fatalf("the stand-in speech manifest: %v", err)
	}
	return m.Voices
}

// ---------------------------------------------------------------------------
// The suite's own client of the built server: JSON-RPC lines over stdio.
// ---------------------------------------------------------------------------

// sandboxEnv is the environment of a server with the stand-in speech workflow
// only, and an empty speech folder, in a temporary folder.
func sandboxEnv(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	flows := filepath.Join(dir, "workflows", "stand-in-speech")
	if err := os.MkdirAll(flows, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "catalog", "testdata", "stand-in-speech", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flows, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"HORUS_ROOT=" + dir, "HORUS_HOME=" + filepath.Join(dir, "home"), "HORUS_WORKFLOWS_DIR=" + filepath.Join(dir, "workflows"),
		"HORUS_SPEECH_DIR=" + filepath.Join(dir, "speech"), "HORUS_OUTPUT_DIR=" + filepath.Join(dir, "out"), "HORUS_AUTOSTART=0"}
}

type server struct {
	in   io.WriteCloser
	out  *bufio.Reader
	next int
}

// startServer starts `<exe> mcp` and does the MCP initialize. The end of the
// test ends the server.
func startServer(t *testing.T, exe string, env []string) *server {
	t.Helper()
	cmd := command(t.TempDir(), env, exe, "mcp")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s mcp: %v", exe, err)
	}
	t.Cleanup(func() {
		in.Close()
		done := make(chan struct{})
		go func() { cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			cmd.Process.Kill()
			<-done
		}
	})
	s := &server{in: in, out: bufio.NewReader(out)}
	s.call(t, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "goal-suite", "version": "0"}})
	s.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return s
}

func (s *server) send(t *testing.T, msg any) {
	t.Helper()
	line, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.in.Write(append(line, '\n')); err != nil {
		t.Fatalf("write to the server: %v", err)
	}
}

// call sends one request and gives the result of its answer.
func (s *server) call(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()
	s.next++
	id := s.next
	s.send(t, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	for {
		line, err := s.out.ReadBytes('\n')
		if err != nil {
			t.Fatalf("%s: the server ended without an answer: %v", method, err)
		}
		var msg struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(line, &msg) != nil || msg.ID == nil || *msg.ID != id {
			continue // a notification or a line of another request
		}
		if msg.Error != nil {
			t.Fatalf("%s: %s", method, msg.Error)
		}
		return msg.Result
	}
}

// speechSchema gives the input schema of generate_speech from tools/list.
func (s *server) speechSchema(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var list struct {
		Tools []struct {
			Name        string                     `json:"name"`
			InputSchema map[string]json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(s.call(t, "tools/list", map[string]any{}), &list); err != nil {
		t.Fatal(err)
	}
	for _, tl := range list.Tools {
		if tl.Name == "generate_speech" {
			return tl.InputSchema
		}
	}
	t.Fatal("tools/list of the built server has no generate_speech")
	return nil
}

// speechError calls generate_speech and gives the error of its envelope. A
// call without an error fails the row.
func (s *server) speechError(t *testing.T, args map[string]any) errorInfo {
	t.Helper()
	var res struct {
		Structured struct {
			OK    bool       `json:"ok"`
			Error *errorInfo `json:"error"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(s.call(t, "tools/call", map[string]any{"name": "generate_speech", "arguments": args}), &res); err != nil {
		t.Fatal(err)
	}
	if res.Structured.OK || res.Structured.Error == nil {
		t.Fatalf("generate_speech %v: no error", args)
		return errorInfo{}
	}
	return *res.Structured.Error
}
