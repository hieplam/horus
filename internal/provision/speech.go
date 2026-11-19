// The install state of a speech workflow. The model files of a speech workflow
// are in the speech folder, not in the model folders of ComfyUI (ADR 0032), and
// the workflow needs more parts than model files: the speech venv, ffmpeg and
// ffprobe. internal/speechsetup lists the parts. This file checks them through
// the interfaces of the service: ModelFiles for the files and System.Programs
// for the programs. The install record of the speech venv tells if its pip
// install ended. The plan and the install are in speechplan.go.
package provision

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/speechsetup"
)

// speechPart is one part of a speech workflow and what the service found.
type speechPart struct {
	part  speechsetup.Part
	file  catalog.ListedFile // the line of models.json, for a model file
	size  int64              // the size of the file at part.Path, -1 when no file is there
	found bool               // the part is there (for a model file: with the size of models.json)
	why   string             // why a part that is there does not count (the speech venv)
}

// isSpeech tells if the folder dir holds a speech workflow.
func isSpeech(dir string) bool { return catalog.KindOfDir(dir) == catalog.KindSpeech }

// venvPackages tells if the speech venv has the pinned packages: its install
// record lists speechsetup.Packages, or it has no install record (a venv that
// horus did not make). A record with other packages is a pip install that did
// not end; why says so.
func venvPackages(venv string) (ok bool, why string) {
	got, has := platform.InstallRecordPackages(venv, platform.RecordSpeechVenv)
	if !has || slices.Equal(got, speechsetup.Packages) {
		return true, ""
	}
	return false, fmt.Sprintf("the install record lists the packages [%s], want [%s]: the pip install did not end",
		strings.Join(got, " "), strings.Join(speechsetup.Packages, " "))
}

// speechParts gives each part of the speech workflow in dir, with the result
// of its look-up, in the order of speechsetup.Parts. A model file with another
// size than models.json is not found. The speech venv is found when its Python
// is there and it has the pinned packages (venvPackages).
func (s *Service) speechParts(name, dir string) ([]speechPart, error) {
	w := &catalog.Workflow{Name: name, Dir: dir}
	place := speechsetup.Here()
	parts, err := speechsetup.Parts(w, place)
	if err != nil {
		return nil, err
	}
	m, err := catalog.ReadModelList(filepath.Join(dir, "models.json"))
	if err != nil {
		return nil, err
	}
	out := make([]speechPart, 0, len(parts))
	next := 0
	for _, p := range parts {
		sp := speechPart{part: p, size: -1}
		switch p.Kind {
		case speechsetup.KindVenv:
			sp.size = s.files.Size(p.Path)
			sp.found = sp.size >= 0
			if sp.found {
				sp.found, sp.why = venvPackages(place.Venv)
			}
		case speechsetup.KindProgram:
			if s.sys.Programs != nil {
				_, err := s.sys.Programs.LookPath(p.Name)
				sp.found = err == nil
			}
		case speechsetup.KindModelFile:
			if next < len(m.Files) {
				sp.file = m.Files[next]
			}
			next++
			sp.size = s.files.Size(p.Path)
			sp.found = sp.size == p.Bytes
		}
		out = append(out, sp)
	}
	return out, nil
}

// speechRow is the row of List for a speech workflow. Files and Present count
// the parts, not only the model files, so that Installed is "yes" only when
// generate_speech finds each part.
func (s *Service) speechRow(f catalog.Folder) (Row, error) {
	parts, err := s.speechParts(f.Name, f.Dir)
	if err != nil {
		return Row{}, err
	}
	present := 0
	var total int64
	var roles []string
	for _, sp := range parts {
		if sp.found {
			present++
		}
		if sp.part.Kind == speechsetup.KindModelFile {
			total += sp.part.Bytes
			roles = append(roles, sp.file.Role)
		}
	}
	return Row{Name: f.Name, Description: f.Description, Files: len(parts), Present: present, Bytes: total,
		Gigabytes: roundGB(total), Installed: installedText(present, len(parts)), Roles: strings.Join(roles, ", ")}, nil
}

// checkSpeech adds one item for each part of a speech workflow and returns the
// number of failures. A workflow of the kind speech has no workflow.json and no
// ComfyUI node, so it has no graph check and no server check. quick skips the
// SHA-256 of the model files. only limits the model files, like for an image
// workflow; the venv and the programs are always checked.
func (s *Service) checkSpeech(res *Result, name, dir string, only []string, quick bool) (int, error) {
	parts, err := s.speechParts(name, dir)
	if err != nil {
		return 0, err
	}
	var listed []catalog.ListedFile
	for _, sp := range parts {
		if sp.part.Kind == speechsetup.KindModelFile {
			listed = append(listed, sp.file)
		}
	}
	picked, err := selectFiles(listed, only)
	if err != nil {
		return 0, err
	}
	want := map[catalog.ListedFile]bool{}
	for _, f := range picked {
		want[f] = true
	}
	res.Checked, res.Quick = true, quick
	failures := 0
	for _, sp := range parts {
		var it Item
		if sp.part.Kind == speechsetup.KindModelFile {
			if !want[sp.file] {
				continue
			}
			res.CheckFiles++
			sha := ""
			if sp.size == sp.part.Bytes && !quick {
				if sha, err = s.files.SHA256(sp.part.Path); err != nil {
					return failures, err
				}
			}
			p := sp.part.Path
			if sp.size < 0 {
				p = ""
			}
			it = fileCheck(sp.file, p, max(sp.size, 0), sha, quick, []string{filepath.Dir(sp.part.Path)})
		} else {
			it = Item{Status: StatusOK, Kind: sp.part.Kind, File: sp.part.Name, Path: sp.part.Path, Bytes: -1, Detail: "found"}
			switch {
			case sp.found:
			case sp.why != "":
				it.Status, it.Detail = StatusMissing, sp.why
			case sp.part.Path != "":
				it.Status, it.Detail = StatusMissing, "not at "+sp.part.Path
			default:
				it.Status, it.Detail = StatusMissing, "not found"
			}
		}
		if it.Status != StatusOK {
			failures++
		}
		res.add(it)
	}
	return failures, nil
}
