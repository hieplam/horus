// Package checksetup is one health check for the whole horus setup. It ports
// scripts/check-setup.ps1.
//
// It prints one PASS or FAIL line for each check. The exit code is 0 only if
// all checks pass.
//
// Checks:
//  1. ComfyUI is installed in -comfyui-dir.
//  2. The ComfyUI server answers at -server-url.
//  3. comfy-mcp is found: in the venv of install-comfy-mcp, or on the PATH.
//  4. comfy-cli (the program "comfy") is found: where COMFY_BIN of the
//     comfy-mcp entry points, or else next to comfy-mcp.
//  5. The comfy-mcp entry of .mcp.json points at programs that exist: its
//     command, and COMFY_BIN when the entry sets it.
//  6. horus is on the PATH.
//  7. The horus entry of .mcp.json points at a program that exists.
//  8. With -workflow: the model files of that workflow are present. Only with
//     -full are the SHA-256 hashes checked, and only then does the line say
//     "verified".
//
// The script checked the two venvs of the Python era. Checks 3 and 6 are now
// "found" checks, as the port map (issue #4) says.
package checksetup

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/settings"
)

// ModelChecker checks the model files of one workflow. The real one starts
// the subcommand "horus workflow check" of the same binary, so this package
// holds no copy of the model-file rules (they live in internal/scripts/workflow).
type ModelChecker interface {
	// Check returns the text lines of the check and ok when the check passed.
	Check(ctx context.Context, workflow string) (lines []string, ok bool, err error)
}

// Options is everything that Check needs.
type Options struct {
	ComfyUIDir string
	ServerURL  string
	// VenvPath is the venv of install-comfy-mcp.
	VenvPath string
	// RepoRoot holds .mcp.json.
	RepoRoot string
	// Home replaces ${USERPROFILE} in .mcp.json.
	Home string
	// Workflow is the workflow whose model files are checked. Empty: no check.
	Workflow string
	Models   ModelChecker
	// FullModels is true when Models checks the SHA-256 hashes (-full).
	FullModels bool
	// Getenv reads the variables of ${VAR} and ${VAR:-default} in .mcp.json.
	// nil means os.Getenv. USERPROFILE (and HOME) fall back to Home.
	Getenv func(string) string
	// LookPath finds a program on the PATH. nil means exec.LookPath.
	LookPath func(string) (string, error)
	Stdout   io.Writer
}

var goos = runtime.GOOS

