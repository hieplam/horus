// The uninstall of horus (horus uninstall): first the plan, then, only when
// the caller asks, the removal of each REMOVE item of the plan.
//
// The rules of the spec (docs/specs/2026-11-15-release-install.md,
// "horus uninstall"):
//
//   - The model files of each workflow go by the rules of horus workflow
//     remove. A file whose size differs from models.json stays.
//   - The ComfyUI folder and the venv of horus install-comfy-mcp go only
//     when they hold the install record (platform.InstallRecordName). A
//     ComfyUI that answers and that horus did not start keeps its folder.
//     horus first stops a ComfyUI that horus started.
//   - The server horus of Claude Code and of Codex goes when the client is
//     found and knows the server.
//   - In the user folder, the copy of the built-in workflows and the state
//     files of horus go. .env and outputs/ stay: they are data of the user.
//   - The PATH entry of the install scripts goes, then the program file.
//
// The plan is a pure function of the facts (planUninstall). A folder is
// removed with platform.RemoveTree, which never follows a link or a junction.
package provision

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// The parts of an uninstall, in the order of the run.
const (
	PartModels  = "models"  // the model files of each workflow
	PartComfyUI = "comfyui" // the ComfyUI install folder
	PartVenv    = "venv"    // the venv of horus install-comfy-mcp
	PartMCP     = "mcp"     // the server horus of Claude Code and of Codex
	PartUser    = "user"    // the files of horus in the user folder
	PartPath    = "path"    // the PATH entry of the install scripts
	PartProgram = "program" // the program file
)

// ComfyInstall is what an uninstall needs from the ComfyUI install.
// cmd/horus gives it the backend of internal/backend.
type ComfyInstall interface {
	// Folder is the folder that the install made, with the install record.
	Folder() string
	// Running reports if a server answers at the address, and if horus
	// started it.
	Running(ctx context.Context) (answers, started bool)
	// Stop ends the ComfyUI that horus started.
	Stop(ctx context.Context, out io.Writer) error
}

// Installer is what an uninstall needs from the OS: the PATH entry of the
// install scripts and the program file. cmd/horus gives it the functions of
// internal/platform. A test gives it a fake.
type Installer interface {
	// Program is the file of the running program.
	Program() (string, error)
	// PathEntry gives the places that hold the PATH entry of the install
	// scripts for program. An empty list means no entry.
	PathEntry(program string) ([]string, error)
	// RemovePathEntry removes that entry.
	RemovePathEntry(program string) error
	// RemoveProgram removes the file of the running program. On Windows the
	// file goes a moment after the end of the program.
	RemoveProgram() error
}

// UninstallSystem is what an uninstall needs besides the model files.
type UninstallSystem struct {
	Comfy    ComfyInstall
	OS       Installer
	Programs Programs // finds and runs claude and codex
	UserDir  string   // the user folder; "" means settings.UserDir()
	VenvPath string   // the venv of horus install-comfy-mcp; "" means settings.ComfyMCPVenv()
	AskDir   string   // the folder where "<client> mcp get horus" runs; "" means os.TempDir()
}

// WithUninstall gives the service the ComfyUI install, the OS and the
// programs for horus uninstall. It returns the service.
func (s *Service) WithUninstall(u UninstallSystem) *Service {
	s.un = u
	return s
}

// UninstallRequest is the request of horus uninstall.
type UninstallRequest struct {
	WorkflowsDir string // the folder of the workflows
	// Models limits the uninstall to the model files, MCP to the MCP
	// registration. Both false means every part.
	Models, MCP bool
}

// parts gives the parts of the request.
func (r UninstallRequest) parts() map[string]bool {
	if !r.Models && !r.MCP {
		return map[string]bool{PartModels: true, PartComfyUI: true, PartVenv: true, PartMCP: true, PartUser: true, PartPath: true, PartProgram: true}
	}
	return map[string]bool{PartModels: r.Models, PartMCP: r.MCP}
}

// UninstallItem is one line of the plan: a REMOVE item or a KEEP item.
type UninstallItem struct {
	Part   string
	Remove bool   // true: REMOVE, false: KEEP
	What   string // what it is, for a person
	Path   string // the path; for the MCP part the client program, for the PATH entry the place
	Why    string // KEEP: the reason; REMOVE: a note, or ""
	// Command is the program and its arguments that remove an MCP
	// registration.
	Command []string
	// Done and Err are the result of RunUninstall: Done is true when the
	// item is gone.
	Done bool
	Err  error
}

