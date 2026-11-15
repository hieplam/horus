// The tool install_workflow.
//
// On a machine where ComfyUI is installed, the tool downloads the missing
// model files after the license consent (accept_license). On a machine that
// needs more (a system package, comfy-cli, ComfyUI), it runs the plan of
// horus setup after the setup consent (accept_setup). The setup consent comes
// first, then the license consent (spec docs/specs/2026-11-15-release-install.md,
// "Setup through MCP").
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/provision"
)

// InstallResult is the result of install_workflow, in the key order that the tool shows.
type InstallResult struct {
	Workflow         string        `json:"workflow"`
	InstallState     string        `json:"install_state"`
	AlreadyInstalled bool          `json:"already_installed"`
	Files            []InstallFile `json:"files"`
	Seconds          float64       `json:"seconds"`
}

func installWorkflow(setup Setup) tool {
	return tool{
		name:  "install_workflow",
		title: "Install an image workflow",
		description: "Download and verify (SHA-256) the model files of one image workflow, so that generate_image can use it. " +
			"Files that are already present are kept. Downloads are large (several GB) and can take 10 to 60 minutes. " +
			"The models have their own license: the first call without accept_license=true returns the error " +
			"LICENSE_CONFIRMATION_REQUIRED with the license and the size. Ask the user, then call again with " +
			"accept_license=true. The call waits until the install is done and reports progress. " +
			"On a machine without ComfyUI, the first call without accept_setup=true returns the error " +
			"SETUP_CONFIRMATION_REQUIRED with each setup step (a system package, ComfyUI) in details.steps. " +
			"Ask the user, then call again with accept_setup=true. The setup consent comes before the license consent.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(true)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows."),
			f("accept_license", tBool).d("True only after the user accepted the license shown in the first reply.").dflt(false),
			f("accept_setup", tBool).d("True only after the user accepted the setup steps shown in details.steps of SETUP_CONFIRMATION_REQUIRED.").dflt(false),
		},
		output: envelope("InstallResult", obj{
			"InstallResult": model("InstallResult",
				f("workflow", tString),
				f("install_state", tConst("installed")),
				f("already_installed", tBool),
				f("files", tList(ref("InstallFile"))),
				f("seconds", tNumber),
			),
			"InstallFile": installFileModel,
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			return install(ctx, c, setup)
		},
	}
}

// progressInterval is the time between two progress notifications of an
// install. Tests make it shorter.
var progressInterval = 5 * time.Second

// sizeText writes a size in bytes as MB or GB.
func sizeText(n int64) string {
	if n >= 100_000_000 {
		return fmt.Sprintf("%.1f GB", float64(n)/1e9)
	}
	return fmt.Sprintf("%.2f MB", float64(n)/1e6)
}

func seconds(t0 time.Time) float64 { return math.Round(time.Since(t0).Seconds()*10) / 10 }

// licenseFile is one entry of details.files of LICENSE_CONFIRMATION_REQUIRED.
type licenseFile struct {
	Role  string `json:"role"`
	File  string `json:"file"`
	Bytes int64  `json:"bytes"`
}

