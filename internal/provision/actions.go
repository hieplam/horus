// The four actions: List, Check, Install and Remove. They port the actions
// of scripts/workflow.ps1.
package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
)

// List gives every workflow of the folder with its model files and the
// number of them on disk (a size check, no SHA-256).
func (s *Service) List(ctx context.Context, workflowsDir string) ([]Row, error) {
	ws, err := catalog.Folders(workflowsDir)
	if err != nil {
		return nil, err
	}
	rows := []Row{}
	for _, w := range ws {
		var files []catalog.ListedFile
		present := 0
		if w.ModelsJSON != "" {
			m, err := catalog.ReadModelList(w.ModelsJSON)
			if err != nil {
				return nil, err
			}
			files = m.Files
			for _, f := range files {
				if p := s.find(f); p != "" && s.files.Size(p) == f.Bytes {
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
		rows = append(rows, Row{Name: w.Name, Description: w.Description, Files: len(files), Present: present,
			Bytes: total, Gigabytes: roundGB(total), Installed: installedText(present, len(files)), Roles: strings.Join(roles, ", ")})
	}
	return rows, nil
}

// PartSize is the size of dest.part, else of dest, else 0: the bytes of a
// download so far.
func (s *Service) PartSize(dest string) int64 {
	if dest == "" {
		return 0
	}
	for _, p := range []string{dest + ".part", dest} {
		if n := s.files.Size(p); n >= 0 {
			return n
		}
	}
	return 0
}

// find is Find-ModelFile: the path of the file, or "".
func (s *Service) find(f catalog.ListedFile) string {
	for _, d := range s.files.Dirs(f.Folder) {
		if p := filepath.Join(d, f.Name); s.files.Size(p) >= 0 {
			return p
		}
	}
	return ""
}

// begin starts the result of an action on one workflow and reads its
// models.json.
func begin(action string, req Request) (*Result, catalog.ModelList, string, error) {
	res := &Result{Action: action, Workflow: req.Workflow, Items: []Item{}}
	m, graphPath, err := catalog.FindModelList(req.WorkflowsDir, req.Workflow, req.ModelsJSON)
	return res, m, graphPath, err
}

// end sets the outcome of the result.
func end(res *Result, err error, okMessage string) (*Result, error) {
	if err != nil {
		res.OK, res.Message = false, err.Error()
		return res, err
	}
	res.OK, res.Message = true, okMessage
	return res, nil
}

// Check checks that the machine is ready to run the workflow: every file of
// models.json on disk with the right size and SHA-256 (Quick: the size only),
// models.json lists every model file that workflow.json loads, and (when
// ComfyUI answers) the server has every node class and lists every model
// file. A failed check is an error, and the result has every item.
func (s *Service) Check(ctx context.Context, req Request) (*Result, error) {
	res, m, graphPath, err := begin("check", req)
	if err != nil {
		return end(res, err, "")
	}
	failures, err := s.check(ctx, res, m, graphPath, req.Only, req.Quick)
	if err == nil && failures > 0 {
		err = failf("%d check(s) failed. Fix: horus workflow install %s", failures, req.Workflow)
	}
	return end(res, err, fmt.Sprintf("workflow '%s' is ready", req.Workflow))
}

// check adds one item for each check, and returns the number of failures.
func (s *Service) check(ctx context.Context, res *Result, m catalog.ModelList, graphPath string, only []string, quick bool) (int, error) {
	files, err := selectFiles(m.Files, only)
	if err != nil {
		return 0, err
	}
	res.Checked, res.CheckFiles, res.Quick = true, len(files), quick
	failures := 0
	for _, f := range files {
		p := s.find(f)
		var size int64
		sha := ""
		if p != "" {
			size = s.files.Size(p)
			if size == f.Bytes && !quick {
				if sha, err = s.files.SHA256(p); err != nil {
					return failures, err
				}
			}
		}
		it := fileCheck(f, p, size, sha, quick, s.files.Dirs(f.Folder))
		if it.Status != StatusOK {
			failures++
		}
		res.add(it)
	}

	refs, err := catalog.ReadGraphRefs(graphPath)
	if err != nil {
		return failures, err
	}
	if refs == nil {
		res.add(Item{Status: StatusSkip, Kind: "graph", Bytes: -1, Detail: "no workflow.json"})
		return failures, nil
	}
	graphItems := graphCheck(m.Files, refs.Models)
	for _, it := range graphItems {
		if it.Status != StatusOK {
			failures++
		}
		res.add(it)
	}

	info, err := s.comfy.ObjectInfo(ctx)
	if err != nil && !notJSON(err) {
		res.add(Item{Status: StatusSkip, Kind: "server", Bytes: -1, Detail: "no answer at " + s.server()})
		return failures, nil
	}
	for _, it := range serverCheck(files, refs.NodeTypes, info) {
		if it.Status != StatusOK {
			failures++
		}
		res.add(it)
	}
	return failures, nil
}

// notJSON reports if the server answered, but not with a JSON object. The
// check then counts the answer as a server with no node class and no file,
// as workflow.ps1 did.
func notJSON(err error) bool {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	return errors.As(err, &syn) || errors.As(err, &typ)
}

// fileCheck is the item of one file of models.json: p is the path where it
// was found ("" = none), size its size, sha its SHA-256 ("" when quick or
// when the size is wrong), dirs the folders where it was looked for.
func fileCheck(f catalog.ListedFile, p string, size int64, sha string, quick bool, dirs []string) Item {
	it := Item{Kind: "file", Role: f.Role, File: f.Name, Path: p, Bytes: size}
	switch {
	case p == "":
		it.Status, it.Path, it.Bytes, it.Detail = StatusMissing, "", f.Bytes, "not in: "+strings.Join(dirs, "; ")
	case size != f.Bytes:
		it.Status, it.Want, it.Detail = StatusBadSize, f.Bytes, fmt.Sprintf("expected %d bytes", f.Bytes)
	case quick:
		it.Status, it.Detail = StatusOK, "size verified"
	case sha != f.SHA256:
		it.Status, it.Detail = StatusBadHash, fmt.Sprintf("SHA-256 is %s, expected %s", sha, f.SHA256)
	default:
		it.Status, it.Detail = StatusOK, "size and SHA-256 verified"
	}
	return it
}

// graphCheck compares the model files that workflow.json loads (wanted)
// with every file of models.json.
func graphCheck(listed []catalog.ListedFile, wanted []string) []Item {
	names := map[string]bool{}
	for _, f := range listed {
		names[strings.ToLower(f.Name)] = true
	}
	var out []Item
	for _, w := range wanted {
		if !names[strings.ToLower(w)] {
			out = append(out, Item{Status: StatusUnlisted, Kind: "graph", File: w, Bytes: -1,
				Detail: "workflow.json loads this file, but models.json does not list it"})
		}
	}
	if len(out) == 0 {
		out = append(out, Item{Status: StatusOK, Kind: "graph", Bytes: -1, Count: len(wanted),
			Detail: fmt.Sprintf("models.json lists all %d model files of workflow.json", len(wanted))})
	}
	return out
}

// serverCheck compares the node classes of the graph and the checked files
// with the /object_info of the server.
func serverCheck(files []catalog.ListedFile, types []string, info graph.ObjectInfo) []Item {
	var out []Item
	missing := 0
	for _, t := range types {
		if hasKey(info, t) {
			continue
		}
		missing++
		out = append(out, Item{Status: StatusNoNode, Kind: "server", File: t, Bytes: -1,
			Detail: fmt.Sprintf("the server has no node class '%s'", t)})
	}
	if missing == 0 {
		out = append(out, Item{Status: StatusOK, Kind: "server", Bytes: -1, Count: len(types),
			Detail: fmt.Sprintf("the server has all %d node classes", len(types))})
	}
	visible := listedModels(info)
	for _, f := range files {
		if visible[strings.ToLower(f.Name)] {
			out = append(out, Item{Status: StatusOK, Kind: "server", Role: f.Role, File: f.Name, Bytes: -1, Detail: "the server lists the file"})
		} else {
			out = append(out, Item{Status: StatusNotSeen, Kind: "server", Role: f.Role, File: f.Name, Bytes: -1, Detail: "the server does not list the file"})
		}
	}
	return out
}

// hasKey looks a node class up as workflow.ps1 reads a property: the exact
// name, else the name without case.
func hasKey(info graph.ObjectInfo, key string) bool {
	if _, ok := info[key]; ok {
		return true
	}
	for k := range info {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// listedModels is the set of model file names (lower case, without a
// folder) that the server lists anywhere in /object_info.
func listedModels(info graph.ObjectInfo) map[string]bool {
	out := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if graph.IsModelFile(x) {
				out[strings.ToLower(leaf(strings.ReplaceAll(x, `\`, "/")))] = true
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, raw := range info {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			walk(v)
		}
	}
	return out
}

// leaf is the last part of a path with / separators.
func leaf(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Install downloads and verifies the files of models.json, adds the download
// folders that are outside ComfyUI's own models folder to
// extra_model_paths.yaml, then checks (size only). w sees the downloads
// while they run. When ctx ends, the install stops after the download that
// runs: its .part file stays for the next install, and the error is the
// error of ctx.
func (s *Service) Install(ctx context.Context, req Request, w Watch) (*Result, error) {
	res, m, graphPath, err := begin("install", req)
	if err != nil {
		return end(res, err, "")
	}
	if dir, ok := s.backend.Installed(); !ok {
		return end(res, failure{fmt.Sprintf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", dir), ErrNoComfyUI}, "")
	}
	files, err := selectFiles(m.Files, req.Only)
	if err != nil {
		return end(res, err, "")
	}
	if w.Started != nil {
		w.Started(m.License, len(files))
	}
	out := w.Out
	if out == nil {
		out = io.Discard
	}
	failures := 0
	var targets [][2]string
	for _, f := range files {
		dest := s.find(f)
		if dest == "" {
			dir := s.files.Dirs(f.Folder)[0]
			dest = filepath.Join(dir, f.Name)
			targets = append(targets, [2]string{f.Folder, dir})
		}
		if w.Downloading != nil {
			w.Downloading(f.Role, dest)
		}
		skipped, err := s.dl.Download(ctx, Fetch{URL: f.URL, Dest: dest, Bytes: f.Bytes, SHA256: f.SHA256}, out)
		it := Item{Kind: "download", Role: f.Role, File: f.Name, Path: dest, Bytes: f.Bytes}
		switch {
		case err != nil:
			failures++
			if w.Failed != nil {
				w.Failed(err)
			}
			it.Status, it.Detail, it.Err = StatusFailed, failDetail(err), err
		case skipped:
			it.Status, it.Detail = StatusPresent, "already present and verified"
		default:
			it.Status, it.Detail = StatusDownloaded, "downloaded and SHA-256 verified"
		}
		res.add(it)
		if ctx.Err() != nil {
			return end(res, ctx.Err(), "")
		}
	}
	if err := s.mapFolders(res, targets); err != nil {
		return end(res, err, "")
	}
	if failures > 0 {
		return end(res, failf("%d of %d file(s) are not ready.", failures, len(files)), "")
	}
	// The downloader has verified every hash, so the check compares the size only.
	n, err := s.check(ctx, res, m, graphPath, req.Only, true)
	if err == nil && n > 0 {
		err = failf("The files are installed, but %d check(s) failed. See the lines above.", n)
	}
	return end(res, err, fmt.Sprintf("workflow '%s' is installed and ready", req.Workflow))
}

// mapFolders adds the download folders that ComfyUI does not read yet to
// extra_model_paths.yaml.
func (s *Service) mapFolders(res *Result, targets [][2]string) error {
	if len(targets) == 0 {
		return nil
	}
	own := map[string]string{}
	mapped := map[string][]string{}
	for _, t := range targets {
		own[t[0]] = s.files.Own(t[0])
		mapped[t[0]] = s.files.Mapped(t[0])
	}
	res.Mappings = mappings(targets, own, mapped)
	var add [][2]string
	for _, mp := range res.Mappings {
		if mp.Added {
			add = append(add, [2]string{mp.Folder, mp.Dir})
		}
	}
	return s.files.Map(add)
}

// mappings decides, for each download folder {folder, dir} in order, if
// extra_model_paths.yaml must map it: not a second time, not ComfyUI's own
// folder (own), and not when the file already maps it (mapped).
func mappings(targets [][2]string, own map[string]string, mapped map[string][]string) []Mapping {
	var out []Mapping
	seen := map[string]bool{}
	for _, t := range targets {
		folder, dir := t[0], t[1]
		if seen[strings.ToLower(dir)] {
			continue
		}
		seen[strings.ToLower(dir)] = true
		if strings.EqualFold(dir, own[folder]) {
			continue
		}
		added := true
		for _, d := range mapped[folder] {
			if strings.EqualFold(d, dir) {
				added = false
			}
		}
		out = append(out, Mapping{Folder: folder, Dir: dir, Added: added})
	}
	return out
}

// failDetail is the detail of a FAILED item: the reason lines of a
// *DownloadError, then "FAIL: " and the text of the error, joined by " | ".
func failDetail(err error) string {
	var why []string
	var de *DownloadError
	if errors.As(err, &de) {
		why = append(why, de.Reasons...)
	}
	return strings.Join(append(why, "FAIL: "+err.Error()), " | ")
}

// Remove deletes the model files of the workflow (Yes), or gives the plan.
// It refuses a file whose size differs from models.json, keeps a file that
// another workflow lists (unless Force), and also deletes an unfinished
// download (<file>.part).
func (s *Service) Remove(ctx context.Context, req Request) (*Result, error) {
	res, m, _, err := begin("remove", req)
	if err != nil {
		return end(res, err, "")
	}
	files, err := selectFiles(m.Files, req.Only)
	if err != nil {
		return end(res, err, "")
	}
	var plan []Item
	refused := 0
	for _, f := range files {
		p := s.find(f)
		for _, d := range s.files.Dirs(f.Folder) {
			if pp := filepath.Join(d, f.Name+".part"); s.files.Size(pp) >= 0 {
				plan = append(plan, Item{Kind: "file", Role: f.Role, File: f.Name + ".part", Path: pp, Bytes: s.files.Size(pp), Part: true})
			}
		}
		var size int64
		var users []string
		if p != "" {
			size = s.files.Size(p)
			if size == f.Bytes {
				if users, err = s.otherUsers(req, f); err != nil {
					return end(res, err, "")
				}
			}
		}
		it, del := removeDecision(f, p, size, users, req.Force)
		if del {
			plan = append(plan, it)
			continue
		}
		if it.Status == StatusRefused {
			refused++
		}
		res.add(it)
	}
	if len(plan) == 0 {
		if refused > 0 {
			return end(res, failf("Nothing deleted. %d file(s) refused.", refused), "")
		}
		return end(res, nil, "nothing to delete")
	}
	if !req.Yes {
		for _, x := range plan {
			x.Status = StatusWouldDelete
			if len(x.Users) > 0 {
				x.Detail = "also used by: " + strings.Join(x.Users, ", ")
			}
			res.add(x)
		}
		return end(res, nil, fmt.Sprintf("dry run: nothing deleted, %d file(s) would be deleted", len(plan)))
	}
	errs := 0
	for _, x := range plan {
		if err := s.files.Delete(x.Path); err != nil {
			errs++
			x.Status, x.Detail = StatusDelFailed, err.Error()
		} else {
			x.Status = StatusDeleted
		}
		res.add(x)
	}
	switch {
	case errs > 0:
		err = failf("%d file(s) were not deleted.", errs)
	case refused > 0:
		err = failf("Deleted %d file(s), but %d file(s) were refused. See the REFUSE lines.", len(plan), refused)
	}
	return end(res, err, fmt.Sprintf("deleted %d file(s)", len(plan)))
}

// removeDecision decides about one file of models.json: p is the path where
// it was found ("" = none), size its size, users the other workflows that
// list it. del is true when the file goes into the plan (it then has no
// status yet).
func removeDecision(f catalog.ListedFile, p string, size int64, users []string, force bool) (it Item, del bool) {
	it = Item{Kind: "file", Role: f.Role, File: f.Name, Path: p, Bytes: size, Users: users}
	switch {
	case p == "":
		it.Status, it.Bytes, it.Detail = StatusAbsent, f.Bytes, "not on disk"
	case size != f.Bytes:
		it.Status, it.Want = StatusRefused, f.Bytes
		it.Detail = fmt.Sprintf("the file has %d bytes, models.json says %d: it is not the listed file", size, f.Bytes)
	case len(users) > 0 && !force:
		it.Status, it.Detail = StatusKeptShared, "also used by: "+strings.Join(users, ", ")
	default:
		return it, true
	}
	return it, false
}

// otherUsers is Get-OtherUsers: the other workflows that list the same
// folder and file name.
func (s *Service) otherUsers(req Request, f catalog.ListedFile) ([]string, error) {
	ws, err := catalog.Folders(req.WorkflowsDir)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, w := range ws {
		if strings.EqualFold(w.Name, req.Workflow) || w.ModelsJSON == "" {
			continue
		}
		m, err := catalog.ReadModelList(w.ModelsJSON)
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
