// The check action. Ports Invoke-Check of scripts/workflow.ps1.
package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// check prints one line per check and adds one item per line. It puts the
// number of failures in r.checkFailures.
func (r *runner) check(m manifest, graphPath string) error {
	r.checkFailures = 0
	files, err := r.selectFiles(m.Files)
	if err != nil {
		return err
	}
	quick := ""
	if r.quick {
		quick = ", size only"
	}
	r.say("Checking workflow '%s': %d model file(s)%s.", r.workflow, len(files), quick)
	for _, f := range files {
		p := r.find(f)
		if p == "" {
			dirs := strings.Join(r.modelDirs(f.Folder), "; ")
			r.say("MISSING   %s %s (not in: %s)", f.Role, f.Name, dirs)
			r.checkFailures++
			r.add("MISSING", "file", f.Role, f.Name, "", f.Bytes, "not in: "+dirs)
			continue
		}
		n := fileSize(p)
		if n != f.Bytes {
			r.say("BAD SIZE  %s %s has %d bytes, expected %d", f.Role, p, n, f.Bytes)
			r.checkFailures++
			r.add("BAD_SIZE", "file", f.Role, f.Name, p, n, fmt.Sprintf("expected %d bytes", f.Bytes))
			continue
		}
		if !r.quick {
			got, err := fileSHA256(p)
			if err != nil {
				return err
			}
			if got != f.Sha256 {
				r.say("BAD HASH  %s %s SHA-256 is %s, expected %s", f.Role, p, got, f.Sha256)
				r.checkFailures++
				r.add("BAD_HASH", "file", f.Role, f.Name, p, n, fmt.Sprintf("SHA-256 is %s, expected %s", got, f.Sha256))
				continue
			}
			r.say("OK        %s %s (%d bytes, SHA-256 verified)", f.Role, p, n)
			r.add("OK", "file", f.Role, f.Name, p, n, "size and SHA-256 verified")
			continue
		}
		r.say("OK        %s %s (%d bytes)", f.Role, p, n)
		r.add("OK", "file", f.Role, f.Name, p, n, "size verified")
	}

	if !isFile(graphPath) {
		r.say("SKIP      graph checks: no workflow.json")
		r.add("SKIP", "graph", "", "", "", -1, "no workflow.json")
		return nil
	}
	graph, err := readJSON(graphPath)
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, f := range m.Files {
		listed[strings.ToLower(f.Name)] = true
	}
	var wanted []string
	allStrings(graph, func(s string) {
		if modelExt.MatchString(s) {
			wanted = append(wanted, leaf(strings.ReplaceAll(s, `\`, "/")))
		}
	})
	wanted = uniqueStrings(wanted)
	unlisted := 0
	for _, w := range wanted {
		if listed[strings.ToLower(w)] {
			continue
		}
		unlisted++
		r.say("UNLISTED  workflow.json loads %s, but models.json does not list it. Add it to models.json.", w)
		r.checkFailures++
		r.add("UNLISTED", "graph", "", w, "", -1, "workflow.json loads this file, but models.json does not list it")
	}
	if unlisted == 0 {
		r.say("OK        models.json lists all %d model file(s) that workflow.json loads", len(wanted))
		r.add("OK", "graph", "", "", "", -1, fmt.Sprintf("models.json lists all %d model files of workflow.json", len(wanted)))
	}

	info, ok := r.objectInfo()
	if !ok {
		r.say("SKIP      server checks: no answer at %s (start ComfyUI with horus start-comfyui)", r.server)
		r.add("SKIP", "server", "", "", "", -1, "no answer at "+r.server)
		return nil
	}
	types := nodeTypes(graph)
	missing := 0
	for _, t := range types {
		if o, isObj := info.(*object); isObj && o.has(t) {
			continue
		}
		missing++
		r.say("NO NODE   the server has no node class '%s'. Install its node pack (MCP install_node), then restart ComfyUI.", t)
		r.checkFailures++
		r.add("NO_NODE", "server", "", t, "", -1, fmt.Sprintf("the server has no node class '%s'", t))
	}
	if missing == 0 {
		r.say("OK        the server has all %d node classes of workflow.json", len(types))
		r.add("OK", "server", "", "", "", -1, fmt.Sprintf("the server has all %d node classes", len(types)))
	}
	visible := map[string]bool{}
	allStrings(info, func(s string) {
		if modelExt.MatchString(s) {
			visible[strings.ToLower(leaf(strings.ReplaceAll(s, `\`, "/")))] = true
		}
	})
	for _, f := range files {
		if visible[strings.ToLower(f.Name)] {
			r.say("OK        the server lists %s", f.Name)
			r.add("OK", "server", f.Role, f.Name, "", -1, "the server lists the file")
		} else {
			r.say("NOT SEEN  the server does not list %s. Check extra_model_paths.yaml, then restart ComfyUI.", f.Name)
			r.checkFailures++
			r.add("NOT_SEEN", "server", f.Role, f.Name, "", -1, "the server does not list the file")
		}
	}
	return nil
}

// objectInfo asks ComfyUI for /object_info, as Invoke-RestMethod does. ok is
// false when no answer came or the answer was an HTTP error. A body that is not
// JSON is one string, as Invoke-RestMethod returns it.
func (r *runner) objectInfo() (any, bool) {
	req, err := newGet(r.ctx, r.server+"/object_info")
	if err != nil {
		return nil, false
	}
	resp, err := r.opt.HTTPClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	if v, err := parseJSON(body); err == nil {
		return v, true
	}
	return string(body), true
}
