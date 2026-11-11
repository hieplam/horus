package backend

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary is also each helper program of the tests. The same helpers
// run on Windows, macOS and Linux:
//
//	HORUS_FAKE_7Z     the fake 7z.exe of the install tests (install_test.go)
//	HORUS_FAKE_PY     the fake python.exe of the portable build: "1" records its
//	                  start and sleeps; "serve" also starts a grandchild and
//	                  answers /system_stats on its --port, as ComfyUI does
//	HORUS_FAKE_PY=grandchild  a process that the fake ComfyUI started: it sleeps
//	HORUS_BACKEND_HELPER=start  calls Portable.Start for HORUS_COMFYUI_DIR and
//	                  HORUS_COMFYUI_URL, then ends: the starter of the stop tests.
//	                  Its fake python gets the mode HORUS_HELPER_PY.
func TestMain(m *testing.M) {
	if os.Getenv("HORUS_FAKE_7Z") != "" {
		os.Exit(fake7z(os.Args[1:]))
	}
	switch os.Getenv("HORUS_FAKE_PY") {
	case "":
	case "grandchild":
		time.Sleep(helperLife)
		os.Exit(0)
	default:
		os.Exit(fakePython(os.Args[1:]))
	}
	if os.Getenv("HORUS_BACKEND_HELPER") == "start" {
		// The fake python gets its mode from the environment of this starter.
		os.Setenv("HORUS_FAKE_PY", os.Getenv("HORUS_HELPER_PY"))
		p :=&Portable{Dir: os.Getenv("HORUS_COMFYUI_DIR"), URL: os.Getenv("HORUS_COMFYUI_URL"),
			Timeout: 20 * time.Second, PollInterval: 20 * time.Millisecond}
		if err := p.Start(context.Background(), os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "Start:", err)
			os.Exit(1)
		}
		os.Exit(0) // no Wait: the starter ends while ComfyUI runs
	}
	os.Exit(m.Run())
}

// helperLife is how long a fake process lives when nothing ends it. Each
// test ends its processes long before.
const helperLife = 2 * time.Minute

// fakePython records its process id, its folder and its arguments, writes one
// line to each output, then exits with HORUS_FAKE_EXIT, or waits. In the mode
// "serve" it starts a grandchild, adds "child <pid>" to the record, and
// answers /system_stats on the port of its --port argument.
func fakePython(args []string) int {
	wd, _ := os.Getwd()
	rec := fmt.Sprintf("pid %d\ndir %s\nargs %s\n", os.Getpid(), wd, strings.Join(args, " "))
	var ln net.Listener
	if os.Getenv("HORUS_FAKE_PY") == "serve" {
		gc := exec.Command(os.Args[0])
		gc.Env = append(os.Environ(), "HORUS_FAKE_PY=grandchild")
		if err := gc.Start(); err != nil {
			return 4
		}
		rec += fmt.Sprintf("child %d\n", gc.Process.Pid)
		port := ""
		for i, a := range args {
			if a == "--port" && i+1 < len(args) {
				port = args[i+1]
			}
		}
		var err error
		if ln, err = net.Listen("tcp", "127.0.0.1:"+port); err != nil {
			return 5
		}
	}
	if err := os.WriteFile(os.Getenv("HORUS_FAKE_RECORD")+".tmp", []byte(rec), 0o644); err == nil {
		os.Rename(os.Getenv("HORUS_FAKE_RECORD")+".tmp", os.Getenv("HORUS_FAKE_RECORD"))
	}
	fmt.Fprintln(os.Stdout, "fake comfyui stdout line")
	fmt.Fprintln(os.Stderr, "fake comfyui stderr line")
	if v := os.Getenv("HORUS_FAKE_EXIT"); v != "" {
		n, _ := strconv.Atoi(v)
		return n
	}
	if ln != nil {
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"system":{}}`)
		}))
	}
	time.Sleep(helperLife)
	return 0
}

// freeURL gives the address of a free port on 127.0.0.1. It is never 8188.
func freeURL(t *testing.T) string {
	t.Helper()
	for {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		l.Close()
		if !strings.HasSuffix(addr, ":8188") && !strings.HasSuffix(addr, ":8189") {
			return "http://" + addr
		}
	}
}

// killAtEnd ends the process pid when the test ends, if it still runs:
// Windows cannot delete a folder that a running program holds open.
func killAtEnd(t *testing.T, pid int) {
	t.Cleanup(func() {
		if pid > 0 && alive(pid) {
			if p, err := os.FindProcess(pid); err == nil {
				p.Kill()
				for end := time.Now().Add(5 * time.Second); alive(pid) && time.Now().Before(end); {
					time.Sleep(20 * time.Millisecond)
				}
			}
		}
	})
}