func install(ctx context.Context, c *call, setup Setup) (any, error) {
	t0 := time.Now()
	w, err := catalog.Get(c.args.str("workflow"))
	if err != nil {
		return nil, err
	}
	// The plan reads the facts and changes nothing. A plan with only model
	// files (ComfyUI is installed) goes on as before.
	if pl, err := setup.Plan(context.WithoutCancel(ctx), request(w)); err == nil && setupSteps(pl) > 0 {
		return installWithSetup(ctx, c, setup, w, pl, t0)
	}
	status, err := listStatus(ctx, setup)
	if err != nil {
		return nil, err
	}
	if row, ok := status[w.Name]; ok && row.State() == "installed" {
		req := request(w)
		req.Quick = true
		if res, _ := setup.Check(context.WithoutCancel(ctx), req); res.OK {
			files := fileRows(res.Items, "file", map[string]string{"OK": "present"})
			return InstallResult{Workflow: w.Name, InstallState: "installed", AlreadyInstalled: true, Files: files, Seconds: seconds(t0)}, nil
		}
	}
	lic, err := w.License()
	if err != nil {
		return nil, err
	}
	if !c.args.boolean("accept_license") {
		return nil, licenseRequired(w)
	}
	res, err := watchInstall(ctx, c, setup, float64(lic.DownloadBytes), 0, func(watch provision.Watch) (*provision.Result, error) {
		return setup.Install(ctx, request(w), watch)
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, installError(res, err, w)
	}
	files := fileRows(res.Items, "download", map[string]string{"DOWNLOADED": "downloaded", "PRESENT": "present"})
	return InstallResult{Workflow: w.Name, InstallState: "installed", AlreadyInstalled: false, Files: files, Seconds: seconds(t0)}, nil
}

// licenseRequired is the answer LICENSE_CONFIRMATION_REQUIRED of w: its
// license, its download size and its model files.
func licenseRequired(w *catalog.Workflow) error {
	lic, err := w.License()
	if err != nil {
		return err
	}
	m, err := w.Models()
	if err != nil {
		return err
	}
	files := []licenseFile{}
	for _, f := range m.Files {
		parts := strings.Split(f.RepoPath, "/")
		files = append(files, licenseFile{Role: f.Role, File: parts[len(parts)-1], Bytes: f.Bytes})
	}
	return herr.New("LICENSE_CONFIRMATION_REQUIRED",
		fmt.Sprintf("installing %s downloads %s under the license '%s'", w.Name, sizeText(lic.DownloadBytes), lic.Name),
		map[string]any{"license": lic.Name, "license_url": lic.URL, "download_bytes": lic.DownloadBytes, "files": files})
}

// watchInstall runs run in its own goroutine and sends a progress
// notification every progressInterval: the setup step that runs (one of
// steps), else the bytes of the download so far. When a setup step starts,
// it sends one notification before the step runs. A cancel of the call ends
// ctx: run stops the download and returns the error of ctx. The .part file
// stays.
func watchInstall(ctx context.Context, c *call, setup Setup, total float64, steps int,
	run func(provision.Watch) (*provision.Result, error)) (*provision.Result, error) {
	var mu sync.Mutex // guards current, step and n, and sends one notification at a time
	current := ""     // the file that the install downloads now
	step := ""        // the text of the setup step that runs now
	n := 0            // the number of that step
	watch := provision.Watch{
		Downloading: func(_, dest string) {
			mu.Lock()
			current, step = dest, ""
			mu.Unlock()
		},
		Step: func(st provision.Step) {
			mu.Lock()
			defer mu.Unlock()
			n++
			step = fmt.Sprintf("setup step %d of %d: %s", n, steps, stepText(st))
			c.report(ctx, 0, total, step)
		},
	}
	type outcome struct {
		res *provision.Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := run(watch)
		done <- outcome{res, err}
	}()
	tick := time.NewTicker(progressInterval)
	defer tick.Stop()
	for {
		select {
		case o := <-done:
			return o.res, o.err
		case <-tick.C:
		}
		mu.Lock()
		if step != "" {
			c.report(ctx, 0, total, step)
			mu.Unlock()
			continue
		}
		dest := current
		mu.Unlock()
		size := setup.PartSize(dest)
		parts := strings.Split(strings.ReplaceAll(dest, "\\", "/"), "/")
		name := parts[len(parts)-1]
		msg := "checking files"
		if name != "" {
			msg = fmt.Sprintf("downloading %s: %.2f GB", name, float64(size)/1e9)
		}
		c.report(ctx, float64(size), total, msg)
	}
}

// -- the setup steps ------------------------------------------------------------

// setupStep is one entry of details.steps of SETUP_CONFIRMATION_REQUIRED, in
// the order of the plan.
type setupStep struct {
	Kind    string   `json:"kind"`    // package, comfy-cli, backend or file (provision.Step kinds)
	Name    string   `json:"name"`    // what the step installs: the package, comfy-cli, ComfyUI or the file name
	Command []string `json:"command"` // the program and its arguments of a package step, else null
	Dir     *string  `json:"dir"`     // the folder that the step writes to, or null
}

// setupSteps is the number of steps of pl that are not a model file.
func setupSteps(pl provision.Plan) int {
	n := 0
	for _, st := range pl.Steps {
		if st.Kind != provision.StepFile {
			n++
		}
	}
	return n
}

// stepRows gives details.steps of pl.
func stepRows(pl provision.Plan) []setupStep {
	rows := []setupStep{}
	for _, st := range pl.Steps {
		row := setupStep{Kind: st.Kind, Name: st.Name, Command: st.Command, Dir: ptr(st.Dir)}
		if st.Kind == provision.StepFile {
			row.Dir = ptr(filepath.Dir(st.File.Path))
		}
		if *row.Dir == "" {
			row.Dir = nil
		}
		rows = append(rows, row)
	}
	return rows
}

// stepText is one line for a step of a plan: the message of a progress
// notification and of the answers.
func stepText(st provision.Step) string {
	switch st.Kind {
	case provision.StepPackage:
		return fmt.Sprintf("install the system package %s: %s", st.Name, strings.Join(st.Command, " "))
	case provision.StepPerson:
		return fmt.Sprintf("install %s by hand from %s", st.Name, st.Package.URL)
	case provision.StepComfyCLI:
		return "install comfy-cli in " + st.Dir
	case provision.StepBackend:
		return "install ComfyUI in " + st.Dir
	}
	return "download " + st.File.File
}

// lockedText is the output of the programs of the setup steps. The steps
// write it while the tool reads it.
type lockedText struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedText) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// tail is the last n lines of the output.
func (l *lockedText) tail(n int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(l.b.String()), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// installWithSetup is install_workflow on a machine whose plan has a step
// that is not a model file. It changes nothing before both consents.
func installWithSetup(ctx context.Context, c *call, setup Setup, w *catalog.Workflow, pl provision.Plan, t0 time.Time) (any, error) {
	steps := stepRows(pl)
	var person *provision.PersonStepError
	if errors.As(pl.Refusal(true), &person) {
		pkg := person.Package
		e := herr.New("BACKEND_NOT_INSTALLED",
			fmt.Sprintf("%s was not found, so horus cannot install %s, which the install of ComfyUI needs", pkg.Manager, pkg.Name),
			map[string]any{"package": pkg.Name, "manager": pkg.Manager, "url": pkg.URL, "steps": steps})
		e.NextStep = fmt.Sprintf("Ask the user to install %s by hand from %s, then call install_workflow again.", pkg.Name, pkg.URL)
		return nil, e
	}
	if !c.args.boolean("accept_setup") {
		var lines []string
		for _, st := range pl.Steps {
			if st.Kind != provision.StepFile {
				lines = append(lines, stepText(st))
			}
		}
		return nil, herr.New("SETUP_CONFIRMATION_REQUIRED",
			fmt.Sprintf("this machine needs %d setup step(s) before %s can run: %s", len(lines), w.Name, strings.Join(lines, "; ")),
			map[string]any{"workflow": w.Name, "steps": steps})
	}
	files := len(pl.Steps) - setupSteps(pl)
	if files > 0 && !c.args.boolean("accept_license") {
		return nil, licenseRequired(w)
	}
	lic, err := w.License()
	if err != nil {
		return nil, err
	}
	req := request(w)
	req.WithPackages = true // the setup consent covers the package steps: details.steps showed each one
	out := &lockedText{}
	var failed provision.Step
	res, err := watchInstall(ctx, c, setup, float64(lic.DownloadBytes), len(pl.Steps)-files, func(watch provision.Watch) (*provision.Result, error) {
		step := watch.Step
		watch.Step = func(st provision.Step) {
			failed = st // the step that runs now: the step that failed when RunPlan returns an error
			step(st)
		}
		watch.Out = out
		return setup.RunPlan(ctx, req, pl, watch)
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		if res != nil {
			return nil, installError(res, err, w)
		}
		row := stepRows(provision.Plan{Steps: []provision.Step{failed}})[0]
		e := herr.New("BACKEND_NOT_INSTALLED", fmt.Sprintf("the setup step %s (%s) failed: %v", failed.Name, failed.Kind, err),
			map[string]any{"step": row, "steps": steps, "output_tail": out.tail(20)})
		e.NextStep = "Show the message and details.step to the user. When the user has fixed the cause, call install_workflow again with accept_setup=true."
		return nil, e
	}
	return InstallResult{Workflow: w.Name, InstallState: "installed", AlreadyInstalled: false, Files: planFiles(pl, res), Seconds: seconds(t0)}, nil
}

// planFiles gives the model files after a plan ran: the state of each
// downloaded file from res, and "present" for a file that the plan found.
func planFiles(pl provision.Plan, res *provision.Result) []InstallFile {
	got := map[string]InstallFile{}
	if res != nil {
		for _, row := range fileRows(res.Items, "download", map[string]string{"DOWNLOADED": "downloaded", "PRESENT": "present"}) {
			got[row.File] = row
		}
	}
	rows := []InstallFile{}
	for _, f := range pl.Files {
		if row, ok := got[f.File]; ok {
			rows = append(rows, row)
		} else if f.Present {
			rows = append(rows, InstallFile{Role: f.Role, File: f.File, Bytes: f.Bytes, State: "present", Path: ptr(f.Path)})
		}
	}
	return rows
}
