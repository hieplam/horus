// The copy of the built-in workflows. The program holds the workflow folders
// (package workflows, go:embed). The code of internal/catalog and
// internal/provision reads a folder, so the program copies them once to
// <user folder>/builtin-workflows and gives that folder as the workflows
// folder.
package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hieplam/horus/workflows"
)

// stampFile is the file in the copy that names the version of the built-in
// workflows that it holds.
const stampFile = ".horus-stamp"

var (
	stampOnce    sync.Once
	builtinStamp string
)

// BuiltinWorkflowsDir is <user folder>/builtin-workflows. It writes the copy
// of the built-in workflows when the folder is absent or when its stamp file
// names another version. When the write fails, it still returns the folder:
// the reader then reports the missing folder or file.
func BuiltinWorkflowsDir() string {
	mu.Lock()
	done := copyDir
	mu.Unlock()
	if done != "" {
		return done
	}
	stampOnce.Do(func() { builtinStamp = stampOf(workflows.FS) })
	dir := filepath.Join(UserDir(), "builtin-workflows")
	if err := writeCopy(dir, workflows.FS, builtinStamp); err == nil {
		mu.Lock()
		copyDir = dir
		mu.Unlock()
	}
	return dir
}

// stampOf is the version of the files of src: the SHA-256 of each path and
// its bytes, in the order of fs.WalkDir. So a build with other workflow files
// has another stamp, also when the version of the program is the same (dev).
func stampOf(src fs.FS) string {
	h := sha256.New()
	_ = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(data))
		h.Write(data)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

func readStamp(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, stampFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeCopy makes dir a copy of src with the stamp file stamp, unless dir
// already has that stamp. It writes the files to a new temporary folder next
// to dir and renames that folder to dir, so a reader never sees half a copy.
// When another program renamed its copy first, writeCopy keeps that copy if
// its stamp is right. An old copy is moved aside and deleted.
func writeCopy(dir string, src fs.FS, stamp string) error {
	if readStamp(dir) == stamp {
		return nil
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, filepath.Base(dir)+".tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) // after a good rename, tmp no longer exists
	if err := os.CopyFS(tmp, src); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, stampFile), []byte(stamp+"\n"), 0o644); err != nil {
		return err
	}
	var last error
	for range 5 {
		if last = os.Rename(tmp, dir); last == nil {
			return nil
		}
		if readStamp(dir) == stamp {
			return nil // another program wrote the same copy first
		}
		old := tmp + ".old"
		if err := os.Rename(dir, old); err == nil {
			os.RemoveAll(old)
		}
	}
	return fmt.Errorf("cannot write the copy of the built-in workflows to %s: %w", dir, last)
}
