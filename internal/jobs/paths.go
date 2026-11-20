// The host paths and the container paths of the stack (spec
// docs/specs/2026-11-19-job-queue.md, "MCP contract"; ADR 0039). An agent
// gives and gets the paths of its own machine, the host. The API process and
// the worker run in Linux containers, and Docker mounts each shared folder of
// HORUS_SHARED_DIRS into both. Paths changes a host path in a shared folder
// to its container path, and back.

package jobs

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/settings"
)

// Paths holds the shared folders of the stack and the path rules of the host.
type Paths struct {
	Dirs []settings.SharedDir
	// Style is the system of the host paths: it gives the separator, and if
	// two paths that differ only in case are the same path.
	Style PathStyle
}

// PathsOf gives the paths of the stack for the shared folders dirs and the
// host system hostOS (HORUS_HOST_OS: windows, darwin or linux). An unknown
// hostOS is windows when a host folder has a drive letter or starts with
// \\, else linux.
func PathsOf(dirs []settings.SharedDir, hostOS string) Paths {
	p := Paths{Dirs: dirs}
	switch hostOS {
	case "windows":
		p.Style = PathWindows
	case "darwin", "macos":
		p.Style = PathMacOS
	case "linux":
		p.Style = PathLinux
	default:
		p.Style = PathLinux
		for _, d := range dirs {
			if isWindowsPath(d.Host) {
				p.Style = PathWindows
			}
		}
	}
	return p
}

// isWindowsPath tells if p starts with a drive letter and a colon, or with
// two separators (a UNC path).
func isWindowsPath(p string) bool {
	return len(p) >= 2 && p[1] == ':' && isLetter(p[0]) || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//")
}

// hostPath is a host path after the clean: the volume (a drive "C:" or a UNC
// share "//server/share" on Windows, else "") and the rest, with "/" as the
// separator, from the root. The case stays as written.
type hostPath struct{ vol, rest string }

// parseHost cleans the host path p by the rules of the style. ok is false
// when p is not an absolute path of that system.
func (s Paths) parseHost(p string) (hostPath, bool) {
	if s.Style != PathWindows {
		if !strings.HasPrefix(p, "/") {
			return hostPath{}, false
		}
		return hostPath{rest: path.Clean(p)}, true
	}
	q := strings.ReplaceAll(p, `\`, "/")
	var h hostPath
	switch {
	case len(q) >= 2 && q[1] == ':' && isLetter(q[0]):
		h.vol, q = strings.ToUpper(q[:1])+":", q[2:]
	case strings.HasPrefix(q, "//"):
		parts := strings.SplitN(q[2:], "/", 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return hostPath{}, false
		}
		h.vol, q = "//"+parts[0]+"/"+parts[1], "/"
		if len(parts) == 3 {
			q += parts[2]
		}
	default:
		return hostPath{}, false
	}
	if !strings.HasPrefix(q, "/") {
		return hostPath{}, false // C:out.png is relative to the folder of drive C
	}
	h.rest = path.Clean(q)
	return h, true
}

// fold tells if two paths that differ only in case are the same path: on
// Windows and macOS, whose file systems do not tell the case apart.
func (s Paths) fold() bool { return s.Style == PathWindows || s.Style == PathMacOS }

// format gives the host path in the form of the host: with "\" on Windows.
func (s Paths) format(h hostPath) string {
	if s.Style != PathWindows {
		return h.rest
	}
	return strings.ReplaceAll(h.vol+h.rest, "/", `\`)
}

// within gives the part of the path h after the folder dir, and true, when
// h is dir or a path in dir.
func (s Paths) within(h, dir hostPath) (string, bool) {
	hs, ds := h.vol+h.rest, strings.TrimSuffix(dir.vol+dir.rest, "/")
	same := func(a, b string) bool {
		if s.fold() {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	switch {
	case same(hs, ds) || same(hs, ds+"/"):
		return "", true
	case len(hs) > len(ds) && hs[len(ds)] == '/' && same(hs[:len(ds)], ds):
		return hs[len(ds)+1:], true
	}
	return "", false
}

// ToContainer gives the container path of the host path p, and the host
// path after the clean, in the form of the host. ok is false when p is not
// an absolute host path in a shared folder.
func (s Paths) ToContainer(p string) (container, host string, ok bool) {
	h, ok := s.parseHost(p)
	if !ok {
		return "", "", false
	}
	for _, d := range s.Dirs {
		dir, ok := s.parseHost(d.Host)
		if !ok {
			continue
		}
		if rel, ok := s.within(h, dir); ok {
			return path.Join(d.Container, rel), s.format(h), true
		}
	}
	return "", "", false
}

// ToHost gives the host path of the container path c, in the form of the
// host. ok is false when c is not in a shared folder.
func (s Paths) ToHost(c string) (string, bool) {
	c = path.Clean(filepath.ToSlash(c))
	for _, d := range s.Dirs {
		dir, ok := s.parseHost(d.Host)
		if !ok {
			continue
		}
		ctr := strings.TrimSuffix(filepath.ToSlash(d.Container), "/")
		var rel string
		switch {
		case c == ctr:
		case strings.HasPrefix(c, ctr+"/"):
			rel = c[len(ctr)+1:]
		default:
			continue
		}
		h := dir
		if rel != "" {
			h.rest = path.Join(dir.rest, rel)
		}
		return s.format(h), true
	}
	return "", false
}

// HostDirs gives the host folders of the shared folders, as written.
func (s Paths) HostDirs() []string {
	out := []string{}
	for _, d := range s.Dirs {
		out = append(out, d.Host)
	}
	return out
}
