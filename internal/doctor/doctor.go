// Package doctor is the service of horus doctor (also horus check-setup):
// one health check for the whole horus setup.
//
// Checks, in this order:
//  1. ComfyUI is installed (the backend says so).
//  2. The ComfyUI server answers.
//  3. comfy-mcp is found: in the venv of install-comfy-mcp, or on the PATH.
//  4. comfy-cli (the program "comfy") is found: where COMFY_BIN of the
//     comfy-mcp entry points, or else next to comfy-mcp.
//  5. The comfy-mcp entry of .mcp.json points at programs that exist: its
//     command, and COMFY_BIN when the entry sets it.
//  6. horus is on the PATH.
//  7. The horus entry of .mcp.json points at a program that exists.
//  8. macOS only: a path of the setup is inside a TCC folder (a folder that
//     macOS protects, platform.InTCCFolder). This check gives one result of
//     level Warn for each such path, and nothing on another OS.
//
// The model check of horus doctor -workflow is not here: it is the check of
// the setup service, and the handler runs it after these checks.
//
// The service changes nothing. It does its input and output through the
// interfaces below, and it reads files (.mcp.json, the venv) itself. It
// returns one typed Result for each check, with a Level; a handler writes
// the lines.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// Server is the running ComfyUI. cmd/horus gives the service a
// *comfy.Client.
type Server interface {
	// Version gives the version of the server. An error means no answer.
	Version(ctx context.Context) (string, error)
}

// Install is the ComfyUI install. cmd/horus gives the service the backend
// of internal/backend, the one place that knows the layout of the install.
type Install interface {
	// Installed gives the install folder, and reports if ComfyUI is
	// installed there.
	Installed() (dir string, ok bool)
}

// Programs finds the programs of this machine. cmd/horus gives the service
// platform.Programs.
type Programs interface {
	LookPath(name string) (string, error)
}

// Options are the facts of one run that are not an interface.
type Options struct {
	// ServerURL is the address of the server, for the text of the check.
	ServerURL string
	// VenvPath is the venv of install-comfy-mcp.
	VenvPath string
	// RepoRoot holds .mcp.json.
	RepoRoot string
	// Home replaces ${USERPROFILE} and ${HOME} in .mcp.json when they are
	// not set.
	Home string
	// Getenv reads the variables of ${VAR} and ${VAR:-default} in .mcp.json.
	// nil means os.Getenv.
	Getenv func(string) string

	// ModelsDir, TextEncoderDir and OutputDir are folders of the setup for
	// the TCC check, with RepoRoot and the install folder. Empty means not
	// checked.
	ModelsDir, TextEncoderDir, OutputDir string
	// InTCCFolder tells if an absolute path is inside a TCC folder, and
	// gives that folder. nil means platform.InTCCFolder in Home.
	InTCCFolder func(path string) (folder string, ok bool)
}

// Level is the level of a result.
type Level int

// The levels. A Warn is not a failed check. The zero value is Fail, so a
// result that does not set its level does not pass.
const (
	Fail Level = iota
	Pass
	Warn
)

// Result is one check.
type Result struct {
	Name   string
	Level  Level
	Detail string
}

// Service runs the checks.
type Service struct {
	server   Server
	install  Install
	programs Programs
}

// New makes the service.
func New(server Server, install Install, programs Programs) *Service {
	return &Service{server: server, install: install, programs: programs}
}

// Failed counts the results of level Fail. A Warn does not count.
func Failed(rs []Result) int {
	n := 0
	for _, r := range rs {
		if r.Level == Fail {
			n++
		}
	}
	return n
}

