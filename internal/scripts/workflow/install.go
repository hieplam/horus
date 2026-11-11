// The install action. Ports the install branch and Add-YamlMappings of
// scripts/workflow.ps1. The download itself is download.Download (task G3),
// which replaces download-model.ps1.
package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hieplam/horus/internal/download"
)

func newGet(ctx context.Context, url string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}

// whyLine picks the download lines that go into the detail of a FAILED item.
// workflow.ps1 kept the lines that start with "FAIL:", "Attempt " and
// "curl exit 22" (the hint line about HTTP 401 and 403). The Go downloader has
// no curl: its hint line starts with "The server answered with an error".
var whyLine = regexp.MustCompile(`^(FAIL:|The server answered with an error|Attempt )`)

// lineTee writes to out and keeps every line.
type lineTee struct {
	out   interface{ Write([]byte) (int, error) }
	buf   bytes.Buffer
	lines []string
}

func (t *lineTee) Write(p []byte) (int, error) {
	t.buf.Write(p)
	for {
		i := bytes.IndexByte(t.buf.Bytes(), '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(t.buf.Next(i+1)), "\r\n")
		t.lines = append(t.lines, line)
		fmt.Fprintln(t.out, line)
	}
	return len(p), nil
}

func (t *lineTee) flush() {
	if t.buf.Len() > 0 {
		line := t.buf.String()
		t.buf.Reset()
		t.lines = append(t.lines, line)
		fmt.Fprintln(t.out, line)
	}
}

type target struct{ Folder, Dir string }

func (r *runner) install(m manifest, graph string) error {
	if !isFile(filepath.Join(r.comfyDir, "main.py")) {
		return failf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", r.comfyUIDir)
	}
	files, err := r.selectFiles(m.Files)
	if err != nil {
		return err
	}
	r.say("License: %s", m.License)
	r.say("Installing %d model file(s) for workflow '%s'.", len(files), r.workflow)
	failures := 0
	var newTargets []target
	for _, f := range files {
		dest := r.find(f)
		if dest == "" {
			dir := r.modelDirs(f.Folder)[0]
			dest = joinPath(dir, f.Name)
			newTargets = append(newTargets, target{f.Folder, dir})
		}
		r.say("-- %s -> %s", f.Role, dest)
		tee := &lineTee{out: r.out}
		_, err := r.opt.Download(r.ctx, download.Options{
			URL: f.URL, Dest: dest, ExpectedBytes: f.Bytes, SHA256: f.Sha256, Out: tee,
		})
		if err != nil {
			fmt.Fprintf(tee, "FAIL: %v\n", err)
		}
		tee.flush()
		switch {
		case err != nil:
			failures++
			var why []string
			for _, l := range tee.lines {
				if whyLine.MatchString(l) {
					why = append(why, l)
				}
			}
			r.add("FAILED", "download", f.Role, f.Name, dest, f.Bytes, strings.Join(why, " | "))
		case skipped(tee.lines):
			r.add("PRESENT", "download", f.Role, f.Name, dest, f.Bytes, "already present and verified")
		default:
			r.add("DOWNLOADED", "download", f.Role, f.Name, dest, f.Bytes, "downloaded and SHA-256 verified")
		}
	}
	if len(newTargets) > 0 {
		if err := r.addYamlMappings(newTargets); err != nil {
			return err
		}
	}
	if failures > 0 {
		return failf("%d of %d file(s) are not ready.", failures, len(files))
	}
	r.say("")
	r.quick = true // the downloader has verified every hash
	if err := r.check(m, graph); err != nil {
		return err
	}
	if r.checkFailures > 0 {
		return failf("The files are installed, but %d check(s) failed. See the lines above.", r.checkFailures)
	}
	r.say("OK: workflow '%s' is installed and ready.", r.workflow)
	r.writeResult(true, fmt.Sprintf("workflow '%s' is installed and ready", r.workflow))
	return nil
}

// skipped: a line that starts with "SKIP " means the file was already there.
func skipped(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "SKIP ") {
			return true
		}
	}
	return false
}

// addYamlMappings maps the download folders that are outside ComfyUI's own
// models folder in extra_model_paths.yaml. It makes a backup of the file first.
func (r *runner) addYamlMappings(targets []target) error {
	yamlPath := filepath.Join(r.comfyDir, "extra_model_paths.yaml")
	text := ""
	exists := false
	if data, err := os.ReadFile(yamlPath); err == nil {
		text, exists = string(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))), true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	changed, backedUp := false, false
	seen := map[string]bool{}
	for _, t := range targets {
		full := fullPath(t.Dir)
		if seen[strings.ToLower(full)] {
			continue
		}
		seen[strings.ToLower(full)] = true
		if strings.EqualFold(full, fullPath(joinPath(r.defaultModels, t.Folder))) {
			continue
		}
		mapped := false
		for _, d := range r.yaml[t.Folder] {
			if strings.EqualFold(d, full) {
				mapped = true
			}
		}
		if mapped {
			r.say("extra_model_paths.yaml already maps %s -> %s", t.Folder, full)
			continue
		}
		if exists && !backedUp {
			if err := copyFile(yamlPath, yamlPath+".bak-"+r.opt.Now().Format("20060102-150405")); err != nil {
				return err
			}
			backedUp = true
		}
		base := strings.ReplaceAll(filepath.Dir(full), `\`, "/")
		name := filepath.Base(full)
		text = strings.TrimLeftFunc(strings.TrimRightFunc(text, isSpace)+
			"\n\nhorus_"+t.Folder+":\n    base_path: "+base+"\n    "+t.Folder+": "+name+"\n", isSpace)
		changed = true
		r.say("extra_model_paths.yaml: added %s -> %s/%s", t.Folder, base, name)
	}
	if changed {
		if err := os.WriteFile(yamlPath, []byte(text), 0o644); err != nil {
			return err
		}
		r.yaml = yamlFolders(yamlPath)
		r.say("Restart ComfyUI so that it reads the new paths (horus start-comfyui).")
	}
	return nil
}

// copyFile is Copy-Item: it writes a copy, and replaces an older copy of the same name.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
