// horus workflow lists the horus workflows, and installs, checks or removes
// the model files of one.
// The work is in the service internal/provision.
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
// With -json, check, install and remove print one last line
// "HORUS_JSON {...}": ok, action, workflow, message and one item per check or
// file (status, kind, role, file, path, bytes, detail). list -json prints a
// JSON list instead of the table.
//
// Exit code 0 = the action succeeded, 1 = anything else, 2 = a flag that does
// not parse.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// Mark starts the result line of horus workflow -json.
const Mark = "HORUS_JSON "

// Setup is what horus workflow needs from the setup service.
// cmd/horus gives it a *provision.Service.
type Setup interface {
	List(ctx context.Context, workflowsDir string) ([]provision.Row, error)
	Check(ctx context.Context, req provision.Request) (*provision.Result, error)
	Install(ctx context.Context, req provision.Request, w provision.Watch) (*provision.Result, error)
	Remove(ctx context.Context, req provision.Request) (*provision.Result, error)
}

// SetupPlace is the ComfyUI and the folders that the flags of horus workflow
// pick. An empty field means the setting.
type SetupPlace struct {
	ComfyUIDir     string // -comfyui-dir
	ModelsDir      string // -models-dir
	TextEncoderDir string // -text-encoder-dir
	ServerURL      string // -server-url
}

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