// Check runs the checks and returns one result for each, in order.
func (s *Service) Check(ctx context.Context, o Options) []Result {
	var rs []Result
	add := func(ok bool, name, detail string) {
		level := Fail
		if ok {
			level = Pass
		}
		rs = append(rs, Result{Name: name, Level: level, Detail: detail})
	}
	look := s.programs.LookPath

	// 1. ComfyUI installed.
	dir, installed := s.install.Installed()
	if installed {
		add(true, "ComfyUI folder", dir)
	} else {
		add(false, "ComfyUI folder", fmt.Sprintf("%s not found in %s. Fix: horus install-comfyui, or pass -comfyui-dir.",
			filepath.Join("ComfyUI", "main.py"), dir))
	}

	// 2. Server answers.
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	version, err := s.server.Version(sctx)
	cancel()
	if err == nil {
		add(true, "ComfyUI server", fmt.Sprintf("answers at %s (ComfyUI %s)", o.ServerURL, version))
	} else {
		add(false, "ComfyUI server", fmt.Sprintf("no answer at %s. Fix: horus start-comfyui", o.ServerURL))
	}

	// 3. comfy-mcp found.
	mcpProg := ""
	venvMCP := platform.VenvBin(o.VenvPath, "comfy-mcp")
	fromVenv := platform.IsFile(venvMCP)
	if fromVenv {
		mcpProg = venvMCP
		add(true, "comfy-mcp environment", venvMCP)
	} else if p, err := look("comfy-mcp"); err == nil {
		mcpProg = p
		add(true, "comfy-mcp environment", p)
	} else {
		add(false, "comfy-mcp environment", fmt.Sprintf("comfy-mcp not found in %s or on the PATH. Fix: horus install-comfy-mcp", filepath.Dir(venvMCP)))
	}

	// 4 and 7 read .mcp.json.
	mcpJSON := filepath.Join(o.RepoRoot, ".mcp.json")
	getenv := o.envFunc()
	entry := func(name string) (mcpEntry, error) { return readEntry(mcpJSON, name, getenv) }
	comfyEntry, comfyErr := entry("comfy-mcp")

	// 4. comfy-cli found: COMFY_BIN of the entry, else next to comfy-mcp.
	// The comfy MCP server starts comfy-cli, so a missing program fails there.
	comfyName := platform.ExeName("comfy")
	switch {
	case comfyErr == nil && comfyEntry.ComfyBin != "":
		if p, ok := resolve(comfyEntry.ComfyBin, look); ok {
			add(true, "comfy-cli", p)
		} else {
			add(false, "comfy-cli", fmt.Sprintf("COMFY_BIN of .mcp.json is '%s' and does not exist. Fix: horus install-comfy-mcp, or edit .mcp.json.", comfyEntry.ComfyBin))
		}
	case mcpProg == "":
		add(false, "comfy-cli", "comfy-mcp is missing, so comfy-cli cannot be found. Fix: horus install-comfy-mcp")
	default:
		next := filepath.Join(filepath.Dir(mcpProg), comfyName)
		if platform.IsFile(next) {
			add(true, "comfy-cli", next)
		} else if p, err := look("comfy"); err == nil && !fromVenv {
			add(true, "comfy-cli", p)
		} else {
			add(false, "comfy-cli", fmt.Sprintf("%s not found next to comfy-mcp. Fix: horus install-comfy-mcp", comfyName))
		}
	}

	// 5. The comfy-mcp entry of .mcp.json.
	if comfyErr != nil {
		add(false, ".mcp.json", fmt.Sprintf("cannot read comfy-mcp entry in %s : %v", mcpJSON, comfyErr))
	} else if p, ok := resolve(comfyEntry.Command, look); !ok {
		add(false, ".mcp.json", fmt.Sprintf("command is '%s' and does not exist. Fix: horus install-comfy-mcp, or edit .mcp.json.", comfyEntry.Command))
	} else {
		detail := "command points at " + p
		if comfyEntry.ComfyBin != "" {
			detail += ", COMFY_BIN at " + comfyEntry.ComfyBin
		}
		add(true, ".mcp.json", detail)
	}

	// 6. horus is on the PATH.
	if p, err := look("horus"); err == nil {
		add(true, "horus on PATH", p)
	} else {
		add(false, "horus on PATH", "horus not found on the PATH. Fix: put the folder of the horus program on the PATH.")
	}

	// 7. The horus entry of .mcp.json.
	if e, err := entry("horus"); err != nil {
		add(false, ".mcp.json horus", fmt.Sprintf("cannot read horus entry in %s : %v", mcpJSON, err))
	} else if p, ok := resolve(e.Command, look); ok {
		add(true, ".mcp.json horus", "command points at "+p)
	} else {
		add(false, ".mcp.json horus", fmt.Sprintf("command is '%s' and does not exist. Fix: put horus on the PATH, or edit .mcp.json.", e.Command))
	}

	// 8. The TCC check.
	return append(rs, o.tccWarnings(dir)...)
}

