// Package comfyclitest is a fake of the program comfy (comfy-cli) for the Go
// tests. It holds no code of comfy-cli: it only answers the commands that
// horus sends, as comfy-cli 1.25.0 answers them.
//
// The test binary is the fake. A test copies it as the program comfy into a
// folder (Fake.Put), and TestMain calls Main first, so that the copy acts as
// comfy when it runs.
//
// Each run appends two lines to the record file: its arguments, and
// "stdin closed" or "stdin open". The commands:
//
//	install  writes a line and makes <workspace>/main.py
//	launch --background -- <args of ComfyUI>
//	         starts a stand-in ComfyUI as a background process that answers
//	         /system_stats on the --port of the args, waits until it answers,
//	         keeps its process id in the background record (one for each
//	         user, as comfy-cli does), then ends. With the global flag --json
//	         it prints the envelope of comfy-cli with that process id. It
//	         refuses a second background ComfyUI.
//	stop     ends the process of the background record and deletes the
//	         record. Without one, it exits 1.
//
// The stand-in ComfyUI sends each request to Fake.Proxy when a test sets it:
// then it is the fake ComfyUI of comfytest on the port of horus.
package comfyclitest

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// The environment of the fake.
const (
	envMode       = "HORUS_FAKE_COMFY"    // the mode; empty = the test binary is no fake
	envRecord     = "HORUS_FAKE_RECORD"   // the record file of the arguments
	envBackground = "HORUS_FAKE_COMFY_BG" // the background record: the process id of the stand-in ComfyUI
	envUpstream   = "HORUS_FAKE_UPSTREAM" // the address that the stand-in ComfyUI sends each request to
)

// The modes of the fake.
const (
	OK     = "1"      // every command works
	Fail   = "fail"   // every command exits 3
	NoMain = "nomain" // install makes no main.py
	NoPID  = "nopid"  // launch prints an envelope without a process id
	server = "server" // the stand-in ComfyUI (a process that launch starts)
)

// life is how long the stand-in ComfyUI lives when nothing ends it. Each
// test ends it long before.
const life = 2 * time.Minute

// Main runs the fake when the environment asks for it, and then ends the
// process. Else it returns. TestMain calls it before m.Run.
func Main() {
	switch mode := os.Getenv(envMode); mode {
	case "":
		return
	case server:
		os.Exit(serve(os.Args[1:]))
	default:
		os.Exit(run(mode, os.Args[1:]))
	}
}

// Fake is the fake comfy of one test.
type Fake struct {
	Record     string // the record file of the arguments
	Background string // the background record
}

// New sets the environment of a fake comfy in the mode OK for the test t,
// with its files in a temporary folder. At the end of the test it ends the
// stand-in ComfyUI, if it still runs.
func New(t *testing.T) *Fake {
	t.Helper()
	tmp := t.TempDir()
	f := &Fake{Record: filepath.Join(tmp, "comfy.record"), Background: filepath.Join(tmp, "comfy.background")}
	t.Setenv(envMode, OK)
	t.Setenv(envRecord, f.Record)
	t.Setenv(envBackground, f.Background)
	t.Cleanup(func() {
		if pid := f.ServerPID(); pid > 0 {
			end(pid)
		}
	})
	return f
}

// Put copies the test binary as the program comfy into folder, and gives
// its path.
func (f *Fake) Put(t *testing.T, folder string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(folder, platform.ExeName("comfy"))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

// SetMode sets the mode of the next runs.
func (f *Fake) SetMode(t *testing.T, mode string) { t.Setenv(envMode, mode) }

// Proxy makes the stand-in ComfyUI send each request to the address
// upstream, for example the fake ComfyUI of comfytest.
func (f *Fake) Proxy(t *testing.T, upstream string) { t.Setenv(envUpstream, upstream) }

// Recorded gives the lines of the record file.
func (f *Fake) Recorded() string {
	b, _ := os.ReadFile(f.Record)
	return string(b)
}

// ServerPID gives the process id of the background record, or 0.
func (f *Fake) ServerPID() int { return backgroundPID(f.Background) }

// -- the program comfy ---------------------------------------------------------

func run(mode string, args []string) int {
	record(args)
	if mode == Fail {
		return 3
	}
	workspace, asJSON, cmd, rest := split(args)
	switch cmd {
	case "install":
		os.Stdout.WriteString("fake comfy install line\n")
		if mode != NoMain && workspace != "" {
			os.MkdirAll(workspace, 0o755)
			os.WriteFile(filepath.Join(workspace, "main.py"), []byte("# fake\n"), 0o644)
		}
		return 0
	case "launch":
		return launch(mode, workspace, asJSON, rest)
	case "stop":
		return stop()
	}
	fmt.Fprintf(os.Stderr, "fake comfy: no command %q\n", cmd)
	return 2
}

// record appends the arguments and the state of the standard input to the
// record file.
func record(args []string) {
	stdin := "stdin open"
	read := make(chan error, 1)
	go func() { _, err := io.ReadAll(os.Stdin); read <- err }()
	select {
	case err := <-read:
		if err == nil {
			stdin = "stdin closed"
		}
	case <-time.After(2 * time.Second):
	}
	if f, err := os.OpenFile(os.Getenv(envRecord), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(strings.Join(args, " ") + "\n" + stdin + "\n")
		f.Close()
	}
}

// split gives the global options (--workspace, --json), the command, and the
// arguments after the command.
func split(args []string) (workspace string, asJSON bool, cmd string, rest []string) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--workspace" && i+1 < len(args):
			workspace = args[i+1]
			i++
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, "-"):
		default:
			return workspace, asJSON, a, args[i+1:]
		}
	}
	return workspace, asJSON, "", nil
}

