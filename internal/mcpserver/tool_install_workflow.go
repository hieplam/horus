// The tool install_workflow. Part of the port of horus/server.py.
package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/models"
)

// InstallResult is the result of install_workflow, in the key order of server.py.
type InstallResult struct {
	Workflow         string        `json:"workflow"`
	InstallState     string        `json:"install_state"`
	AlreadyInstalled bool          `json:"already_installed"`
	Files            []InstallFile `json:"files"`
	Seconds          float64       `json:"seconds"`
}

func installWorkflow() tool {
	return tool{
		name:  "install_workflow",
		title: "Install an image workflow",
		description: "Download and verify (SHA-256) the model files of one image workflow, so that generate_image can use it. " +
			"Files that are already present are kept. Downloads are large (several GB) and can take 10 to 60 minutes. " +
			"The models have their own license: the first call without accept_license=true returns the error " +
			"LICENSE_CONFIRMATION_REQUIRED with the license and the size. Ask the user, then call again with " +
			"accept_license=true. The call waits until the install is done and reports progress.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(true)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows."),
			f("accept_license", tBool).d("True only after the user accepted the license shown in the first reply.").dflt(false),
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
			return install(ctx, c)
		},
	}
}

// ProgressInterval is the time between two progress notifications of an
// install, as asyncio.wait(timeout=5) of server.py. Tests make it shorter.
var ProgressInterval = 5 * time.Second

// sizeText is _size of server.py.
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

func install(ctx context.Context, c *call) (any, error) {
	t0 := time.Now()
	w, err := catalog.Get(c.args.str("workflow"))
	if err != nil {
		return nil, err
	}
	status, err := models.ListStatus(ctx)
	if err != nil {
		return nil, err
	}
	if row, ok := status[w.Name]; ok && models.InstallState(&row) == "installed" {
		res, err := models.Check(ctx, w, true)
		if err != nil {
			return nil, err
		}
		if res.OK {
			files := fileRows(res.Items, "file", map[string]string{"OK": "present"})
			return InstallResult{Workflow: w.Name, InstallState: "installed", AlreadyInstalled: true, Files: files, Seconds: seconds(t0)}, nil
		}
	}
	lic, err := w.License()
	if err != nil {
		return nil, err
	}
	if !c.args.boolean("accept_license") {
		m, err := w.Models()
		if err != nil {
			return nil, err
		}
		files := []licenseFile{}
		for _, f := range m.Files {
			parts := strings.Split(f.RepoPath, "/")
			files = append(files, licenseFile{Role: f.Role, File: parts[len(parts)-1], Bytes: f.Bytes})
		}
		return nil, herr.New("LICENSE_CONFIRMATION_REQUIRED",
			fmt.Sprintf("installing %s downloads %s under the license '%s'", w.Name, sizeText(lic.DownloadBytes), lic.Name),
			map[string]any{"license": lic.Name, "license_url": lic.URL, "download_bytes": lic.DownloadBytes, "files": files})
	}
	total := float64(lic.DownloadBytes)
	run := &models.Run{}
	type outcome struct {
		res *models.Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := models.Install(ctx, w, run)
		done <- outcome{res, err}
	}()
	tick := time.NewTicker(ProgressInterval)
	defer tick.Stop()
	for {
		select {
		case o := <-done:
			// A cancel of the call ends ctx: models.Install kills the child and
			// returns the error of ctx. The .part file stays.
			if o.err != nil {
				return nil, o.err
			}
			files := fileRows(o.res.Items, "download", map[string]string{"DOWNLOADED": "downloaded", "PRESENT": "present"})
			return InstallResult{Workflow: w.Name, InstallState: "installed", AlreadyInstalled: false, Files: files, Seconds: seconds(t0)}, nil
		case <-tick.C:
			dest := run.CurrentDest()
			n := models.PartSize(dest)
			parts := strings.Split(strings.ReplaceAll(dest, "\\", "/"), "/")
			name := parts[len(parts)-1]
			msg := "checking files"
			if name != "" {
				msg = fmt.Sprintf("downloading %s: %.2f GB", name, float64(n)/1e9)
			}
			c.report(ctx, float64(n), total, msg)
		}
	}
}