// tccWarnings gives one Warn for each path of the setup that is inside a
// TCC folder: the repo root, the install folder dir, the models folder, the
// text encoder folder and the output folder. The line names the absolute
// path. A path that two settings share gives one line.
func (o Options) tccWarnings(dir string) []Result {
	in := o.InTCCFolder
	if in == nil {
		in = func(p string) (string, bool) { return platform.InTCCFolder(o.Home, p) }
	}
	paths := []struct{ what, path string }{
		{"the repo root", o.RepoRoot},
		{"the install folder", dir},
		{"the models folder", o.ModelsDir},
		{"the text encoder folder", o.TextEncoderDir},
		{"the output folder", o.OutputDir},
	}
	var rs []Result
	seen := map[string]bool{}
	for _, p := range paths {
		if p.path == "" {
			continue
		}
		abs, err := filepath.Abs(p.path)
		if err != nil {
			abs = p.path
		}
		folder, ok := in(abs)
		if !ok || seen[abs] {
			continue
		}
		seen[abs] = true
		rs = append(rs, Result{Name: "macOS privacy folder", Level: Warn, Detail: fmt.Sprintf(
			"%s %s is inside %s, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. "+
				"Fix: move it out of %s, or grant access in System Settings > Privacy & Security.", p.what, abs, folder, folder)})
	}
	return rs
}

// mcpEntry is the part of one .mcp.json server that the check reads, with
// ${VAR} and ${VAR:-default} expanded.
type mcpEntry struct {
	Command  string
	ComfyBin string // env.COMFY_BIN, empty when the entry does not set it
}

func (o Options) envFunc() func(string) string {
	get := o.Getenv
	if get == nil {
		get = os.Getenv
	}
	return func(name string) string {
		if v := get(name); v != "" {
			return v
		}
		if name == "USERPROFILE" || name == "HOME" {
			return o.Home
		}
		return ""
	}
}

// expand replaces ${VAR} and ${VAR:-default}. An unset or empty VAR gives its
// default, or the empty string.
func expand(s string, getenv func(string) string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "${")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.Index(s[i:], "}")
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		name, def, hasDef := strings.Cut(s[i+2:i+j], ":-")
		v := getenv(name)
		if v == "" && hasDef {
			v = def
		}
		b.WriteString(v)
		s = s[i+j+1:]
	}
}

// readEntry returns the command and COMFY_BIN of one server in .mcp.json.
func readEntry(path, name string, getenv func(string) string) (mcpEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mcpEntry{}, err
	}
	var cfg struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return mcpEntry{}, err
	}
	s, ok := cfg.Servers[name]
	if !ok {
		return mcpEntry{}, fmt.Errorf("no %s entry", name)
	}
	return mcpEntry{Command: expand(s.Command, getenv), ComfyBin: expand(s.Env["COMFY_BIN"], getenv)}, nil
}

// resolve finds the program of a command: a path must be a file, a bare name
// must be on the PATH.
func resolve(cmd string, look func(string) (string, error)) (string, bool) {
	if cmd == "" {
		return "", false
	}
	if strings.ContainsAny(cmd, `/\`) {
		return cmd, platform.IsFile(cmd)
	}
	p, err := look(cmd)
	return p, err == nil
}
