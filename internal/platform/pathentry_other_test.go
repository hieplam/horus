//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// osPathEntryCase: in a scratch home folder, PathEntry finds the profile
// file with the marked line, and RemovePathEntry removes that line only.
func osPathEntryCase(t *testing.T) {
	home := t.TempDir()
	zp := filepath.Join(home, ".zprofile")
	line := `export PATH="` + home + `/.local/bin:$PATH" ` + ProfileMarker
	if err := os.WriteFile(zp, []byte("export A=1\n\n"+line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(home, ".profile")
	if err := os.WriteFile(other, []byte("export B=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	where, err := PathEntry(filepath.Join(home, ".local", "bin", "horus"), home)
	if err != nil || len(where) != 1 || where[0] != zp {
		t.Fatalf("PathEntry = %v, %v; want [%s]", where, err, zp)
	}
	if err := RemovePathEntry(filepath.Join(home, ".local", "bin", "horus"), home); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(zp)
	if string(data) != "export A=1\n\n" {
		t.Errorf(".zprofile = %q", data)
	}
	if fi, _ := os.Stat(zp); fi.Mode().Perm() != 0o600 {
		t.Errorf("the mode of .zprofile changed: %v", fi.Mode())
	}
	if data, _ := os.ReadFile(other); string(data) != "export B=2\n" {
		t.Errorf(".profile changed: %q", data)
	}
	if where, _ := PathEntry("", home); len(where) != 0 {
		t.Errorf("PathEntry after the removal = %v", where)
	}
}

// makeLink makes the symbolic link link to the folder target.
func makeLink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