func launch(mode, workspace string, asJSON bool, rest []string) int {
	if len(rest) == 0 || rest[0] != "--background" {
		fmt.Fprintln(os.Stderr, "fake comfy: only launch --background")
		return 2
	}
	var extra []string
	for i, a := range rest {
		if a == "--" {
			extra = rest[i+1:]
		}
	}
	port := valueOf(extra, "--port")
	bg := os.Getenv(envBackground)
	if pid := backgroundPID(bg); pid > 0 && platform.Alive(pid) {
		fmt.Fprintln(os.Stderr, "ComfyUI is already running in background.")
		if asJSON {
			envelope(false, map[string]any{"code": "server_already_running", "message": "ComfyUI is already running in background."}, nil)
		}
		return 1
	}
	logDir := filepath.Join(workspace, "user")
	os.MkdirAll(logDir, 0o755)
	logFile, err := os.Create(filepath.Join(logDir, "comfyui_"+port+".log"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake comfy:", err)
		return 1
	}
	defer logFile.Close()
	child := exec.Command(os.Args[0], extra...)
	child.Env = append(os.Environ(), envMode+"="+server)
	child.Stdout, child.Stderr = logFile, logFile
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "fake comfy:", err)
		return 1
	}
	pid := child.Process.Pid
	if !waitAnswer("http://127.0.0.1:"+port, 20*time.Second) {
		end(pid)
		fmt.Fprintln(os.Stderr, "Execution error: failed to launch ComfyUI")
		return 1
	}
	os.WriteFile(bg, []byte(strconv.Itoa(pid)), 0o644)
	data := map[string]any{"background": true, "listen": valueOf(extra, "--listen"), "port": port}
	if mode != NoPID {
		data["pid"] = pid
	}
	if asJSON {
		envelope(true, nil, data)
	} else {
		fmt.Println("ComfyUI is successfully launched in the background.")
	}
	return 0 // no Wait: the stand-in ComfyUI runs after this process ends
}

func stop() int {
	bg := os.Getenv(envBackground)
	pid := backgroundPID(bg)
	if pid <= 0 || !platform.Alive(pid) {
		fmt.Fprintln(os.Stderr, "No ComfyUI is running in the background.")
		return 1
	}
	end(pid)
	os.Remove(bg)
	fmt.Println("Background ComfyUI is stopped.")
	return 0
}

// envelope prints one envelope line of comfy --json on the standard output.
func envelope(ok bool, errBlock map[string]any, data map[string]any) {
	b, _ := json.Marshal(map[string]any{"schema": "envelope/1", "type": "envelope", "ok": ok, "command": "launch",
		"data": data, "error": errBlock})
	fmt.Println(string(b))
}

func valueOf(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func backgroundPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

// end ends the process pid and waits up to 10 s until it is gone.
func end(pid int) {
	if p, err := os.FindProcess(pid); err == nil && platform.Alive(pid) {
		p.Kill()
	}
	for deadline := time.Now().Add(10 * time.Second); platform.Alive(pid) && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
}

func waitAnswer(base string, wait time.Duration) bool {
	client := &http.Client{Timeout: time.Second}
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if resp, err := client.Get(base + "/system_stats"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
	}
	return false
}

// -- the stand-in ComfyUI ------------------------------------------------------

// serve answers on 127.0.0.1:<--port>: /system_stats, or each request
// through the proxy to HORUS_FAKE_UPSTREAM.
func serve(args []string) int {
	ln, err := net.Listen("tcp", "127.0.0.1:"+valueOf(args, "--port"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "stand-in ComfyUI:", err)
		return 5
	}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system_stats" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"system":{}}`)
	})
	if up := os.Getenv(envUpstream); up != "" {
		u, err := url.Parse(up)
		if err != nil {
			return 6
		}
		h = httputil.NewSingleHostReverseProxy(u)
	}
	fmt.Println("To see the GUI go to: http://" + ln.Addr().String())
	go http.Serve(ln, h)
	time.Sleep(life)
	return 0
}
