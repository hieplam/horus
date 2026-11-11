// The typed results of the service.
package provision

import (
	"fmt"
	"math"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
)

// The statuses of an item (17). The kind of an item is "file", "graph",
// "server" or "download".
const (
	StatusOK          = "OK"           // check: the file, the graph or the server is right
	StatusMissing     = "MISSING"      // check: the file is in no folder
	StatusBadSize     = "BAD_SIZE"     // check: the file has another size
	StatusBadHash     = "BAD_HASH"     // check: the file has another SHA-256
	StatusSkip        = "SKIP"         // check: no workflow.json, or no answer from the server
	StatusUnlisted    = "UNLISTED"     // check: workflow.json loads a file that models.json does not list
	StatusNoNode      = "NO_NODE"      // check: the server has no node class of the graph
	StatusNotSeen     = "NOT_SEEN"     // check: the server does not list the file
	StatusDownloaded  = "DOWNLOADED"   // install: downloaded and verified
	StatusPresent     = "PRESENT"      // install: already present and verified
	StatusFailed      = "FAILED"       // install: the download failed
	StatusAbsent      = "ABSENT"       // remove: not on disk
	StatusRefused     = "REFUSED"      // remove: the file on disk is not the listed file
	StatusKeptShared  = "KEPT_SHARED"  // remove: another workflow lists the file
	StatusWouldDelete = "WOULD_DELETE" // remove without Yes: the plan
	StatusDeleted     = "DELETED"      // remove: deleted
	StatusDelFailed   = "DELETE_FAILED"
)

// Item is one check or one file of a result. The fields with a JSON name
// are the item of the HORUS_JSON line of horus workflow -json. The other
// fields are for the text of a handler and for the error codes of the MCP
// tools.
type Item struct {
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	File   string `json:"file"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Detail string `json:"detail"`

	// Want is the size that models.json says (BAD_SIZE, REFUSED).
	Want int64 `json:"-"`
	// Count is the number of model files of workflow.json (OK graph) or of
	// node classes (OK server, with no file).
	Count int `json:"-"`
	// Users are the other workflows that list the same file (KEPT_SHARED,
	// and a file of the plan of a remove).
	Users []string `json:"-"`
	// Part is true for an unfinished download (<file>.part) in the plan of a
	// remove.
	Part bool `json:"-"`
	// Err is the error of the downloader (FAILED), a *DownloadError when the
	// downloader gave one.
	Err error `json:"-"`
}

// Mapping is one download folder that an install checked against
// extra_model_paths.yaml.
type Mapping struct {
	Folder string
	Dir    string // a full path
	Added  bool   // false: the file already maps the folder
}

// Result is the result of check, install or remove. The fields with a JSON
// name are the HORUS_JSON line of horus workflow -json.
type Result struct {
	OK       bool   `json:"ok"`
	Action   string `json:"action"`
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
	Items    []Item `json:"items"`

	// Checked is true when the check part ran (also in an install).
	// CheckFiles is the number of files that it checked, and Quick is true
	// when it compared the size only.
	Checked    bool `json:"-"`
	CheckFiles int  `json:"-"`
	Quick      bool `json:"-"`
	// Mappings are the download folders of an install that are outside
	// ComfyUI's own models folder, in order.
	Mappings []Mapping `json:"-"`
}

func (r *Result) add(it Item) { r.Items = append(r.Items, it) }

// Row is one workflow of List. The JSON names are the rows of
// horus workflow list -json.
type Row struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Files       int     `json:"files"`
	Present     int     `json:"present"`
	Bytes       int64   `json:"bytes"`
	Gigabytes   float64 `json:"gigabytes"`
	Installed   string  `json:"installed"`
	Roles       string  `json:"roles"`
}

// State is the install state of the MCP tools for the row (InstallState). A
// nil row is "not_installed".
func (r *Row) State() string {
	if r == nil {
		return InstallState(0, 0)
	}
	return InstallState(r.Present, r.Files)
}

// InstallState is the install state of the MCP tools: "installed" when all
// files of a workflow are present, "partially_installed" when some are, and
// "not_installed" when none are or the workflow has no files. It is the one
// place of this rule, for list_workflows and describe_workflow.
func InstallState(present, files int) string {
	switch {
	case files == 0:
		return "not_installed"
	case present == files:
		return "installed"
	case present > 0:
		return "partially_installed"
	}
	return "not_installed"
}

// installedText is the column "Installed" of list.
func installedText(present, files int) string {
	switch {
	case files == 0:
		return "no models"
	case present == files:
		return "yes"
	}
	return fmt.Sprintf("no (%d of %d files)", present, files)
}

// roundGB is [math]::Round($bytes / 1e9, 2).
func roundGB(b int64) float64 { return math.RoundToEven(float64(b)/1e9*100) / 100 }

// selectFiles is Select-Files: the files whose role or name is in only.
func selectFiles(files []catalog.ListedFile, only []string) ([]catalog.ListedFile, error) {
	if len(only) == 0 {
		return files, nil
	}
	var picked []catalog.ListedFile
	for _, f := range files {
		for _, o := range only {
			if strings.EqualFold(o, f.Role) || strings.EqualFold(o, f.Name) {
				picked = append(picked, f)
				break
			}
		}
	}
	if len(picked) == 0 {
		var roles []string
		seen := map[string]bool{}
		for _, f := range files {
			if !seen[f.Role] {
				seen[f.Role] = true
				roles = append(roles, f.Role)
			}
		}
		return nil, failf("-only %s matches no file. Roles: %s", strings.Join(only, ","), strings.Join(roles, ", "))
	}
	return picked, nil
}
