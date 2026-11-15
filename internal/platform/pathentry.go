// The PATH entry of the install scripts (install.ps1, install.sh), and the
// removal of the running program. horus uninstall uses them.
//
//   - Windows: install.ps1 appends the install folder to the value Path of
//     the registry key HKCU\Environment. RemovePathEntry removes that one
//     entry and keeps every other entry byte for byte.
//   - macOS: install.sh writes one line that ends with ProfileMarker into a
//     profile file of the shell. RemovePathEntry removes each line with the
//     marker from the three profile files, and keeps every other line.
//
// The edit of the value is a pure function (RemoveListEntry,
// RemoveMarkedLines). Only a small function of each OS reads and writes it.
package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// ProfileMarker is the end of the line that install.sh writes into a profile
// file (docs/plans/2026-11-15-release-install-evidence.md, "## T3").
const ProfileMarker = "# added by the horus installer"

// ProfileFiles are the profile files where install.sh can write its line,
// in the home folder home.
func ProfileFiles(home string) []string {
	return []string{filepath.Join(home, ".zprofile"), filepath.Join(home, ".bash_profile"), filepath.Join(home, ".profile")}
}

// sameFolder reports if two folder paths of a PATH list are the same folder
// for Windows: without case, and without a trailing backslash or slash.
func sameFolder(a, b string) bool {
	trim := func(s string) string { return strings.TrimRight(s, `\/`) }
	return a != "" && strings.EqualFold(trim(a), trim(b))
}

// RemoveListEntry removes each entry that is the folder dir (sameFolder)
// from the ';' list value. Every other entry, also an empty one, stays byte
// for byte and in its order. changed is false when no entry is dir; value
// then comes back as it was. It is a pure function.
func RemoveListEntry(value, dir string) (out string, changed bool) {
	parts := strings.Split(value, ";")
	kept := parts[:0:0]
	for _, p := range parts {
		if sameFolder(p, dir) {
			changed = true
			continue
		}
		kept = append(kept, p)
	}
	if !changed {
		return value, false
	}
	return strings.Join(kept, ";"), true
}

// HasListEntry reports if the ';' list value has an entry that is the
// folder dir (sameFolder). It is a pure function.
func HasListEntry(value, dir string) bool {
	_, changed := RemoveListEntry(value, dir)
	return changed
}

// RemoveMarkedLines removes each line that contains ProfileMarker from the
// text of a profile file. Every other line stays byte for byte, with its
// line end. changed is false when no line has the marker. It is a pure
// function.
func RemoveMarkedLines(text string) (out string, changed bool) {
	lines := strings.SplitAfter(text, "\n")
	var b strings.Builder
	for _, l := range lines {
		if strings.Contains(l, ProfileMarker) {
			changed = true
			continue
		}
		b.WriteString(l)
	}
	if !changed {
		return text, false
	}
	return b.String(), true
}

// onlyHorusFiles reports if the folder dir holds no file other than the
// program horus (and the horus.exe.old of an update of install.ps1). The
// PATH entry of such a folder serves only horus.
func onlyHorusFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		switch strings.ToLower(e.Name()) {
		case "horus", "horus.exe", "horus.exe.old":
		default:
			return false
		}
	}
	return true
}

// PathEntry gives the places that hold the PATH entry of the installer for
// the program file program, in the home folder home. An empty list means
// that there is no entry.
//
//   - Windows: "HKCU\Environment\Path" when that value has the folder of
//     program as an entry, and the folder holds only horus. A folder with
//     other programs (for example the bin folder of Go) is not the folder
//     of install.ps1, so its entry stays.
//   - other systems: each profile file of ProfileFiles that has a line with
//     ProfileMarker.
func PathEntry(program, home string) ([]string, error) { return pathEntry(program, home) }

// RemovePathEntry removes the PATH entry of the installer that PathEntry
// finds. It changes nothing when there is no entry.
func RemovePathEntry(program, home string) error { return removePathEntry(program, home) }

// RemoveSelf deletes the file of the running program (os.Executable). It
// takes no path, so it cannot delete another file.
//
//   - Windows: a running program cannot delete its own file. RemoveSelf
//     starts a hidden PowerShell process that waits until this process ends,
//     then deletes the file. The file is gone a moment after the exit.
//   - other systems: the file is deleted at once.
func RemoveSelf() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return self, removeSelf(self)
}

// profilePathEntry is PathEntry on a system with profile files.
func profilePathEntry(home string) ([]string, error) {
	var out []string
	for _, f := range ProfileFiles(home) {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if _, changed := RemoveMarkedLines(string(data)); changed {
			out = append(out, f)
		}
	}
	return out, nil
}

// removeProfileLines is RemovePathEntry on a system with profile files. It
// keeps the mode of each file.
func removeProfileLines(home string) error {
	for _, f := range ProfileFiles(home) {
		fi, err := os.Lstat(f)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		out, changed := RemoveMarkedLines(string(data))
		if !changed {
			continue
		}
		if err := os.WriteFile(f, []byte(out), fi.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
