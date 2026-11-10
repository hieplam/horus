// Package workflow lists the horus workflows, and installs, checks or removes
// the model files of one. It ports scripts/workflow.ps1.
//
// Actions:
//
//	list                 every workflow with its description, its model files and
//	                     whether they are on disk (size check)
//	check   <workflow>   is this machine ready to run the workflow: every file of
//	                     models.json on disk with the right size and SHA-256 (-quick:
//	                     size only), models.json lists every model file that
//	                     workflow.json loads, and (when ComfyUI answers) the server
//	                     has every node class and lists every model file
//	install <workflow>   download and verify the files of models.json, then check
//	remove  <workflow>   delete the model files; without -yes only print the plan
//
// Where a model file of folder F is looked for, in this order: -models-dir/F
// (or HORUS_MODELS_DIR/F; for F = text_encoders, -text-encoder-dir or
// HORUS_TEXT_ENCODER_DIR comes first), <ComfyUI dir>/ComfyUI/models/F, then
// every folder that ComfyUI's extra_model_paths.yaml maps to F. A new download
// goes to the first folder. When that folder is outside ComfyUI's own models
// folder, install adds it to extra_model_paths.yaml, with a backup first.
//
// With -json, check, install and remove print one last line
// "HORUS_JSON {...}": ok, action, workflow, message and one item per check or
// file (status, kind, role, file, path, bytes, detail). horus mcp reads it.
// list -json prints a JSON list instead of the table.
//
// Exit code 0 = the action succeeded, 1 = anything else, 2 = a flag that does
// not parse.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/scripts/downloadmodel"
	"github.com/hieplam/horus/internal/settings"
)

// Options holds the test seams. The zero value is the real behaviour.
type Options struct {
	// Download downloads one file. Default: downloadmodel.Download.
	Download func(ctx context.Context, o downloadmodel.Options) (downloadmodel.Result, error)
	// HTTPClient asks ComfyUI for /object_info. Default: a client with a 30 s limit.
	HTTPClient *http.Client
	// Now gives the time in the name of the backup of extra_model_paths.yaml.
	Now func() time.Time
}

