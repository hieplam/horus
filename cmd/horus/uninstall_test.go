// The test of horus uninstall through the wiring of cmd/horus: horus setup
// with the fakes of setup_packages_test.go, then horus uninstall, on the
// sandbox. The home folder, the user folder, the repo root, LOCALAPPDATA,
// APPDATA, CODEX_HOME and the PATH are in temporary folders. The PATH holds
// only fake programs. The PATH entry and the program file are a fake of the
// OS (the seam installer), so the test never reads or writes the registry and
// never deletes a real program.
package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// envClientRecord is the file where the fake claude and the fake codex write
// one line for each run.
const envClientRecord = "HORUS_FAKE_CLIENT_RECORD"

// fakeMCPClient is the fake claude or codex. It writes "<name> <args>" to
// the record. "mcp get horus" exits 0 until "mcp remove" ran for that name.
func fakeMCPClient(exe string, args []string) int {
	rec := os.Getenv(envClientRecord)
	name := strings.TrimSuffix(filepath.Base(exe), ".exe")
	if f, err := os.OpenFile(rec, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(name + " " + strings.Join(args, " ") + "\n")
		f.Close()
	}
	removed := rec + "." + name + ".removed"
	switch {
	case len(args) > 1 && args[1] == "remove":
		os.WriteFile(removed, nil, 0o644)
		return 0
	case len(args) > 1 && args[1] == "get":
		if platform.IsFile(removed) {
			return 1
		}
		return 0
	}
	return 0
}

// sandboxOS is the OS of the sandbox: the PATH entry is the marked line of a
// profile file of the sandbox, and the program is a file of the sandbox.
type sandboxOS struct{ profile, program string }

func (o sandboxOS) Program() (string, error) { return o.program, nil }
func (o sandboxOS) PathEntry(string) ([]string, error) {
	b, _ := os.ReadFile(o.profile)
	if _, changed := platform.RemoveMarkedLines(string(b)); changed {
		return []string{o.profile}, nil
	}
	return nil, nil
}
func (o sandboxOS) RemovePathEntry(string) error {
	b, err := os.ReadFile(o.profile)
	if err != nil {
		return err
	}
	out, _ := platform.RemoveMarkedLines(string(b))
	return os.WriteFile(o.profile, []byte(out), 0o644)
}
func (o sandboxOS) RemoveProgram() error { return os.Remove(o.program) }

