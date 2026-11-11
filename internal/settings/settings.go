// Package settings holds the settings of horus.
//
// Each setting comes from, in this order: the environment variable, the .env
// file in the repo root, the default. An empty value counts as not set. The
// .env file can hold HF_TOKEN. This package never returns or logs that token:
// only the download code reads it.
//
// The repo root (the rule of the spec): HORUS_ROOT, then the nearest folder at
// or above the current folder that has a workflows/ folder, then the folder of
// the executable.
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
)

var (
	mu       sync.Mutex
	root     string
	envCache map[string]string
)

// Reset forgets the cached repo root and .env values. Tests use it.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	root, envCache = "", nil
}

// Root is the repo root.
func Root() string {
	mu.Lock()
	defer mu.Unlock()
	return rootLocked()
}

func rootLocked() string {
	if root == "" {
		root = findRoot()
	}
	return root
}

func findRoot() string {
	if r := os.Getenv("HORUS_ROOT"); r != "" {
		return r
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			if fi, err := os.Stat(filepath.Join(dir, "workflows")); err == nil && fi.IsDir() {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		return filepath.Dir(exe)
	}
	return "."
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
		if data, err := os.ReadFile(filepath.Join(rootLocked(), ".env")); err == nil {
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

// ComfyDir is the ComfyUI portable folder.
func ComfyDir() string {
	home, _ := os.UserHomeDir()
	return Get("HORUS_COMFYUI_DIR", filepath.Join(home, "ComfyUI_windows_portable"))
}

// WorkflowsDir is the folder of the workflows.
func WorkflowsDir() string {
	return Get("HORUS_WORKFLOWS_DIR", filepath.Join(Root(), "workflows"))
}

// OutputDir is where a PNG goes when a call has no out_path.
func OutputDir() string {
	return Get("HORUS_OUTPUT_DIR", filepath.Join(Root(), ".scratch", "outputs"))
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