// Workflow runs horus workflow with args (the arguments after the command
// name). It writes its lines to out and the usage to errOut, and returns
// the exit code. setupAt gives the setup service for the place of the flags.
func Workflow(ctx context.Context, args []string, out, errOut io.Writer, setupAt func(SetupPlace) Setup) int {
	var place SetupPlace
	var req provision.Request
	var jsonOut bool
	fs := flag.NewFlagSet("horus workflow", flag.ContinueOnError)
	fs.SetOutput(errOut)
	only := listFlag{}
	fs.Var(&only, "only", "limit install, check and remove to these roles or file names (comma separated, or the flag again)")
	fs.BoolVar(&req.Yes, "yes", false, "remove: delete the files; without it, remove only prints the plan")
	fs.BoolVar(&req.Force, "force", false, "remove: also delete a file that another workflow lists")
	fs.BoolVar(&req.Quick, "quick", false, "check and list: compare the size only, no SHA-256")
	fs.BoolVar(&jsonOut, "json", false, "list: print JSON; check, install, remove: add the HORUS_JSON line")
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI portable folder (default: HORUS_COMFYUI_DIR, then ~/ComfyUI_windows_portable)")
	fs.StringVar(&place.ModelsDir, "models-dir", "", "the models folder (default: HORUS_MODELS_DIR, then <comfyui-dir>/ComfyUI/models)")
	fs.StringVar(&place.TextEncoderDir, "text-encoder-dir", "", "the text encoder folder (default: HORUS_TEXT_ENCODER_DIR)")
	fs.StringVar(&place.ServerURL, "server-url", "", "the ComfyUI address (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	fs.StringVar(&req.ModelsJSON, "models-json", "", "use this file instead of workflows/<workflow>/models.json")
	fs.StringVar(&req.WorkflowsDir, "workflows-dir", "", "use this folder instead of the repo's workflows folder")
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
	req.Only = only
	if len(pos) == 0 || len(pos) > 2 {
		fs.Usage()
		return 2
	}
	action := strings.ToLower(pos[0])
	switch action {
	case "list", "check", "install", "remove":
	default:
		fmt.Fprintf(errOut, "horus workflow: unknown action %q: use list, check, install or remove\n", pos[0])
		return 2
	}
	if len(pos) == 2 {
		req.Workflow = pos[1]
	}
	if req.WorkflowsDir == "" {
		req.WorkflowsDir = filepath.Join(settings.Root(), "workflows")
	}
	p := &printer{out: out, json: jsonOut}
	svc := setupAt(place)
	var res *provision.Result
	var err error
	switch action {
	case "list":
		return p.list(ctx, svc, req.WorkflowsDir)
	case "check":
		res, err = svc.Check(ctx, req)
		p.check(res)
		if err == nil {
			p.say("OK: workflow '%s' is ready.", req.Workflow)
		}
	case "install":
		res, err = svc.Install(ctx, req, provision.Watch{
			Started: func(license string, files int) {
				p.say("License: %s", license)
				p.say("Installing %d model file(s) for workflow '%s'.", files, req.Workflow)
			},
			Downloading: func(role, dest string) { p.say("-- %s -> %s", role, dest) },
			Out:         out,
			Failed:      func(err error) { p.say("FAIL: %v", err) },
		})
		p.install(res)
		if err == nil {
			p.say("OK: workflow '%s' is installed and ready.", req.Workflow)
		}
	default:
		res, err = svc.Remove(ctx, req)
		p.remove(res, err == nil)
	}
	if err != nil {
		p.say("FAIL: %v", err)
	}
	p.result(res)
	if err != nil {
		return 1
	}
	return 0
}

// printer writes the lines of horus workflow. horus doctor writes the lines
// of a check with an indent.
type printer struct {
	out    io.Writer
	json   bool
	indent string
}

func (p *printer) say(format string, a ...any) {
	io.WriteString(p.out, p.indent)
	fmt.Fprintf(p.out, format+"\n", a...)
}

// result writes the HORUS_JSON line.
func (p *printer) result(res *provision.Result) {
	if !p.json || res == nil {
		return
	}
	if res.Items == nil {
		res.Items = []provision.Item{}
	}
	b, _ := json.Marshal(res)
	fmt.Fprintln(p.out, Mark+string(b))
}

func (p *printer) list(ctx context.Context, svc Setup, dir string) int {
	rows, err := svc.List(ctx, dir)
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	if p.json {
		b, _ := json.MarshalIndent(rows, "", "    ")
		fmt.Fprintln(p.out, string(b))
		return 0
	}
	if len(rows) == 0 {
		p.say("No workflows in %s", dir)
		return 0
	}
	for _, row := range rows {
		p.say("%s", row.Name)
		p.say("  %s", row.Description)
		p.say("  Models: %d file(s), %s GB (%s). Installed: %s (size check).", row.Files,
			strconv.FormatFloat(row.Gigabytes, 'f', -1, 64), row.Roles, row.Installed)
		p.say("")
	}
	p.say("Install: horus workflow install <name>    Check: horus workflow check <name>")
	return 0
}

// check writes the lines of the check part: the head line and one line for
// each item that is not a download.
func (p *printer) check(res *provision.Result) {
	if !res.Checked {
		return
	}
	quick := ""
	if res.Quick {
		quick = ", size only"
	}
	p.say("Checking workflow '%s': %d model file(s)%s.", res.Workflow, res.CheckFiles, quick)
	for _, it := range res.Items {
		if it.Kind != "download" {
			p.say("%s", checkLine(it, res.Quick))
		}
	}
}

// checkLine is the line of one item of a check.
func checkLine(it provision.Item, quick bool) string {
	switch it.Status {
	case provision.StatusMissing:
		return fmt.Sprintf("MISSING   %s %s (%s)", it.Role, it.File, it.Detail)
	case provision.StatusBadSize:
		return fmt.Sprintf("BAD SIZE  %s %s has %d bytes, expected %d", it.Role, it.Path, it.Bytes, it.Want)
	case provision.StatusBadHash:
		return fmt.Sprintf("BAD HASH  %s %s %s", it.Role, it.Path, it.Detail)
	case provision.StatusUnlisted:
		return fmt.Sprintf("UNLISTED  workflow.json loads %s, but models.json does not list it. Add it to models.json.", it.File)
	case provision.StatusNoNode:
		return fmt.Sprintf("NO NODE   the server has no node class '%s'. Install its node pack (MCP install_node), then restart ComfyUI.", it.File)
	case provision.StatusNotSeen:
		return fmt.Sprintf("NOT SEEN  the server does not list %s. Check extra_model_paths.yaml, then restart ComfyUI.", it.File)
	case provision.StatusSkip:
		if it.Kind == "graph" {
			return "SKIP      graph checks: no workflow.json"
		}
		return fmt.Sprintf("SKIP      server checks: %s (start ComfyUI with horus start-comfyui)", it.Detail)
	}
	switch {
	case it.Kind == "file" && quick:
		return fmt.Sprintf("OK        %s %s (%d bytes)", it.Role, it.Path, it.Bytes)
	case it.Kind == "file":
		return fmt.Sprintf("OK        %s %s (%d bytes, SHA-256 verified)", it.Role, it.Path, it.Bytes)
	case it.Kind == "graph":
		return fmt.Sprintf("OK        models.json lists all %d model file(s) that workflow.json loads", it.Count)
	case it.File == "":
		return fmt.Sprintf("OK        the server has all %d node classes of workflow.json", it.Count)
	}
	return fmt.Sprintf("OK        the server lists %s", it.File)
}

// install writes the lines after the downloads: the folders of
// extra_model_paths.yaml, then the check part.
func (p *printer) install(res *provision.Result) {
	added := false
	for _, m := range res.Mappings {
		if !m.Added {
			p.say("extra_model_paths.yaml already maps %s -> %s", m.Folder, m.Dir)
			continue
		}
		added = true
		p.say("extra_model_paths.yaml: added %s -> %s/%s", m.Folder, strings.ReplaceAll(filepath.Dir(m.Dir), `\`, "/"), filepath.Base(m.Dir))
	}
	if added {
		p.say("Restart ComfyUI so that it reads the new paths (horus start-comfyui).")
	}
	if res.Checked {
		p.say("")
		p.check(res)
	}
}

// remove writes the lines of a remove: the files that stay, the plan, the
// files that could not be deleted, and (when ok) the last line.
func (p *printer) remove(res *provision.Result, ok bool) {
	var plan []provision.Item
	for _, it := range res.Items {
		switch it.Status {
		case provision.StatusAbsent:
			p.say("ABSENT    %s %s", it.Role, it.File)
		case provision.StatusRefused:
			p.say("REFUSE    %s %s has %d bytes, models.json says %d. It is not the listed file. Delete it yourself.", it.Role, it.Path, it.Bytes, it.Want)
		case provision.StatusKeptShared:
			p.say("KEEP      %s %s (also used by: %s; -force deletes it)", it.Role, it.Path, strings.Join(it.Users, ", "))
		default:
			plan = append(plan, it)
		}
	}
	var total int64
	dry := false
	for _, x := range plan {
		total += x.Bytes
		dry = dry || x.Status == provision.StatusWouldDelete
	}
	verb := "DELETE   "
	if dry {
		verb = "WOULD DEL"
	}
	for _, x := range plan {
		label := x.Role
		switch {
		case x.Part:
			label = "unfinished download of " + strings.TrimSuffix(x.File, ".part")
		case len(x.Users) > 0:
			label += " (also used by: " + strings.Join(x.Users, ", ") + ")"
		}
		p.say("%s  %s %s (%s GB)", verb, label, x.Path, gb2(x.Bytes))
	}
	for _, x := range plan {
		if x.Status == provision.StatusDelFailed {
			p.say("FAIL      cannot delete %s: %s (Is ComfyUI using it? Stop ComfyUI and try again.)", x.Path, x.Detail)
		}
	}
	if !ok {
		return
	}
	switch {
	case len(plan) == 0:
		p.say("Nothing to delete for workflow '%s'.", res.Workflow)
	case dry:
		p.say("Dry run: nothing deleted. %s GB in %d file(s). Add -yes to delete.", gb2(total), len(plan))
	default:
		p.say("OK: deleted %d file(s), %s GB. Reinstall: horus workflow install %s", len(plan), gb2(total), res.Workflow)
	}
}

// gb2 writes bytes in GB with two decimals, as PowerShell "{0:N2}" -f ($bytes / 1e9) prints them on Windows in
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
