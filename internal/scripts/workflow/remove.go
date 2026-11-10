// The remove action. Ports the remove branch of scripts/workflow.ps1.
package workflow

import (
	"fmt"
	"os"
	"strings"
)

type planEntry struct {
	Path        string
	Bytes       int64
	Label, Role string
	Name, Users string
}

func (r *runner) remove(m manifest) error {
	files, err := r.selectFiles(m.Files)
	if err != nil {
		return err
	}
	var plan []planEntry
	refused := 0
	for _, f := range files {
		p := r.find(f)
		for _, d := range r.modelDirs(f.Folder) {
			if pp := joinPath(d, f.Name+".part"); isFile(pp) {
				plan = append(plan, planEntry{pp, fileSize(pp), "unfinished download of " + f.Name, f.Role, f.Name + ".part", ""})
			}
		}
		if p == "" {
			r.say("ABSENT    %s %s", f.Role, f.Name)
			r.add("ABSENT", "file", f.Role, f.Name, "", f.Bytes, "not on disk")
			continue
		}
		n := fileSize(p)
		if n != f.Bytes {
			r.say("REFUSE    %s %s has %d bytes, models.json says %d. It is not the listed file. Delete it yourself.", f.Role, p, n, f.Bytes)
			refused++
			r.add("REFUSED", "file", f.Role, f.Name, p, n, fmt.Sprintf("the file has %d bytes, models.json says %d: it is not the listed file", n, f.Bytes))
			continue
		}
		users, err := r.otherUsers(f)
		if err != nil {
			return err
		}
		also := strings.Join(users, ", ")
		if len(users) > 0 && !r.force {
			r.say("KEEP      %s %s (also used by: %s; -force deletes it)", f.Role, p, also)
			r.add("KEPT_SHARED", "file", f.Role, f.Name, p, n, "also used by: "+also)
			continue
		}
		label := f.Role
		if len(users) > 0 {
			label += " (also used by: " + also + ")"
		}
		plan = append(plan, planEntry{p, n, label, f.Role, f.Name, also})
	}
	var total int64
	for _, x := range plan {
		total += x.Bytes
	}
	if len(plan) == 0 {
		if refused > 0 {
			return failf("Nothing deleted. %d file(s) refused.", refused)
		}
		r.say("Nothing to delete for workflow '%s'.", r.workflow)
		r.writeResult(true, "nothing to delete")
		return nil
	}
	verb := "WOULD DEL"
	if r.yes {
		verb = "DELETE   "
	}
	for _, x := range plan {
		r.say("%s  %s %s (%s GB)", verb, x.Label, x.Path, gb2(x.Bytes))
	}
	if !r.yes {
		for _, x := range plan {
			detail := ""
			if x.Users != "" {
				detail = "also used by: " + x.Users
			}
			r.add("WOULD_DELETE", "file", x.Role, x.Name, x.Path, x.Bytes, detail)
		}
		r.say("Dry run: nothing deleted. %s GB in %d file(s). Add -yes to delete.", gb2(total), len(plan))
		r.writeResult(true, fmt.Sprintf("dry run: nothing deleted, %d file(s) would be deleted", len(plan)))
		return nil
	}
	errs := 0
	for _, x := range plan {
		if err := removeFile(x.Path); err != nil {
			r.say("FAIL      cannot delete %s: %v (Is ComfyUI using it? Stop ComfyUI and try again.)", x.Path, err)
			errs++
			r.add("DELETE_FAILED", "file", x.Role, x.Name, x.Path, x.Bytes, err.Error())
			continue
		}
		r.add("DELETED", "file", x.Role, x.Name, x.Path, x.Bytes, "")
	}
	if errs > 0 {
		return failf("%d file(s) were not deleted.", errs)
	}
	if refused > 0 {
		return failf("Deleted %d file(s), but %d file(s) were refused. See the REFUSE lines.", len(plan), refused)
	}
	r.say("OK: deleted %d file(s), %s GB. Reinstall: horus workflow install %s", len(plan), gb2(total), r.workflow)
	r.writeResult(true, fmt.Sprintf("deleted %d file(s)", len(plan)))
	return nil
}

// removeFile is Remove-Item -Force: it also deletes a read-only file.
func removeFile(p string) error {
	err := os.Remove(p)
	if err == nil {
		return nil
	}
	if fi, serr := os.Stat(p); serr == nil && fi.Mode().Perm()&0o200 == 0 {
		if os.Chmod(p, fi.Mode().Perm()|0o200) == nil {
			if err2 := os.Remove(p); err2 == nil {
				return nil
			}
		}
	}
	return err
}
