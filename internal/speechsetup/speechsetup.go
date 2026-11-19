// Package speechsetup says where the speech setup of horus is on this
// machine, and which parts a speech workflow needs there (ADR 0022, ADR 0032).
//
// The speech setup is one folder, the speech folder: HORUS_SPEECH_DIR, else
// <user folder>/speech. It holds:
//
//	venv/       the speech venv: the Python packages of the speech worker.
//	            horus makes it and writes its install record there.
//	models/     the model files of the speech workflows, in the folders of
//	            models.json (models/vieneu, models/codec, models/whisper-small).
//	worker.py   the file of the speech worker. horus writes it before each start.
//
// A speech workflow needs four kinds of parts: the Python of the speech venv,
// the programs ffmpeg and ffprobe on the PATH, and each model file of its
// models.json with its size. Parts lists them. Missing gives the parts that are
// not there, from two functions that the caller gives.
//
// This is a base package of rules: it reads the settings and models.json, and
// it looks at no other file. Each service checks the parts through its own
// adapters: the speak service before each call (WORKFLOW_NOT_INSTALLED), the
// setup plan and horus doctor.
package speechsetup

import (
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// Place is where the speech setup is.
type Place struct {
	Dir    string // the speech folder
	Venv   string // the speech venv: <Dir>/venv
	Python string // the Python program of the speech venv
	Models string // the model folder: <Dir>/models
	Script string // the file of the speech worker: <Dir>/worker.py
}

// Here is the place of the speech setup of the settings: HORUS_SPEECH_DIR,
// else <user folder>/speech. The user folder is HORUS_HOME, else
// <home>/horus (ADR 0009).
func Here() Place {
	return At(settings.Get("HORUS_SPEECH_DIR", filepath.Join(settings.UserDir(), "speech")))
}

// At is the place of the speech setup in the speech folder dir.
func At(dir string) Place {
	venv := filepath.Join(dir, "venv")
	return Place{
		Dir:    dir,
		Venv:   venv,
		Python: platform.VenvBin(venv, "python"),
		Models: filepath.Join(dir, "models"),
		Script: filepath.Join(dir, "worker.py"),
	}
}

// The kinds of a part.
const (
	KindVenv      = "venv"       // the Python of the speech venv
	KindProgram   = "program"    // a program on the PATH
	KindModelFile = "model_file" // one model file of models.json
)

// Programs are the programs that the speech worker runs: ffmpeg sets the
// loudness and the format, and ffprobe measures the length of the audio.
var Programs = []string{"ffmpeg", "ffprobe"}

// PythonVersion is the version of the Python that makes the speech venv.
const PythonVersion = "3.12"

// Packages are the Python packages of the speech venv, with the pinned
// versions of the spec. The speech worker passes codec_dir to the engine of
// vieneu itself, so the version of vieneu must stay pinned.
var Packages = []string{"vieneu==3.8.4", "faster-whisper==1.2.1"}

// Part is one thing that a speech workflow needs on this machine.
type Part struct {
	Kind  string `json:"kind"`  // KindVenv, KindProgram or KindModelFile
	Name  string `json:"name"`  // "speech venv", the program name, or the file name of a model file
	Path  string `json:"path"`  // the file that must be there; "" for a program
	Bytes int64  `json:"bytes"` // the size of a model file; 0 for the other kinds
}

// Parts gives the parts of the speech workflow w at the place p, in the order
// of the setup plan: the speech venv, the programs, then the model files in the
// order of models.json.
func Parts(w *catalog.Workflow, p Place) ([]Part, error) {
	m, err := w.Models()
	if err != nil {
		return nil, err
	}
	parts := []Part{{Kind: KindVenv, Name: "speech venv", Path: p.Python}}
	for _, name := range Programs {
		parts = append(parts, Part{Kind: KindProgram, Name: name})
	}
	for _, f := range m.Files {
		name := leaf(f.RepoPath)
		parts = append(parts, Part{Kind: KindModelFile, Name: name, Path: filepath.Join(p.Models, filepath.FromSlash(f.Folder), name), Bytes: f.Bytes})
	}
	return parts, nil
}

// Missing gives the parts that are not there, in their order. size gives the
// bytes of the file at a path, or -1 when no file is there. found tells if a
// program is on the PATH. A model file with another size than models.json is
// missing too.
func Missing(parts []Part, size func(path string) int64, found func(program string) bool) []Part {
	var out []Part
	for _, p := range parts {
		ok := false
		switch p.Kind {
		case KindVenv:
			ok = size(p.Path) >= 0
		case KindProgram:
			ok = found(p.Name)
		case KindModelFile:
			ok = size(p.Path) == p.Bytes
		}
		if !ok {
			out = append(out, p)
		}
	}
	return out
}

// leaf is the last part of a path with / separators.
func leaf(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
