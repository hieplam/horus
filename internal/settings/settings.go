// Package settings holds the settings of horus.
//
// Each setting comes from, in this order: the environment variable, the .env
// file (EnvPath), the default. An empty value counts as not set. The .env
// file can hold HF_TOKEN. This package never returns or logs that token: only
// the download code reads it.
//
// The rules of the spec (docs/specs/2026-11-15-release-install.md, "A machine
// with only the program"):
//
//   - The repo root is HORUS_ROOT, then the nearest folder at or above the
//     current folder that is a horus clone: a folder with
//     workflows/_template/README.md. Without both, there is no repo root.
//   - The user folder is HORUS_HOME, then <home>/horus (platform.UserDir).
//   - .env is <repo root>/.env, or <user folder>/.env without a repo root.
//   - The workflows folder is HORUS_WORKFLOWS_DIR, then <repo root>/workflows,
//     then the copy of the built-in workflows in <user folder>/builtin-workflows.
//   - The output folder is HORUS_OUTPUT_DIR, then <repo root>/.scratch/outputs,
//     then <user folder>/outputs.
package settings

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/hieplam/horus/internal/platform"
)

var (
	mu       sync.Mutex
	rootDone bool // root holds the result of findRoot
	root     string
	envCache map[string]string
	copyDir  string // the copy of the built-in workflows, after it was checked
)

// Reset forgets the cached repo root, the .env values and the checked copy
// of the built-in workflows. Tests use it.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	rootDone, root, envCache, copyDir = false, "", nil, ""
}

// Root is the repo root, or "" when there is no repo root (a machine with
// only the program).
func Root() string {
	mu.Lock()
	defer mu.Unlock()
	return rootLocked()
}

func rootLocked() string {
	if !rootDone {
		root, rootDone = findRoot(), true
	}
	return root
}

func findRoot() string {
	if r := os.Getenv("HORUS_ROOT"); r != "" {
		return r
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; ; {
		if IsClone(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// IsClone tells if dir is a horus clone: it has workflows/_template/README.md.
// A folder that only has a workflows folder is not a clone.
func IsClone(dir string) bool {
	return platform.IsFile(filepath.Join(dir, "workflows", "_template", "README.md"))
}

// UserDir is the user folder of horus: HORUS_HOME, then <home>/horus. Only
// the environment sets HORUS_HOME, because the .env file can be in the user
// folder.
func UserDir() string {
	home, _ := os.UserHomeDir()
	return platform.UserDir(os.Getenv("HORUS_HOME"), home)
}

// EnvPath is the path of the .env file that horus reads: <repo root>/.env,
// or <user folder>/.env when there is no repo root. The file can be absent.
func EnvPath() string {
	mu.Lock()
	r := rootLocked()
	mu.Unlock()
	return envPathFor(r)
}

func envPathFor(root string) string {
	if root != "" {
		return filepath.Join(root, ".env")
	}
	return filepath.Join(UserDir(), ".env")
}

// ParseEnv reads the lines of a .env file: it skips empty
// lines, comments and lines without "=", strips quotes from the value, and
// drops a key whose value is empty.
func ParseEnv(data []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		value = strings.Trim(strings.Trim(strings.TrimSpace(value), `"`), "'")
		if value != "" {
			out[strings.TrimSpace(key)] = value
		}
	}
	return out
}

func envFile() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	if envCache == nil {
		envCache = map[string]string{}
		if data, err := os.ReadFile(envPathFor(rootLocked())); err == nil {
			envCache = ParseEnv(data)
		}
	}
	return envCache
}

// Get returns a setting: the environment variable, then .env, then def.
// It panics for HF_TOKEN, as settings.get raises ValueError: horus code other
// than the downloader must not read the token.
func Get(name, def string) string {
	if name == "HF_TOKEN" {
		panic("horus code must not read the token; horus download-model reads it")
	}
	if v := os.Getenv(name); v != "" {
		return v
	}
	if v := envFile()[name]; v != "" {
		return v
	}
	return def
}

// ComfyURL is the ComfyUI address, without a trailing slash.
func ComfyURL() string {
	return strings.TrimRight(Get("HORUS_COMFYUI_URL", "http://127.0.0.1:8188"), "/")
}

// ComfyDir is the install folder of ComfyUI: HORUS_COMFYUI_DIR, then the
// default folder of this OS (platform.DefaultComfyDir).
func ComfyDir() string {
	home, _ := os.UserHomeDir()
	return Get("HORUS_COMFYUI_DIR", platform.DefaultComfyDir(home))
}

// ComfyMCPVenv is the Python venv of horus install-comfy-mcp:
// <home>/.venvs/comfy-mcp. It holds the programs comfy-mcp and comfy.
func ComfyMCPVenv() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".venvs", "comfy-mcp")
}

// WorkflowsDir is the folder of the workflows: HORUS_WORKFLOWS_DIR, then
// <repo root>/workflows, then the copy of the built-in workflows. The first
// call without both settings writes the copy when it is absent or old.
func WorkflowsDir() string {
	if v := Get("HORUS_WORKFLOWS_DIR", ""); v != "" {
		return v
	}
	if r := Root(); r != "" {
		return filepath.Join(r, "workflows")
	}
	return BuiltinWorkflowsDir()
}

// OutputDir is where a PNG goes when a call has no out_path:
// HORUS_OUTPUT_DIR, then <repo root>/.scratch/outputs, then
// <user folder>/outputs.
func OutputDir() string {
	if v := Get("HORUS_OUTPUT_DIR", ""); v != "" {
		return v
	}
	if r := Root(); r != "" {
		return filepath.Join(r, ".scratch", "outputs")
	}
	return filepath.Join(UserDir(), "outputs")
}