// ClientFact is one MCP client on this machine.
type ClientFact struct {
	Name, Title string
	Program     string // the program of the client, "" when it is not found
	Knows       bool   // "<client> mcp get horus" exits 0
}

// ModelFact is one model file of the plan of horus workflow remove.
type ModelFact struct {
	Workflow string
	Item     Item // StatusWouldDelete or StatusRefused
}

// UninstallFacts is what horus uninstall found on this machine.
type UninstallFacts struct {
	Parts  map[string]bool
	Models []ModelFact

	ComfyFolder                string
	ComfyExists, ComfyRecord   bool
	ComfyAnswers, ComfyStarted bool
	VenvPath                   string
	VenvExists, VenvRecord     bool
	Clients                    []ClientFact
	UserDir                    string
	UserFiles                  []string // the files and folders of horus in the user folder
	StateFile                  string   // the state file of a started ComfyUI in the user folder, or ""
	PathPlaces                 []string // the places of the PATH entry
	Program                    string   // the file of the running program, or ""
}

// Uninstall is the plan of horus uninstall: the facts and the items, in the
// order of the run.
type Uninstall struct {
	Facts UninstallFacts
	Items []UninstallItem
}

// Removes gives the number of REMOVE items.
func (u *Uninstall) Removes() int {
	n := 0
	for _, it := range u.Items {
		if it.Remove {
			n++
		}
	}
	return n
}

// The reasons of a KEEP item.
const (
	whyUserData = "data of the user"
	whyNoRecord = "it has no install record " + platform.InstallRecordName +
		", so horus did not make it (or an older horus did). Delete it by hand when you do not need it."
)

// planUninstall gives the items for the facts, in the order of the run. It
// is a pure function.
func planUninstall(f UninstallFacts) []UninstallItem {
	var out []UninstallItem
	add := func(it UninstallItem) { out = append(out, it) }
	if f.Parts[PartModels] {
		seen := map[string]bool{}
		for _, m := range f.Models {
			key := strings.ToLower(m.Item.Path)
			if seen[key] {
				continue
			}
			seen[key] = true
			what := fmt.Sprintf("model file %s of workflow %s", m.Item.File, m.Workflow)
			if m.Item.Status == StatusRefused {
				add(UninstallItem{Part: PartModels, What: what, Path: m.Item.Path, Why: m.Item.Detail})
			} else {
				add(UninstallItem{Part: PartModels, Remove: true, What: what, Path: m.Item.Path})
			}
		}
	}
	if f.Parts[PartComfyUI] && f.ComfyExists {
		it := UninstallItem{Part: PartComfyUI, What: "the ComfyUI install folder", Path: f.ComfyFolder}
		switch {
		case !f.ComfyRecord:
			it.Why = whyNoRecord
		case f.ComfyAnswers && !f.ComfyStarted:
			it.Why = "a ComfyUI answers, and horus did not start it. Stop it where it was started, then run horus uninstall again."
		default:
			it.Remove = true
			if f.ComfyStarted {
				it.Why = "horus first stops the ComfyUI that it started"
			}
		}
		add(it)
	}
	if f.Parts[PartVenv] && f.VenvExists {
		it := UninstallItem{Part: PartVenv, What: "the venv of horus install-comfy-mcp", Path: f.VenvPath, Remove: f.VenvRecord}
		if !f.VenvRecord {
			it.Why = whyNoRecord
		}
		add(it)
	}
	if f.Parts[PartMCP] {
		for _, c := range f.Clients {
			if c.Program == "" || !c.Knows {
				continue
			}
			add(UninstallItem{Part: PartMCP, Remove: true, What: "the MCP server horus of " + c.Title, Path: c.Program,
				Command: append([]string{c.Program}, removeArgs(c.Name)...)})
		}
	}
	if f.Parts[PartUser] {
		comfyGoes := hasRemove(out, PartComfyUI)
		for _, p := range f.UserFiles {
			add(UninstallItem{Part: PartUser, Remove: true, What: "a file of horus in the user folder", Path: p})
		}
		if f.StateFile != "" {
			it := UninstallItem{Part: PartUser, What: "the state file of the ComfyUI that horus started", Path: f.StateFile}
			if f.ComfyStarted && !comfyGoes {
				it.Why = "it names the ComfyUI that horus started, and horus stop-comfyui needs it"
			} else {
				it.Remove = true
			}
			add(it)
		}
		add(UninstallItem{Part: PartUser, What: "the .env file", Path: filepath.Join(f.UserDir, ".env"), Why: whyUserData})
		add(UninstallItem{Part: PartUser, What: "the images", Path: filepath.Join(f.UserDir, "outputs"), Why: whyUserData})
	}
	if f.Parts[PartPath] {
		for _, p := range f.PathPlaces {
			add(UninstallItem{Part: PartPath, Remove: true, What: "the PATH entry of the install script", Path: p})
		}
	}
	if f.Parts[PartProgram] && f.Program != "" {
		add(UninstallItem{Part: PartProgram, Remove: true, What: "the program file", Path: f.Program,
			Why: "the last item. A system package (7-Zip, Python) stays."})
	}
	return out
}

