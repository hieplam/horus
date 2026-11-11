// Package modelfiles is the adapter for the model files on the disk. It ports
// the file part of scripts/workflow.ps1.
//
// Where a model file of folder F is looked for, in this order: the models
// folder/F (HORUS_MODELS_DIR; for F = text_encoders, the text encoder folder
// HORUS_TEXT_ENCODER_DIR comes first), <ComfyUI dir>/ComfyUI/models/F, then
// every folder that ComfyUI's extra_model_paths.yaml maps to F. A new
// download goes to the first folder.
//
// The package also gives the size and the SHA-256 of a file, deletes a file
// (also a read-only one), and adds folders to extra_model_paths.yaml after a
// backup of the file. It decides nothing: which file to delete, to download
// or to map is the decision of internal/provision.
package modelfiles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// Store is the model files of one ComfyUI. An empty field is read from the
// settings at each call, so a Store that cmd/horus makes at the start sees
// the settings of each call.
type Store struct {
	ComfyUIDir     string // the ComfyUI portable folder (default: HORUS_COMFYUI_DIR)
	ModelsDir      string // default: HORUS_MODELS_DIR, then <ComfyUIDir>/ComfyUI/models
	TextEncoderDir string // default: HORUS_TEXT_ENCODER_DIR
	// Now gives the time in the name of the backup of extra_model_paths.yaml.
	// Default: time.Now.
	Now func() time.Time
}

// Dirs gives the folders where a model file of this folder is looked for, in
// order, as full paths, each one time.
func (s *Store) Dirs(folder string) []string {
	var dirs []string
	if te := s.textEncoderDir(); folder == "text_encoders" && te != "" {
		dirs = append(dirs, te)
	}
	dirs = append(dirs, filepath.Join(s.modelsDir(), folder), filepath.Join(s.defaultModels(), folder))
	dirs = append(dirs, s.yaml()[folder]...)
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		d = fullPath(d)
		if k := strings.ToLower(d); !seen[k] {
			seen[k] = true
			out = append(out, d)
		}
	}
	return out
}

// Own gives ComfyUI's own models folder for this folder, as a full path.
// ComfyUI reads it without a line in extra_model_paths.yaml.
func (s *Store) Own(folder string) string {
	return fullPath(filepath.Join(s.defaultModels(), folder))
}

// Mapped gives the folders that extra_model_paths.yaml maps to this folder,
// as full paths, in the order of the file.
func (s *Store) Mapped(folder string) []string { return s.yaml()[folder] }