// Item is one check or one file of the result.
type Item struct {
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	File   string `json:"file"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Detail string `json:"detail"`
}

// Result is the HORUS_JSON line.
type Result struct {
	OK       bool   `json:"ok"`
	Action   string `json:"action"`
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
	Items    []Item `json:"items"`
}

// Mark starts the result line.
const Mark = "HORUS_JSON "

type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

type runner struct {
	ctx context.Context
	out io.Writer
	opt Options

	action, workflow                     string
	only                                 []string
	yes, force, quick, json              bool
	comfyUIDir, modelsDir, teDir, server string
	modelsJSON, workflowsDir             string
	comfyDir, defaultModels              string

	yaml          map[string][]string
	items         []Item
	checkFailures int
}

// failure is a message after "FAIL: ".
type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func failf(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

func (r *runner) say(format string, a ...any) { fmt.Fprintf(r.out, format+"\n", a...) }

func (r *runner) add(status, kind, role, file, path string, bytes int64, detail string) {
	r.items = append(r.items, Item{status, kind, role, file, path, bytes, detail})
}

func (r *runner) writeResult(ok bool, msg string) {
	if !r.json || r.action == "list" {
		return
	}
	items := r.items
	if items == nil {
		items = []Item{}
	}
	b, _ := json.Marshal(Result{ok, r.action, r.workflow, msg, items})
	fmt.Fprintln(r.out, Mark+string(b))
}

// Main runs horus workflow with args (the arguments after the command name),
// writes its lines to out and returns the exit code.
func Main(ctx context.Context, args []string, out io.Writer, opt Options) int {
	r := &runner{ctx: ctx, out: out, opt: opt}
	fs := flag.NewFlagSet("horus workflow", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	only := listFlag{}
	fs.Var(&only, "only", "limit install, check and remove to these roles or file names (comma separated, or the flag again)")
	fs.BoolVar(&r.yes, "yes", false, "remove: delete the files; without it, remove only prints the plan")
	fs.BoolVar(&r.force, "force", false, "remove: also delete a file that another workflow lists")
	fs.BoolVar(&r.quick, "quick", false, "check and list: compare the size only, no SHA-256")
	fs.BoolVar(&r.json, "json", false, "list: print JSON; check, install, remove: add the HORUS_JSON line")
	fs.StringVar(&r.comfyUIDir, "comfyui-dir", "", "the ComfyUI portable folder (default: HORUS_COMFYUI_DIR, then ~/ComfyUI_windows_portable)")
	fs.StringVar(&r.modelsDir, "models-dir", "", "the models folder (default: HORUS_MODELS_DIR, then <comfyui-dir>/ComfyUI/models)")
	fs.StringVar(&r.teDir, "text-encoder-dir", "", "the text encoder folder (default: HORUS_TEXT_ENCODER_DIR)")
	fs.StringVar(&r.server, "server-url", "", "the ComfyUI address (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	fs.StringVar(&r.modelsJSON, "models-json", "", "use this file instead of workflows/<workflow>/models.json")
	fs.StringVar(&r.workflowsDir, "workflows-dir", "", "use this folder instead of the repo's workflows folder")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus workflow list|check|install|remove [<workflow>] [flags]")
		fmt.Fprintln(w, "List the horus workflows, and install, check or remove the model files of one.")
		fmt.Fprintln(w, "Exit code 0 = the action succeeded. Exit code 1 = anything else.")
		fs.PrintDefaults()
	}
	// Flags may come before and after the action and the workflow name.
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	r.only = only
	if len(pos) == 0 || len(pos) > 2 {
		fs.Usage()
		return 2
	}
	r.action = strings.ToLower(pos[0])
	switch r.action {
	case "list", "check", "install", "remove":
	default:
		fmt.Fprintf(os.Stderr, "horus workflow: unknown action %q: use list, check, install or remove\n", pos[0])
		return 2
	}
	if len(pos) == 2 {
		r.workflow = pos[1]
	}
	if r.opt.Download == nil {
		r.opt.Download = downloadmodel.Download
	}
	if r.opt.HTTPClient == nil {
		r.opt.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if r.opt.Now == nil {
		r.opt.Now = time.Now
	}
	if err := r.run(); err != nil {
		r.say("FAIL: %v", err)
		r.writeResult(false, err.Error())
		return 1
	}
	return 0
}

var workflowName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (r *runner) run() error {
	// Settings and folders.
	if r.workflowsDir == "" {
		r.workflowsDir = filepath.Join(settings.Root(), "workflows")
	}
	if !isDir(r.workflowsDir) {
		return failf("No workflows folder: %s", r.workflowsDir)
	}
	if r.comfyUIDir == "" {
		r.comfyUIDir = settings.ComfyDir()
	}
	r.comfyDir = filepath.Join(r.comfyUIDir, "ComfyUI")
	r.defaultModels = filepath.Join(r.comfyDir, "models")
	if r.modelsDir == "" {
		r.modelsDir = settings.Get("HORUS_MODELS_DIR", r.defaultModels)
	}
	if r.teDir == "" {
		r.teDir = settings.Get("HORUS_TEXT_ENCODER_DIR", "")
	}
	if r.server == "" {
		r.server = settings.Get("HORUS_COMFYUI_URL", "http://127.0.0.1:8188")
	}
	r.server = strings.TrimRight(r.server, "/")
	r.yaml = yamlFolders(filepath.Join(r.comfyDir, "extra_model_paths.yaml"))

	if r.action == "list" {
		return r.list()
	}

	// The other actions need one workflow.
	if r.workflow == "" {
		return failf("Name a workflow. Run 'horus workflow list' to see them.")
	}
	if !workflowName.MatchString(r.workflow) {
		return failf("Invalid workflow name: %s", r.workflow)
	}
	wfDir := filepath.Join(r.workflowsDir, r.workflow)
	if r.modelsJSON == "" {
		if !isDir(wfDir) {
			ws, err := listWorkflows(r.workflowsDir)
			if err != nil {
				return err
			}
			names := make([]string, len(ws))
			for i, w := range ws {
				names[i] = w.Name
			}
			return failf("No workflow '%s'. Known: %s", r.workflow, strings.Join(names, ", "))
		}
		r.modelsJSON = filepath.Join(wfDir, "models.json")
	}
	if !isFile(r.modelsJSON) {
		return failf("Model list not found: %s", r.modelsJSON)
	}
	m, err := readManifest(r.modelsJSON)
	if err != nil {
		return err
	}
	graph := filepath.Join(wfDir, "workflow.json")

	switch r.action {
	case "check":
		if err := r.check(m, graph); err != nil {
			return err
		}
		if r.checkFailures > 0 {
			return failf("%d check(s) failed. Fix: horus workflow install %s", r.checkFailures, r.workflow)
		}
		r.say("OK: workflow '%s' is ready.", r.workflow)
		r.writeResult(true, fmt.Sprintf("workflow '%s' is ready", r.workflow))
		return nil
	case "install":
		return r.install(m, graph)
	default:
		return r.remove(m)
	}
}

// listRow is one row of list -json.
type listRow struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Files       int     `json:"files"`
	Present     int     `json:"present"`
	Bytes       int64   `json:"bytes"`
	Gigabytes   float64 `json:"gigabytes"`
	Installed   string  `json:"installed"`
	Roles       string  `json:"roles"`
}

func (r *runner) list() error {
	ws, err := listWorkflows(r.workflowsDir)
	if err != nil {
		return err
	}
	rows := []listRow{}
	for _, w := range ws {
		var files []modelFile
		present := 0
		if w.ModelsJSON != "" {
			m, err := readManifest(w.ModelsJSON)
			if err != nil {
				return err
			}
			files = m.Files
			for _, f := range files {
				if p := r.find(f); p != "" && fileSize(p) == f.Bytes {
					present++
				}
			}
		}
		var total int64
		roles := make([]string, len(files))
		for i, f := range files {
			total += f.Bytes
			roles[i] = f.Role
		}
		installed := fmt.Sprintf("no (%d of %d files)", present, len(files))
		switch {
		case len(files) == 0:
			installed = "no models"
		case present == len(files):
			installed = "yes"
		}
		rows = append(rows, listRow{w.Name, w.Description, len(files), present, total,
			roundGB(total), installed, strings.Join(roles, ", ")})
	}
	if r.json {
		b, _ := json.MarshalIndent(rows, "", "    ")
		fmt.Fprintln(r.out, string(b))
		return nil
	}
	if len(rows) == 0 {
		r.say("No workflows in %s", r.workflowsDir)
		return nil
	}
	for _, row := range rows {
		r.say("%s", row.Name)
		r.say("  %s", row.Description)
		r.say("  Models: %d file(s), %s GB (%s). Installed: %s (size check).", row.Files,
			strconv.FormatFloat(row.Gigabytes, 'f', -1, 64), row.Roles, row.Installed)
		r.say("")
	}
	r.say("Install: horus workflow install <name>    Check: horus workflow check <name>")
	return nil
}

// roundGB is [math]::Round($bytes / 1e9, 2).
func roundGB(b int64) float64 { return math.RoundToEven(float64(b)/1e9*100) / 100 }

// gb2 is "{0:N2}" -f ($bytes / 1e9) as Windows PowerShell 5.1 prints it in
// the en-US culture: .NET Framework takes the value with 15 significant digits,
// rounds it half away from zero to two decimals, and puts a comma between
// groups of three digits. So 999.995 gives "1,000.00" and 1.005 gives "1.01".
func gb2(b int64) string {
	e := strconv.FormatFloat(float64(b)/1e9, 'e', 14, 64) // d.dddddddddddddde+XX
	mant, expText, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expText)
	keep := exp + 3 // the digits of value*100
	var n uint64
	if keep > 0 {
		for len(digits) <= keep {
			digits += "0"
		}
		n, _ = strconv.ParseUint(digits[:keep], 10, 64)
		if digits[keep] >= '5' {
			n++
		}
	} else if keep == 0 && digits[0] >= '5' {
		n = 1
	}
	intPart, frac := strconv.FormatUint(n/100, 10), fmt.Sprintf("%02d", n%100)
	var g []byte
	for i, c := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			g = append(g, ',')
		}
		g = append(g, c)
	}
	return string(g) + "." + frac
}

func fileSize(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return -1
	}
	return fi.Size()
}

// selectFiles is Select-Files: the files whose role or name is in -only.
func (r *runner) selectFiles(files []modelFile) ([]modelFile, error) {
	if len(r.only) == 0 {
		return files, nil
	}
	var picked []modelFile
	for _, f := range files {
		for _, o := range r.only {
			if strings.EqualFold(o, f.Role) || strings.EqualFold(o, f.Name) {
				picked = append(picked, f)
				break
			}
		}
	}
	if len(picked) == 0 {
		var roles []string
		for _, f := range files {
			roles = append(roles, f.Role)
		}
		return nil, failf("-only %s matches no file. Roles: %s", strings.Join(r.only, ","), strings.Join(uniqueStrings(roles), ", "))
	}
	return picked, nil
}

// modelDirs is Get-ModelDirs: the folders for a model file of one folder, in order.
func (r *runner) modelDirs(folder string) []string {
	var dirs []string
	if folder == "text_encoders" && r.teDir != "" {
		dirs = append(dirs, r.teDir)
	}
	dirs = append(dirs, joinPath(r.modelsDir, folder), joinPath(r.defaultModels, folder))
	dirs = append(dirs, r.yaml[folder]...)
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		d = fullPath(d)
		if k := strings.ToLower(d); !seen[k] {
			seen[k] = true
			out = append(out, d)
		}
	}
	return out
}

// find is Find-ModelFile: the path of the file, or "".
func (r *runner) find(f modelFile) string {
	for _, d := range r.modelDirs(f.Folder) {
		if p := joinPath(d, f.Name); isFile(p) {
			return p
		}
	}
	return ""
}

// otherUsers is Get-OtherUsers: the other workflows that list the same folder and file name.
func (r *runner) otherUsers(f modelFile) ([]string, error) {
	ws, err := listWorkflows(r.workflowsDir)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, w := range ws {
		if strings.EqualFold(w.Name, r.workflow) || w.ModelsJSON == "" {
			continue
		}
		m, err := readManifest(w.ModelsJSON)
		if err != nil {
			return nil, err
		}
		for _, g := range m.Files {
			if strings.EqualFold(g.Folder, f.Folder) && strings.EqualFold(g.Name, f.Name) {
				users = append(users, w.Name)
				break
			}
		}
	}
	return users, nil
}