// Autostart reports if horus starts ComfyUI when it does not answer.
// HORUS_AUTOSTART=0 (or false, no) turns this off.
func Autostart() bool {
	switch Get("HORUS_AUTOSTART", "1") {
	case "0", "false", "no":
		return false
	}
	return true
}

// GenerateTimeout is the time limit of one image job, in seconds.
func GenerateTimeout() (int, error) {
	v := Get("HORUS_GENERATE_TIMEOUT", "1800")
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("HORUS_GENERATE_TIMEOUT is not an integer: %q", v)
	}
	return n, nil
}

// DatabaseURL is HORUS_DATABASE_URL: the connection to horus-db. When it is
// set, the job queue is on. Empty: the direct path.
func DatabaseURL() string { return strings.TrimSpace(Get("HORUS_DATABASE_URL", "")) }

// positive reads the setting name as a whole number of 1 or more.
func positive(name, def string) (int, error) {
	v := Get(name, def)
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a whole number of 1 or more: %q", name, v)
	}
	return n, nil
}

// WorkerConcurrency is HORUS_WORKER_CONCURRENCY: the slots of the worker,
// the count of jobs that it runs at the same time. Default 1.
func WorkerConcurrency() (int, error) { return positive("HORUS_WORKER_CONCURRENCY", "1") }

// JobRetentionDays is HORUS_JOB_RETENTION_DAYS: the age in days at which the
// worker deletes an ended job and its events. Default 30.
func JobRetentionDays() (int, error) { return positive("HORUS_JOB_RETENTION_DAYS", "30") }

// APIPort is HORUS_API_PORT: the port of horus api (/mcp, the Horus UI and
// /readyz), on 127.0.0.1. Default 8190. In the stack, horus api listens on
// 8190 in its container, and compose.yaml publishes HORUS_API_PORT of the
// host on 127.0.0.1.
func APIPort() (int, error) {
	n, err := positive("HORUS_API_PORT", "8190")
	if err == nil && n > 65535 {
		return 0, fmt.Errorf("HORUS_API_PORT must be a port from 1 to 65535: %d", n)
	}
	return n, err
}

// Stack is HORUS_STACK: the name of the Docker stack. The names of its
// containers are <stack>-api, <stack>-worker and <stack>-db. Default horus.
func Stack() string { return strings.TrimSpace(Get("HORUS_STACK", "horus")) }

// QueueMax is HORUS_QUEUE_MAX: the count of queued jobs at which a new call
// of the stack gets QUEUE_FULL. Default 20.
func QueueMax() (int, error) { return positive("HORUS_QUEUE_MAX", "20") }

// QueueTimeout is HORUS_QUEUE_TIMEOUT: the longest wait of a job in the
// queue of the stack, in seconds. Default 7200.
func QueueTimeout() (int, error) { return positive("HORUS_QUEUE_TIMEOUT", "7200") }

// RunEstimate is HORUS_RUN_ESTIMATE: the run time of one job, in seconds,
// that the estimated start of a queued job uses until a job succeeded. After
// that, the mean run time of the last succeeded jobs counts. Default 240.
func RunEstimate() (int, error) { return positive("HORUS_RUN_ESTIMATE", "240") }

// SharedDir is one pair of HORUS_SHARED_DIRS: a folder of the host and the
// folder in the containers of the stack where Docker mounts it.
type SharedDir struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

// SharedPrefix is the folder of the containers under which each container
// folder of HORUS_SHARED_DIRS must be.
const SharedPrefix = "/shared/"

// SharedDirs is HORUS_SHARED_DIRS: the pairs "<host folder>=<container
// folder>", with ";" between two pairs, for example
// C:\Users\you\Pictures\horus=/shared/pictures. The last "=" of a pair ends the
// host folder. Each container folder is a folder under /shared/, and two pairs
// never share one. Empty: no pair.
func SharedDirs() ([]SharedDir, error) { return ParseSharedDirs(Get("HORUS_SHARED_DIRS", "")) }

// ParseSharedDirs reads the value of HORUS_SHARED_DIRS.
func ParseSharedDirs(v string) ([]SharedDir, error) {
	out := []SharedDir{}
	seen := map[string]bool{}
	for _, part := range strings.Split(v, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.LastIndex(part, "=")
		if i <= 0 {
			return nil, fmt.Errorf("HORUS_SHARED_DIRS: %q is not a pair <host folder>=<container folder>", part)
		}
		d := SharedDir{Host: strings.TrimSpace(part[:i]), Container: strings.TrimRight(strings.TrimSpace(part[i+1:]), "/")}
		name := strings.TrimPrefix(d.Container, SharedPrefix)
		if !strings.HasPrefix(d.Container, SharedPrefix) || name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			return nil, fmt.Errorf("HORUS_SHARED_DIRS: the container folder %q of %q must be one folder under %s, for example %spictures",
				d.Container, part, SharedPrefix, SharedPrefix)
		}
		if seen[d.Container] {
			return nil, fmt.Errorf("HORUS_SHARED_DIRS: two pairs have the container folder %s", d.Container)
		}
		seen[d.Container] = true
		out = append(out, d)
	}
	return out, nil
}

// FormatSharedDirs is the value of HORUS_SHARED_DIRS for the pairs.
func FormatSharedDirs(dirs []SharedDir) string {
	parts := make([]string, len(dirs))
	for i, d := range dirs {
		parts[i] = d.Host + "=" + d.Container
	}
	return strings.Join(parts, ";")
}

// HostOS is HORUS_HOST_OS: the operating system of the host of the stack
// (windows, darwin or linux), as horus stack-mounts writes it. It gives the
// rules of a host path: a Windows or macOS path is the same path in any case.
// Empty: not known.
func HostOS() string { return strings.ToLower(strings.TrimSpace(Get("HORUS_HOST_OS", ""))) }
