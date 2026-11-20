package mcpserver

import (
	"testing"

	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/settings"
)

// TestPathsWindows: a host path of a Windows PC in a shared folder becomes
// its container path and back, in any case and with either separator. A
// path outside each shared folder, a relative path and a path that leaves
// the folder with ".." are not in a shared folder.
func TestPathsWindows(t *testing.T) {
	p := PathsOf([]settings.SharedDir{{Host: `C:\Users\you\Pictures\horus\`, Container: "/shared/pictures"},
		{Host: `\\nas\share\out`, Container: "/shared/nas"}}, "")
	if p.Style != jobs.PathWindows {
		t.Fatalf("style %s, want windows from the drive letter", p.Style)
	}
	for _, c := range []struct{ in, container, host string }{
		{`C:\Users\you\Pictures\horus\cover.png`, "/shared/pictures/cover.png", `C:\Users\you\Pictures\horus\cover.png`},
		{`c:/users/YOU/pictures/HORUS/a/b.png`, "/shared/pictures/a/b.png", `C:\users\YOU\pictures\HORUS\a\b.png`},
		{`C:\Users\you\Pictures\horus\x\..\y.png`, "/shared/pictures/y.png", `C:\Users\you\Pictures\horus\y.png`},
		{`\\nas\share\out\z.png`, "/shared/nas/z.png", `\\nas\share\out\z.png`},
	} {
		container, host, ok := p.ToContainer(c.in)
		if !ok || container != c.container || host != c.host {
			t.Errorf("ToContainer(%q) = %q %q %v, want %q %q", c.in, container, host, ok, c.container, c.host)
		}
	}
	for _, out := range []string{`C:\Users\you\Pictures\horus2\a.png`, `C:\Users\you\Pictures\horus\..\secret.png`,
		`D:\Users\you\Pictures\horus\a.png`, `Pictures\horus\a.png`, `C:a.png`, "/shared/pictures/a.png"} {
		if c, _, ok := p.ToContainer(out); ok {
			t.Errorf("ToContainer(%q) = %q, want not in a shared folder", out, c)
		}
	}
	if h, ok := p.ToHost("/shared/pictures/it-illustration/1.png"); !ok || h != `C:\Users\you\Pictures\horus\it-illustration\1.png` {
		t.Errorf("ToHost = %q %v", h, ok)
	}
	if h, ok := p.ToHost("/tmp/outputs/1.png"); ok {
		t.Errorf("ToHost of a path outside the shared folders = %q", h)
	}
}

// TestPathsMacOSAndLinux: on macOS the case does not count, on Linux it
// does. Without HORUS_HOST_OS and a drive letter, the rules are those of
// Linux.
func TestPathsMacOSAndLinux(t *testing.T) {
	dirs := []settings.SharedDir{{Host: "/Users/you/out", Container: "/shared/out"}}
	mac := PathsOf(dirs, "darwin")
	if c, h, ok := mac.ToContainer("/users/YOU/out/a.png"); !ok || c != "/shared/out/a.png" || h != "/users/YOU/out/a.png" {
		t.Errorf("macOS: %q %q %v", c, h, ok)
	}
	linux := PathsOf(dirs, "")
	if linux.Style != jobs.PathLinux {
		t.Fatalf("style %s, want linux", linux.Style)
	}
	if c, _, ok := linux.ToContainer("/users/YOU/out/a.png"); ok {
		t.Errorf("Linux: another case is another folder, got %q", c)
	}
	if c, _, ok := linux.ToContainer("/Users/you/out/a.png"); !ok || c != "/shared/out/a.png" {
		t.Errorf("Linux: %q %v", c, ok)
	}
	if h, ok := linux.ToHost("/shared/out/x/y.png"); !ok || h != "/Users/you/out/x/y.png" {
		t.Errorf("ToHost: %q %v", h, ok)
	}
}