// Map adds each pair {folder, dir} to extra_model_paths.yaml, in order, as a
// block "horus_<folder>". Before the first change of a file that exists, it
// writes a copy "<file>.bak-<time>". It writes the file one time.
func (s *Store) Map(pairs [][2]string) error {
	if len(pairs) == 0 {
		return nil
	}
	yamlPath := s.yamlPath()
	text := ""
	if data, err := os.ReadFile(yamlPath); err == nil {
		text = string(bytes.TrimPrefix(data, bom))
		now := time.Now
		if s.Now != nil {
			now = s.Now
		}
		if err := os.WriteFile(yamlPath+".bak-"+now().Format("20060102-150405"), data, 0o644); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, p := range pairs {
		folder, full := p[0], fullPath(p[1])
		base := strings.ReplaceAll(filepath.Dir(full), `\`, "/")
		text = strings.TrimLeftFunc(strings.TrimRightFunc(text, isSpace)+
			"\n\nhorus_"+folder+":\n    base_path: "+base+"\n    "+folder+": "+filepath.Base(full)+"\n", isSpace)
	}
	return os.WriteFile(yamlPath, []byte(text), 0o644)
}

// Size gives the size of the file at path, or -1 when no file is there.
func (s *Store) Size(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return -1
	}
	return fi.Size()
}

// SHA256 gives the SHA-256 of the file at path, in lower-case hexadecimal.
func (s *Store) SHA256(path string) (string, error) {
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

// Delete is Remove-Item -Force: it also deletes a read-only file.
func (s *Store) Delete(path string) error {
	err := os.Remove(path)
	if err == nil {
		return nil
	}
	if fi, serr := os.Stat(path); serr == nil && fi.Mode().Perm()&0o200 == 0 {
		if os.Chmod(path, fi.Mode().Perm()|0o200) == nil {
			if err2 := os.Remove(path); err2 == nil {
				return nil
			}
		}
	}
	return err
}

func (s *Store) comfyUIDir() string {
	if s.ComfyUIDir != "" {
		return s.ComfyUIDir
	}
	return settings.ComfyDir()
}

func (s *Store) defaultModels() string {
	return filepath.Join(s.comfyUIDir(), "ComfyUI", "models")
}

func (s *Store) modelsDir() string {
	if s.ModelsDir != "" {
		return s.ModelsDir
	}
	return settings.Get("HORUS_MODELS_DIR", s.defaultModels())
}

func (s *Store) textEncoderDir() string {
	if s.TextEncoderDir != "" {
		return s.TextEncoderDir
	}
	return settings.Get("HORUS_TEXT_ENCODER_DIR", "")
}

func (s *Store) yamlPath() string {
	return filepath.Join(s.comfyUIDir(), "ComfyUI", "extra_model_paths.yaml")
}

// yaml reads extra_model_paths.yaml at each call, so a change of the file is
// seen at once.
func (s *Store) yaml() map[string][]string { return yamlFolders(s.yamlPath()) }

var (
	yamlComment  = regexp.MustCompile(`\s+#.*$`)
	yamlBasePath = regexp.MustCompile(`(?i)^\s+base_path:\s*(.+)$`)
	yamlDefault  = regexp.MustCompile(`(?i)^\s+is_default:`)
	yamlBlock    = regexp.MustCompile(`^\s+([A-Za-z0-9_]+):\s*\|\s*$`)
	yamlKeyValue = regexp.MustCompile(`^\s+([A-Za-z0-9_]+):\s*(.+)$`)
	yamlTopLevel = regexp.MustCompile(`^\S`)
)

// yamlFolders is Get-YamlFolders, the small reader of extra_model_paths.yaml:
// folder name -> full paths, in file order.
func yamlFolders(path string) map[string][]string {
	out := map[string][]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	base, blockKey := "", ""
	for _, raw := range splitLines(data) {
		line := strings.TrimRightFunc(yamlComment.ReplaceAllString(raw, ""), isSpace)
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeftFunc(line, isSpace), "#") {
			continue
		}
		if yamlTopLevel.MatchString(line) {
			base, blockKey = "", ""
			continue
		}
		if m := yamlBasePath.FindStringSubmatch(line); m != nil {
			base, blockKey = trimQuotes(m[1]), ""
			continue
		}
		if yamlDefault.MatchString(line) {
			blockKey = ""
			continue
		}
		if m := yamlBlock.FindStringSubmatch(line); m != nil {
			blockKey = m[1]
			continue
		}
		var key, val string
		if m := yamlKeyValue.FindStringSubmatch(line); m != nil {
			blockKey, key, val = "", m[1], m[2]
		} else if blockKey != "" {
			key, val = blockKey, strings.TrimSpace(line)
		} else {
			continue
		}
		val = trimQuotes(val)
		if val == "" {
			continue
		}
		p := val
		if !platform.IsRooted(val) && base != "" {
			p = filepath.Join(base, val)
		}
		out[key] = append(out[key], fullPath(p))
	}
	return out
}

// splitLines splits as [IO.File]::ReadAllLines does, without a BOM.
func splitLines(data []byte) []string {
	s := string(bytes.TrimPrefix(data, bom))
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var bom = []byte{0xef, 0xbb, 0xbf}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '\f' || r == '\v'
}

// trimQuotes is .Trim().Trim('"').Trim("'").
func trimQuotes(s string) string {
	return strings.Trim(strings.Trim(strings.TrimSpace(s), `"`), "'")
}

// fullPath is Get-FullPath: [IO.Path]::GetFullPath, without a trailing backslash.
func fullPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	return strings.TrimRight(filepath.Clean(p), `\`)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