// insideAny reports if p is in one of the folders roots.
func insideAny(roots []string, p string) bool {
	for _, r := range roots {
		rel, err := filepath.Rel(r, p)
		if err == nil && filepath.IsAbs(p) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// planInsideSandbox fails the test when a path of the plan is not in the
// sandbox folders roots. A test calls it before it runs uninstall -yes.
func planInsideSandbox(t *testing.T, roots []string, u *provision.Uninstall) {
	t.Helper()
	for _, p := range u.Paths() {
		if !insideAny(roots, p) {
			t.Fatalf("the plan names %s, outside the sandbox %v", p, roots)
		}
	}
}

// linkTo makes link a link to the folder target: a symbolic link, or an NTFS
// junction when this system refuses a symbolic link (Windows without the
// right). The test installs of the owner use such junctions.
func linkTo(t *testing.T, link, target string) {
	t.Helper()
	if os.Symlink(target, link) == nil {
		return
	}
	shell := os.Getenv("ComSpec") // the PATH of the test holds only fakes
	if shell == "" {
		shell = "cmd"
	}
	if out, err := exec.Command(shell, "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("no link: %v %s", err, out)
	}
}

// files lists every file under root, as paths from root with slashes.
func files(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	must(t, err)
	sort.Strings(out)
	return out
}

// TestSetupThenUninstallLeavesOnlyTheKeptItems: horus setup demo -yes
// -with-packages installs the system package, ComfyUI (with its install
// record), the venv on macOS, and the model files. The clients know the
// server horus. Then horus uninstall without -yes changes no file, and with
// -yes the home folder holds only .env, the image of outputs/ and the
// profile without the marked line. The system package stays, the fake
// clients got "mcp remove", and the target of a junction in the ComfyUI
// folder keeps its file.
func TestSetupThenUninstallLeavesOnlyTheKeptItems(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newPkgRig(t, goos, true)
			home := filepath.Join(r.tmp, "home")
			// The record of the fake clients is outside the folders that the
			// test compares: "mcp get" of the plan writes to it.
			clients := filepath.Join(t.TempDir(), "clients.record")
			user := filepath.Join(home, "horus")
			for k, v := range map[string]string{"HORUS_HOME": user, "LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
				"APPDATA": filepath.Join(home, "AppData", "Roaming"), "CODEX_HOME": filepath.Join(home, ".codex"),
				envClientRecord: clients} {
				t.Setenv(k, v)
			}
			settings.Reset()
			exe, err := os.Executable()
			must(t, err)
			bin := filepath.Dir(r.manager)
			for _, c := range []string{"claude", "codex"} {
				must(t, copyFile(exe, filepath.Join(bin, platform.ExeName(c))))
			}

			code, out := r.s.runSetup(t, r.sm, "demo", "-yes", "-with-packages")
			if code != 0 {
				t.Fatalf("setup: exit %d:\n%s", code, out)
			}
			// The user folder: the copy of the built-in workflows, and the data of the user.
			settings.BuiltinWorkflowsDir()
			must(t, os.WriteFile(filepath.Join(user, ".env"), []byte("HORUS_AUTOSTART=0\n"), 0o644))
			must(t, os.MkdirAll(filepath.Join(user, "outputs"), 0o755))
			must(t, os.WriteFile(filepath.Join(user, "outputs", "a.png"), []byte("png"), 0o644))
			// The program and the PATH entry of the install script.
			program := filepath.Join(home, "AppData", "Local", "Programs", "horus", platform.ExeName("horus"))
			must(t, os.MkdirAll(filepath.Dir(program), 0o755))
			must(t, os.WriteFile(program, []byte("program"), 0o755))
			profile := filepath.Join(home, ".zprofile")
			must(t, os.WriteFile(profile, []byte("export A=1\n\nexport PATH=\"x:$PATH\" "+platform.ProfileMarker+"\n"), 0o644))
			// A junction in the ComfyUI folder to model files outside it.
			outside := filepath.Join(r.tmp, "outside-models")
			must(t, os.MkdirAll(outside, 0o755))
			keep := filepath.Join(outside, "keep.safetensors")
			must(t, os.WriteFile(keep, []byte("model of a person"), 0o644))
			linkTo(t, filepath.Join(r.s.comfy, "ComfyUI", "loras"), outside)

			sm := r.sm
			sm.installer = sandboxOS{profile: profile, program: program}
			place := cli.SetupPlace{ComfyUIDir: r.s.comfy, ModelsDir: r.s.models, ServerURL: "http://127.0.0.1:9"}
			args := []string{"-workflows-dir", r.s.workflows, "-comfyui-dir", r.s.comfy, "-models-dir", r.s.models, "-server-url", place.ServerURL}
			at := func(p cli.SetupPlace) cli.Uninstaller { return newUninstall(p, sm) }

			// The rule of the spec: every path of the plan is in the sandbox.
			u, err := newUninstall(place, sm).UninstallPlan(context.Background(), provision.UninstallRequest{WorkflowsDir: r.s.workflows})
			must(t, err)
			planInsideSandbox(t, []string{r.s.root, r.tmp}, u)
			if u.Removes() < 8 {
				t.Fatalf("only %d REMOVE items", u.Removes())
			}

			before := r.state(t)
			var buf bytes.Buffer
			if code := cli.RunUninstall(context.Background(), args, &buf, &buf, at); code != 0 || !strings.Contains(buf.String(), "Nothing changed.") {
				t.Fatalf("uninstall: exit %d:\n%s", code, buf.String())
			}
			if after := r.state(t); after != before {
				t.Fatalf("uninstall without -yes changed files.\nbefore:\n%s\nafter:\n%s", before, after)
			}

			buf.Reset()
			if code := cli.RunUninstall(context.Background(), append(args, "-yes"), &buf, &buf, at); code != 0 {
				t.Fatalf("uninstall -yes: exit %d:\n%s", code, buf.String())
			}
			if got, want := strings.Join(files(t, home), "\n"), ".zprofile\nhorus/.env\nhorus/outputs/a.png"; got != want {
				t.Errorf("the home folder holds:\n%s\nwant:\n%s\noutput:\n%s", got, want, buf.String())
			}
			if b, _ := os.ReadFile(profile); string(b) != "export A=1\n\n" {
				t.Errorf("the profile = %q", b)
			}
			if !platform.IsFile(r.put) {
				t.Errorf("the system package %s was removed", r.put)
			}
			if !platform.IsFile(keep) {
				t.Error("the target of the junction lost its file")
			}
			if _, err := os.Lstat(filepath.Join(r.s.comfy, "ComfyUI")); !os.IsNotExist(err) {
				t.Errorf("the ComfyUI folder is still there: %v", err)
			}
			if fs := files(t, r.s.models); len(fs) != 0 {
				t.Errorf("model files left: %v", fs)
			}
			rec, _ := os.ReadFile(clients)
			for _, w := range []string{"claude mcp remove --scope user horus\n", "codex mcp remove horus\n"} {
				if !strings.Contains(string(rec), w) {
					t.Errorf("the clients did not get %q:\n%s", w, rec)
				}
			}
		})
	}
}
