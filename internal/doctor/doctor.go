// Package doctor is the service of horus doctor (also horus check-setup):
// one health check for the whole horus setup. It ports
// scripts/check-setup.ps1.
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
//  8. With a workflow: the model files of that workflow are present, and by
//     default their SHA-256 hashes are checked. Quick checks the sizes only.
//
// The service changes nothing. It does its input and output through the
// interfaces below, and it reads files (.mcp.json, the venv) itself. It
// returns one typed Result for each check; a handler writes the lines.
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

// Models checks the model files of one workflow. cmd/horus gives the service
// the check of horus workflow.
type Models interface {
	// Check returns the text lines of the check and ok when the check
	// passed. quick: the size of each file only, no SHA-256.
	Check(ctx context.Context, workflow string, quick bool) (lines []string, ok bool, err error)
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
	// Workflow is the workflow whose model files are checked. Empty: no check.
	Workflow string
	// Quick: the model check compares the sizes only.
	Quick bool
	// Getenv reads the variables of ${VAR} and ${VAR:-default} in .mcp.json.
	// nil means os.Getenv.
	Getenv func(string) string
}

// Result is one check.
type Result struct {
	Name   string
	OK     bool
	Detail string
	// Lines are the lines of the check that come before its result line
	// (the model check).
	Lines []string
}

// Service runs the checks.
type Service struct {
	server   Server
	install  Install
	programs Programs
	models   Models
}

// New makes the service. models can be nil when no run names a workflow.
func New(server Server, install Install, programs Programs, models Models) *Service {
	return &Service{server: server, install: install, programs: programs, models: models}
}

// Failed counts the results that did not pass.
func Failed(rs []Result) int {
	n := 0
	for _, r := range rs {
		if !r.OK {
			n++
		}
	}
	return n
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// Check runs the checks and returns one result for each, in order.
func (s *Service) Check(ctx context.Context, o Options) []Result {
	var rs []Result
	add := func(ok bool, name, detail string) { rs = append(rs, Result{Name: name, OK: ok, Detail: detail}) }
	look := s.programs.LookPath

	// 1. ComfyUI installed.
	if dir, ok := s.install.Installed(); ok {
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
	fromVenv := isFile(venvMCP)
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
		if isFile(next) {
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

	// 8. Models of one workflow.
	if o.Workflow != "" {
		r := Result{Name: fmt.Sprintf("models for '%s'", o.Workflow)}
		var ok bool
		var err error
		if s.models == nil {
			err = fmt.Errorf("no model check")
		} else {
			r.Lines, ok, err = s.models.Check(ctx, o.Workflow, o.Quick)
		}
		switch {
		case err != nil:
			r.Detail = "cannot run the model check: " + err.Error()
		case ok && o.Quick:
			r.OK, r.Detail = true, "all files present, size checked (the SHA-256 is not checked; drop -quick)"
		case ok:
			r.OK, r.Detail = true, "all files present and verified (SHA-256 checked)"
		default:
			r.Detail = "see lines above. Fix: horus workflow install " + o.Workflow
		}
		rs = append(rs, r)
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
		return cmd, isFile(cmd)
	}
	p, err := look(cmd)
	return p, err == nil
}
