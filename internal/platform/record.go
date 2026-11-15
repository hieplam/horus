// The install record and the removal of a folder that horus made.
package platform

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// InstallRecordName is the file name of the install record: the file that
// horus writes into a folder that horus made (the ComfyUI install folder, the
// venv of horus install-comfy-mcp). horus uninstall deletes such a folder only
// when the record is there.
const InstallRecordName = "horus-install.json"

// The kinds of an install record.
const (
	RecordComfyUI  = "comfyui"        // the install folder of ComfyUI
	RecordComfyMCP = "comfy-mcp-venv" // the venv of horus install-comfy-mcp
)

// installRecord is the content of horus-install.json.
type installRecord struct {
	Program string `json:"program"` // always "horus"
	Kind    string `json:"kind"`    // RecordComfyUI or RecordComfyMCP
	Created string `json:"created"` // the time of the install (RFC 3339)
}

// WriteInstallRecord writes the install record of the kind into the folder
// dir. The folder must exist.
func WriteInstallRecord(dir, kind string) error {
	data, err := json.MarshalIndent(installRecord{Program: "horus", Kind: kind, Created: time.Now().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, InstallRecordName), append(data, '\n'), 0o644)
}

// HasInstallRecord reports if the folder dir holds an install record of the
// kind: a regular file (not a link) whose program is horus.
func HasInstallRecord(dir, kind string) bool {
	p := filepath.Join(dir, InstallRecordName)
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var r installRecord
	return json.Unmarshal(data, &r) == nil && r.Program == "horus" && r.Kind == kind
}

// IsLink reports if path is a symbolic link, or a reparse point such as an
// NTFS junction (Go gives it the mode ModeIrregular). RemoveTree removes such
// an entry as a link and never reads the folder that it points to.
func IsLink(mode fs.FileMode) bool { return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// RemoveTree deletes the folder dir and everything in it. It never follows a
// link: a symbolic link or a junction in the tree is deleted as a link, and
// the folder that it points to keeps its content. When dir itself is a link,
// only the link goes. A read-only file is deleted too. A missing dir is not
// an error.
func RemoveTree(dir string) error {
	fi, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if IsLink(fi.Mode()) || !fi.IsDir() {
		return removeEntry(dir, fi.Mode())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var first error
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		info, err := os.Lstat(p)
		if err != nil {
			if first == nil && !errors.Is(err, fs.ErrNotExist) {
				first = err
			}
			continue
		}
		if !IsLink(info.Mode()) && info.IsDir() {
			err = RemoveTree(p)
		} else {
			err = removeEntry(p, info.Mode())
		}
		if err != nil && first == nil {
			first = err
		}
	}
	if first != nil {
		return first
	}
	return os.Remove(dir)
}

// removeEntry deletes one file or one link. A read-only file gets write
// permission first, because Windows refuses to delete it.
func removeEntry(p string, mode fs.FileMode) error {
	err := os.Remove(p)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if mode.IsRegular() && mode.Perm()&0o200 == 0 {
		if os.Chmod(p, 0o644) == nil {
			return os.Remove(p)
		}
	}
	return err
}