func hasRemove(items []UninstallItem, part string) bool {
	for _, it := range items {
		if it.Part == part && it.Remove {
			return true
		}
	}
	return false
}

// removeArgs are the arguments of "<client> mcp remove" for the server horus
// (checked with "claude mcp remove --help" and "codex mcp remove --help").
func removeArgs(client string) []string {
	if client == ClientCodex {
		return []string{"mcp", "remove", "horus"}
	}
	return []string{"mcp", "remove", "--scope", "user", "horus"}
}

func (s *Service) userDir() string {
	if s.un.UserDir != "" {
		return s.un.UserDir
	}
	return settings.UserDir()
}

func (s *Service) uninstallVenv() string {
	if s.un.VenvPath != "" {
		return s.un.VenvPath
	}
	return settings.ComfyMCPVenv()
}

func (s *Service) askDir() string {
	if s.un.AskDir != "" {
		return s.un.AskDir
	}
	return os.TempDir()
}

// exists reports if something is at p, also a link (it does not follow it).
func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// UninstallPlan reads the facts of this machine and gives the plan of horus
// uninstall. It changes nothing: it reads files, asks the ComfyUI server,
// and runs "<client> mcp get horus".
func (s *Service) UninstallPlan(ctx context.Context, req UninstallRequest) (*Uninstall, error) {
	f := UninstallFacts{Parts: req.parts()}
	if f.Parts[PartModels] {
		ms, err := s.modelFacts(ctx, req.WorkflowsDir)
		if err != nil {
			return nil, err
		}
		f.Models = ms
	}
	if f.Parts[PartComfyUI] && s.un.Comfy != nil {
		f.ComfyFolder = s.un.Comfy.Folder()
		if fi, err := os.Lstat(f.ComfyFolder); err == nil && (fi.IsDir() || platform.IsLink(fi.Mode())) {
			f.ComfyExists = true
			f.ComfyRecord = platform.HasInstallRecord(f.ComfyFolder, platform.RecordComfyUI)
			f.ComfyAnswers, f.ComfyStarted = s.un.Comfy.Running(ctx)
		}
	}
	if f.Parts[PartVenv] {
		f.VenvPath = s.uninstallVenv()
		f.VenvExists = exists(f.VenvPath)
		f.VenvRecord = f.VenvExists && platform.HasInstallRecord(f.VenvPath, platform.RecordComfyMCP)
	}
	if f.Parts[PartMCP] && s.un.Programs != nil {
		f.Clients = s.clientFacts(ctx)
	}
	if f.Parts[PartUser] {
		f.UserDir = s.userDir()
		f.UserFiles = userFiles(f.UserDir)
		if p := filepath.Join(f.UserDir, stateName); exists(p) {
			f.StateFile = p
		}
	}
	if (f.Parts[PartPath] || f.Parts[PartProgram]) && s.un.OS != nil {
		prog, err := s.un.OS.Program()
		if err != nil {
			return nil, err
		}
		f.Program = prog
		if f.Parts[PartPath] {
			if f.PathPlaces, err = s.un.OS.PathEntry(prog); err != nil {
				return nil, err
			}
		}
	}
	return &Uninstall{Facts: f, Items: planUninstall(f)}, nil
}

// stateName is the state file of a ComfyUI that horus started
// (internal/backend). On macOS it is in the user folder, because the user
// folder is the install folder of ComfyUI.
const stateName = "horus-comfyui.state.json"

