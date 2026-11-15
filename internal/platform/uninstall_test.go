package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPathEntryOfTheInstallerIsRemoved: the pure edit removes the one entry
// of the installer and keeps every other entry byte for byte, and the OS
// part (osPathEntryCase) removes it from a scratch place: a scratch registry
// key on Windows (never HKCU\Environment), a scratch home folder elsewhere.
func TestPathEntryOfTheInstallerIsRemoved(t *testing.T) {
	dir := `C:\Users\u\AppData\Local\Programs\horus`
	for _, c := range []struct{ in, want string }{
		{`C:\a;` + dir, `C:\a`},
		{dir + `;C:\a`, `C:\a`},
		{`C:\a;` + dir + `\;%USERPROFILE%\go\bin;;D:\b `, `C:\a;%USERPROFILE%\go\bin;;D:\b `},
		{`C:\a;C:\USERS\U\APPDATA\LOCAL\PROGRAMS\HORUS;D:\b`, `C:\a;D:\b`},
		{dir, ``},
	} {
		got, changed := RemoveListEntry(c.in, dir)
		if !changed || got != c.want {
			t.Errorf("RemoveListEntry(%q) = %q, %v; want %q, true", c.in, got, changed, c.want)
		}
	}
	for _, in := range []string{`C:\a;C:\b`, `C:\a;` + dir + `x;C:\b`, ``} {
		if got, changed := RemoveListEntry(in, dir); changed || got != in {
			t.Errorf("RemoveListEntry(%q) = %q, %v; want no change", in, got, changed)
		}
	}

	line := `export PATH="/Users/u/.local/bin:$PATH" ` + ProfileMarker
	text := "# my profile\r\nexport A=1\n\n" + line + "\nalias l='ls'\n"
	got, changed := RemoveMarkedLines(text)
	if want := "# my profile\r\nexport A=1\n\nalias l='ls'\n"; !changed || got != want {
		t.Errorf("RemoveMarkedLines = %q, %v; want %q", got, changed, want)
	}
	if got, changed := RemoveMarkedLines("export A=1\n"); changed || got != "export A=1\n" {
		t.Errorf("a profile without the marker changed: %q", got)
	}

	osPathEntryCase(t)
}

// TestTheProgramFileRemovesItself: a copy of the test binary in a temporary
// folder calls RemoveSelf and ends. Its file is gone within 10 s. The copy is
// the only program that the test runs, so no other file can be the target.
func TestTheProgramFileRemovesItself(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	copyPath := filepath.Join(dir, ExeName("horus"))
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(copyPath)
	cmd.Env = append(os.Environ(), helperEnv+"=removeself")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the copy failed: %v %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); !strings.EqualFold(got, copyPath) {
		t.Fatalf("RemoveSelf named %q, want the copy %q", got, copyPath)
	}
	deadline := time.Now().Add(10 * time.Second)
	for IsFile(copyPath) {
		if time.Now().After(deadline) {
			t.Fatalf("%s is still there after 10 s", copyPath)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !IsFile(keep) {
		t.Error("another file of the folder was deleted")
	}
}

// TestRemoveTreeKeepsTheTargetOfALink: a folder with a link (a junction on
// Windows, a symbolic link elsewhere) to a folder outside it. RemoveTree
// removes the folder and the link, and the target keeps its file.
func TestRemoveTreeKeepsTheTargetOfALink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real-models")
	if err := os.MkdirAll(filepath.Join(target, "vae"), 0o755); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(target, "vae", "big.safetensors")
	if err := os.WriteFile(model, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(root, "install")
	if err := os.MkdirAll(filepath.Join(tree, "ComfyUI"), 0o755); err != nil {
		t.Fatal(err)
	}
	ro := filepath.Join(tree, "ComfyUI", "readonly.txt")
	if err := os.WriteFile(ro, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	makeLink(t, filepath.Join(tree, "ComfyUI", "models"), target)
	if !IsFile(filepath.Join(tree, "ComfyUI", "models", "vae", "big.safetensors")) {
		t.Fatal("the link does not reach the target")
	}
	if err := RemoveTree(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(tree); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}
	if !IsFile(model) {
		t.Fatal("RemoveTree deleted a file of the link target")
	}

	// A link given as the folder itself: only the link goes.
	link := filepath.Join(root, "link")
	makeLink(t, link, target)
	if err := RemoveTree(link); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("the link is still there: %v", err)
	}
	if !IsFile(model) {
		t.Fatal("RemoveTree of a link deleted a file of its target")
	}
	if err := RemoveTree(filepath.Join(root, "absent")); err != nil {
		t.Errorf("a missing folder: %v", err)
	}
}

func TestInstallRecordIsReadForItsKindOnly(t *testing.T) {
	dir := t.TempDir()
	if HasInstallRecord(dir, RecordComfyUI) {
		t.Fatal("an empty folder has a record")
	}
	if err := WriteInstallRecord(dir, RecordComfyUI); err != nil {
		t.Fatal(err)
	}
	if !HasInstallRecord(dir, RecordComfyUI) {
		t.Fatal("the record that was written is not read")
	}
	if HasInstallRecord(dir, RecordComfyMCP) {
		t.Error("a record of ComfyUI counts as a record of the venv")
	}
	if err := os.WriteFile(filepath.Join(dir, InstallRecordName), []byte(`{"program":"other","kind":"comfyui"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasInstallRecord(dir, RecordComfyUI) {
		t.Error("a record of another program counts")
	}
}