func exe(name string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

func venvBin(venv string) string {
	if goos == "windows" {
		return filepath.Join(venv, "Scripts")
	}
	return filepath.Join(venv, "bin")
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

type checker struct {
	out    io.Writer
	failed int
}

func (c *checker) write(ok bool, name, detail string) {
	if ok {
		fmt.Fprintf(c.out, "PASS  %s - %s\n", name, detail)
		return
	}
	fmt.Fprintf(c.out, "FAIL  %s - %s\n", name, detail)
	c.failed++
}

// Check runs the checks and returns the number of checks that failed.
func Check(ctx context.Context, o Options) int {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	look := o.LookPath
	if look == nil {
		look = exec.LookPath
	}
	c := &checker{out: out}

	// 1. ComfyUI installed.
	if isFile(filepath.Join(o.ComfyUIDir, "ComfyUI", "main.py")) {
		c.write(true, "ComfyUI folder", o.ComfyUIDir)
	} else {
		c.write(false, "ComfyUI folder", fmt.Sprintf("%s not found in %s. Fix: horus install-comfyui, or pass -comfyui-dir.",
			filepath.Join("ComfyUI", "main.py"), o.ComfyUIDir))
	}

	// 2. Server answers.
	if version, err := systemStats(ctx, o.ServerURL); err == nil {
		c.write(true, "ComfyUI server", fmt.Sprintf("answers at %s (ComfyUI %s)", o.ServerURL, version))
	} else {
		c.write(false, "ComfyUI server", fmt.Sprintf("no answer at %s. Fix: horus start-comfyui", o.ServerURL))
	}

	// 3. comfy-mcp found.
	mcpProg := ""
	venvMCP := filepath.Join(venvBin(o.VenvPath), exe("comfy-mcp"))
	fromVenv := isFile(venvMCP)
	if fromVenv {
		mcpProg = venvMCP
		c.write(true, "comfy-mcp environment", venvMCP)
	} else if p, err := look("comfy-mcp"); err == nil {
		mcpProg = p
		c.write(true, "comfy-mcp environment", p)
	} else {
		c.write(false, "comfy-mcp environment", fmt.Sprintf("comfy-mcp not found in %s or on the PATH. Fix: horus install-comfy-mcp", venvBin(o.VenvPath)))
	}

	// 4 and 7 read .mcp.json.
	mcpJSON := filepath.Join(o.RepoRoot, ".mcp.json")
	getenv := o.envFunc()
	entry := func(name string) (mcpEntry, error) { return readEntry(mcpJSON, name, getenv) }
	comfyEntry, comfyErr := entry("comfy-mcp")

	// 4. comfy-cli found: COMFY_BIN of the entry, else next to comfy-mcp.
	// The comfy MCP server starts comfy-cli, so a missing program fails there.
	switch {
	case comfyErr == nil && comfyEntry.ComfyBin != "":
		if p, ok := resolve(comfyEntry.ComfyBin, look); ok {
			c.write(true, "comfy-cli", p)
		} else {
			c.write(false, "comfy-cli", fmt.Sprintf("COMFY_BIN of .mcp.json is '%s' and does not exist. Fix: horus install-comfy-mcp, or edit .mcp.json.", comfyEntry.ComfyBin))
		}
	case mcpProg == "":
		c.write(false, "comfy-cli", "comfy-mcp is missing, so comfy-cli cannot be found. Fix: horus install-comfy-mcp")
	default:
		next := filepath.Join(filepath.Dir(mcpProg), exe("comfy"))
		if isFile(next) {
			c.write(true, "comfy-cli", next)
		} else if p, err := look("comfy"); err == nil && !fromVenv {
			c.write(true, "comfy-cli", p)
		} else {
			c.write(false, "comfy-cli", fmt.Sprintf("%s not found next to comfy-mcp. Fix: horus install-comfy-mcp", exe("comfy")))
		}
	}

	// 5. The comfy-mcp entry of .mcp.json.
	if comfyErr != nil {
		c.write(false, ".mcp.json", fmt.Sprintf("cannot read comfy-mcp entry in %s : %v", mcpJSON, comfyErr))
	} else if p, ok := resolve(comfyEntry.Command, look); !ok {
		c.write(false, ".mcp.json", fmt.Sprintf("command is '%s' and does not exist. Fix: horus install-comfy-mcp, or edit .mcp.json.", comfyEntry.Command))
	} else {
		detail := "command points at " + p
		if comfyEntry.ComfyBin != "" {
			detail += ", COMFY_BIN at " + comfyEntry.ComfyBin
		}
		c.write(true, ".mcp.json", detail)
	}

	// 6. horus is on the PATH.
	if p, err := look("horus"); err == nil {
		c.write(true, "horus on PATH", p)
	} else {
		c.write(false, "horus on PATH", "horus not found on the PATH. Fix: put the folder of the horus program on the PATH.")
	}

	// 7. The horus entry of .mcp.json.
	if e, err := entry("horus"); err != nil {
		c.write(false, ".mcp.json horus", fmt.Sprintf("cannot read horus entry in %s : %v", mcpJSON, err))
	} else if p, ok := resolve(e.Command, look); ok {
		c.write(true, ".mcp.json horus", "command points at "+p)
	} else {
		c.write(false, ".mcp.json horus", fmt.Sprintf("command is '%s' and does not exist. Fix: put horus on the PATH, or edit .mcp.json.", e.Command))
	}

	// 8. Models of one workflow.
	if o.Workflow != "" {
		lines, ok, err := o.Models.Check(ctx, o.Workflow)
		for _, l := range lines {
			fmt.Fprintf(out, "      %s\n", l)
		}
		name := fmt.Sprintf("models for '%s'", o.Workflow)
		switch {
		case err != nil:
			c.write(false, name, "cannot run the model check: "+err.Error())
		case ok:
			if o.FullModels {
				c.write(true, name, "all files present and verified (SHA-256 checked)")
			} else {
				c.write(true, name, "all files present, size checked (the SHA-256 is not checked; add -full)")
			}
		default:
			c.write(false, name, "see lines above. Fix: horus workflow install "+o.Workflow)
		}
	}

	if c.failed > 0 {
		fmt.Fprintf(out, "%d check(s) failed.\n", c.failed)
	} else {
		fmt.Fprintln(out, "All checks passed.")
	}
	return c.failed
}

func systemStats(ctx context.Context, base string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/system_stats", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var v struct {
		System struct {
			Version string `json:"comfyui_version"`
		} `json:"system"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.System.Version, nil
}

// mcpEntry is the part of one .mcp.json server that the check reads, with
// ${VAR} and ${VAR:-default} expanded.
type mcpEntry struct {
	Command  string
	ComfyBin string // env.COMFY_BIN, empty when the entry does not set it
}

func (o Options) envFunc() func(string) string {
	return func(name string) string {
		if o.Getenv != nil {
			if v := o.Getenv(name); v != "" {
				return v
			}
		} else if v := os.Getenv(name); v != "" {
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

// ExecModels is the real ModelChecker. It runs
// "<Exe> workflow check <name> -quick -json" with the folders in the
// environment, and reads the text lines (it drops the HORUS_JSON line).
type ExecModels struct {
	Exe        string
	Full       bool // true: no -quick, so the hashes are checked
	ComfyUIDir string
	ModelsDir  string
	TextDir    string
}

// Check runs the subcommand.
func (m ExecModels) Check(ctx context.Context, workflow string) ([]string, bool, error) {
	args := []string{"workflow", "check", workflow}
	if !m.Full {
		args = append(args, "-quick")
	}
	args = append(args, "-json")
	cmd := exec.CommandContext(ctx, m.Exe, args...)
	cmd.Env = os.Environ()
	for k, v := range map[string]string{"HORUS_COMFYUI_DIR": m.ComfyUIDir, "HORUS_MODELS_DIR": m.ModelsDir, "HORUS_TEXT_ENCODER_DIR": m.TextDir} {
		if v != "" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	var lines []string
	sc := bufio.NewScanner(&buf)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if l := strings.TrimRight(sc.Text(), "\r"); !strings.HasPrefix(l, "HORUS_JSON ") {
			lines = append(lines, l)
		}
	}
	if err == nil {
		return lines, true, nil
	}
	if _, isExit := err.(*exec.ExitError); isExit {
		return lines, false, nil
	}
	return nil, false, err
}

// Run is the command horus check-setup. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus check-setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("comfyui-dir", "", "the portable folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	serverURL := fs.String("server-url", "http://127.0.0.1:8188", "address of the ComfyUI server")
	venv := fs.String("venv-path", "", "folder of the comfy-mcp venv (default: <home>/.venvs/comfy-mcp)")
	workflow := fs.String("workflow", "", "also check the model files of workflows/<name>/models.json")
	full := fs.Bool("full", false, "with -workflow: also check the SHA-256 of each file (the default checks the size only; hashing 11 GB takes minutes)")
	modelsDir := fs.String("models-dir", "", "with -workflow: models folder (default: HORUS_MODELS_DIR)")
	textDir := fs.String("text-encoder-dir", "", "with -workflow: folder of the text encoder on another drive (default: HORUS_TEXT_ENCODER_DIR)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	home, _ := os.UserHomeDir()
	o := Options{ComfyUIDir: *dir, ServerURL: *serverURL, VenvPath: *venv, RepoRoot: settings.Root(), Home: home,
		Workflow: *workflow, FullModels: *full, Stdout: stdout}
	if o.ComfyUIDir == "" {
		o.ComfyUIDir = settings.ComfyDir()
	}
	if o.VenvPath == "" {
		o.VenvPath = filepath.Join(home, ".venvs", "comfy-mcp")
	}
	if o.Workflow != "" {
		self, err := os.Executable()
		if err != nil {
			fmt.Fprintln(stderr, "horus check-setup:", err)
			return 1
		}
		o.Models = ExecModels{Exe: self, Full: *full, ComfyUIDir: o.ComfyUIDir, ModelsDir: *modelsDir, TextDir: *textDir}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if Check(ctx, o) > 0 {
		return 1
	}
	return 0
}