// userFiles gives the files and folders of horus in the user folder: the
// copy of the built-in workflows, and its temporary folders of an
// interrupted write.
func userFiles(dir string) []string {
	var out []string
	if p := filepath.Join(dir, "builtin-workflows"); exists(p) {
		out = append(out, p)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "builtin-workflows.tmp-") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// modelFacts runs the plan of horus workflow remove (no -yes) for each
// workflow of the folder, with -force: the uninstall removes every workflow,
// so a file that two workflows list goes too.
func (s *Service) modelFacts(ctx context.Context, workflowsDir string) ([]ModelFact, error) {
	ws, err := catalog.Folders(workflowsDir)
	if err != nil {
		return nil, err
	}
	var out []ModelFact
	for _, w := range ws {
		if w.ModelsJSON == "" {
			continue
		}
		res, err := s.Remove(ctx, Request{WorkflowsDir: workflowsDir, Workflow: w.Name, Force: true})
		if err != nil && (res == nil || len(res.Items) == 0) {
			return nil, err
		}
		for _, it := range res.Items {
			if it.Status == StatusWouldDelete || it.Status == StatusRefused {
				out = append(out, ModelFact{Workflow: w.Name, Item: it})
			}
		}
	}
	return out, nil
}

// clientFacts finds claude and codex, and asks each one found if it knows
// the server horus.
func (s *Service) clientFacts(ctx context.Context) []ClientFact {
	var out []ClientFact
	for _, name := range []string{ClientClaude, ClientCodex} {
		c, _ := clientOf(name)
		f := ClientFact{Name: name, Title: c.title}
		if p, err := s.un.Programs.LookPath(name); err == nil {
			f.Program = p
			f.Knows = s.knows(ctx, p)
		}
		out = append(out, f)
	}
	return out
}

func (s *Service) knows(ctx context.Context, program string) bool {
	code, err := s.un.Programs.Run(ctx, platform.Command{Path: program, Args: []string{"mcp", "get", "horus"}, Dir: s.askDir()})
	return err == nil && code == 0
}

// RunUninstall removes each REMOVE item of u, in order. Its lines go to out.
// Then it checks each REMOVE item again and sets Done. The error is not nil
// when a REMOVE item is still there.
func (s *Service) RunUninstall(ctx context.Context, u *Uninstall, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	items := u.Items
	// 1. Stop the ComfyUI that horus started, before its folder goes.
	for i := range items {
		it := &items[i]
		if it.Part == PartComfyUI && it.Remove && u.Facts.ComfyStarted && s.un.Comfy != nil {
			if err := s.un.Comfy.Stop(ctx, out); err != nil {
				it.Err = fmt.Errorf("cannot stop the ComfyUI that horus started, so its folder stays: %w", err)
			}
		}
	}
	pathDone := false
	for i := range items {
		it := &items[i]
		if !it.Remove || it.Err != nil {
			continue
		}
		switch it.Part {
		case PartModels:
			it.Err = s.files.Delete(it.Path)
			if errors.Is(it.Err, fs.ErrNotExist) {
				it.Err = nil
			}
		case PartComfyUI, PartVenv, PartUser:
			it.Err = platform.RemoveTree(it.Path)
		case PartMCP:
			code, err := s.un.Programs.Run(ctx, platform.Command{Path: it.Command[0], Args: it.Command[1:], Dir: s.askDir(), Stdout: out, Stderr: out})
			if err == nil && code != 0 {
				err = fmt.Errorf("%s exited with code %d", strings.Join(it.Command, " "), code)
			}
			it.Err = err
		case PartPath:
			if !pathDone {
				pathDone = true
				it.Err = s.un.OS.RemovePathEntry(u.Facts.Program)
			}
		case PartProgram:
			it.Err = s.un.OS.RemoveProgram()
		}
	}
	// The user folder itself goes when nothing is left in it.
	if u.Facts.Parts[PartUser] && u.Facts.UserDir != "" {
		if entries, err := os.ReadDir(u.Facts.UserDir); err == nil && len(entries) == 0 {
			if os.Remove(u.Facts.UserDir) == nil {
				fmt.Fprintf(out, "OK: removed the empty user folder %s\n", u.Facts.UserDir)
			}
		}
	}
	// 2. Check each REMOVE item again.
	var places []string
	if pathDone {
		places, _ = s.un.OS.PathEntry(u.Facts.Program)
	}
	left := 0
	for i := range items {
		it := &items[i]
		if !it.Remove {
			continue
		}
		switch it.Part {
		case PartMCP:
			it.Done = it.Err == nil && !s.knows(ctx, it.Path)
		case PartPath:
			it.Done = it.Err == nil && !contains(places, it.Path)
		case PartProgram:
			// On Windows the file goes after the end of this program.
			it.Done = it.Err == nil
		default:
			it.Done = it.Err == nil && !exists(it.Path)
		}
		if !it.Done {
			left++
		}
	}
	if left > 0 {
		return fmt.Errorf("%d of %d REMOVE item(s) are still there. See the FAIL lines.", left, u.Removes())
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
