// Tests of the package steps of horus setup through the wiring of cmd/horus,
// on the sandbox of workflow_test.go, for Windows and for macOS (the test
// seam goos picks the needs and the backend of the OS). The package managers
// winget and brew, and python3.12, are copies of the test binary
// (fakeProgram). The PATH of each test holds only its fakes, and
// %ProgramFiles% and the home folder are in a temporary folder: no test runs
// a real winget, brew or Python.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/backend/comfyclitest"
	"github.com/hieplam/horus/internal/platform"
)

// The environment of the fake package manager.
const (
	envPkgRecord = "HORUS_FAKE_PKG_RECORD" // the file that gets one line of arguments for each run
	envPkgPut    = "HORUS_FAKE_PKG_PUT"    // the program file that the package manager makes
)

// The pinned versions that the fake python reports after its pip install.
const fakeMCPVersion, fakeCLIVersion = "0.10.0", "1.25.0"

// fakeProgram runs the test binary as a fake program when the name of its
// file is the name of one: the package manager (winget, brew) or Python
// (python3.12, and the python of a venv). It reports false for another name.
func fakeProgram() (int, bool) {
	exe, err := os.Executable()
	if err != nil {
		return 0, false
	}
	switch strings.TrimSuffix(filepath.Base(exe), ".exe") {
	case "winget", "brew":
		return fakePackageManager(exe, os.Args[1:]), true
	case "python3.12", "python":
		return fakePython(exe, os.Args[1:]), true
	}
	return 0, false
}

// fakePackageManager appends its arguments to the record file, then makes
// the program file of the package: a copy of itself, so that a fake
// python3.12 runs as fakePython.
func fakePackageManager(exe string, args []string) int {
	f, err := os.OpenFile(os.Getenv(envPkgRecord), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 10
	}
	fmt.Fprintln(f, strings.Join(args, " "))
	f.Close()
	if put := os.Getenv(envPkgPut); put != "" {
		if copyFile(exe, put) != nil {
			return 11
		}
	}
	fmt.Println("fake package manager: installed")
	return 0
}

// fakePython answers the calls of horus install-comfy-mcp. "-m venv <dir>"
// copies itself as the python of the venv. The version check exits 1 before
// the pip install. "-m pip install" copies itself as comfy-mcp and comfy of
// the venv (the copy named comfy is the fake comfy of comfyclitest).
func fakePython(exe string, args []string) int {
	joined := strings.Join(args, " ")
	bin := filepath.Dir(exe)
	switch {
	case strings.Contains(joined, "-m venv"):
		if copyFile(exe, platform.VenvBin(args[len(args)-1], "python")) != nil {
			return 12
		}
	case strings.Contains(joined, "importlib.metadata"):
		if !platform.IsFile(filepath.Join(bin, "installed.marker")) {
			return 1
		}
		fmt.Println(fakeMCPVersion)
		fmt.Println(fakeCLIVersion)
	case strings.Contains(joined, "-m pip install"):
		venv := filepath.Dir(bin)
		for _, name := range []string{"comfy-mcp", "comfy"} {
			if copyFile(exe, platform.VenvBin(venv, name)) != nil {
				return 13
			}
		}
		os.WriteFile(filepath.Join(bin, "installed.marker"), nil, 0o644)
	}
	return 0
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

// fakeInstall is the ComfyUI install of the Windows rig: the portable build
// downloads 1.9 GB, so the test replaces it. Like the portable build, it
// needs 7z.exe in the 7-Zip folder of %ProgramFiles%, then it makes main.py.
type fakeInstall struct {
	dir, sevenZip string
	runs          int
}

func (f *fakeInstall) Installed() (string, bool) {
	return f.dir, platform.IsFile(filepath.Join(f.dir, "ComfyUI", "main.py"))
}

func (f *fakeInstall) Install(_ context.Context, out io.Writer) error {
	f.runs++
	if !platform.IsFile(f.sevenZip) {
		return fmt.Errorf("7-Zip was not found at %s", f.sevenZip)
	}
	if err := os.MkdirAll(filepath.Join(f.dir, "ComfyUI"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.dir, "ComfyUI", "main.py"), []byte("# fake\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "fake install of ComfyUI in %s\n", f.dir)
	return nil
}

// pkgRig is the sandbox of one OS without ComfyUI. bin is the only folder
// of the PATH. It holds the fake package manager when the rig has one.
type pkgRig struct {
	s       *sandbox
	sm      seams
	tmp     string
	manager string // the path of the fake package manager
	record  string // its record file
	put     string // the program file that it makes
	args    string // the arguments of the spec
	install *fakeInstall
	comfy   *comfyclitest.Fake
}

func newPkgRig(t *testing.T, goos string, withManager bool) *pkgRig {
	t.Helper()
	s := newSandbox(t)
	must(t, os.RemoveAll(filepath.Join(s.comfy, "ComfyUI")))
	tmp := t.TempDir()
	bin, pf, home := filepath.Join(tmp, "bin"), filepath.Join(tmp, "Program Files"), filepath.Join(tmp, "home")
	for _, d := range []string{bin, pf, home} {
		must(t, os.MkdirAll(d, 0o755))
	}
	t.Setenv("PATH", bin)
	t.Setenv("ProgramFiles", pf)
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("HOME", home) // the venv of install-comfy-mcp is in the empty home folder
	t.Setenv("USERPROFILE", home)
	r := &pkgRig{s: s, sm: s.seams(1 << 50), tmp: tmp, record: filepath.Join(tmp, "pkg.record")}
	r.sm.goos = goos
	t.Setenv(envPkgRecord, r.record)
	name := "winget"
	switch goos {
	case "windows":
		r.put = filepath.Join(pf, "7-Zip", platform.ExeName("7z"))
		r.args = "install --id 7zip.7zip -e --accept-source-agreements --accept-package-agreements"
		r.install = &fakeInstall{dir: s.comfy, sevenZip: r.put}
		r.sm.install = r.install
	case "darwin":
		name = "brew"
		r.put = filepath.Join(bin, platform.ExeName("python3.12"))
		r.args = "install python@3.12"
		r.comfy = comfyclitest.New(t)
	}
	t.Setenv(envPkgPut, r.put)
	if withManager {
		exe, err := os.Executable()
		must(t, err)
		r.manager = filepath.Join(bin, platform.ExeName(name))
		must(t, copyFile(exe, r.manager))
	}
	return r
}

func (r *pkgRig) recorded() string {
	b, _ := os.ReadFile(r.record)
	return string(b)
}

// state is every file of the sandbox and of the temporary folder of the rig.
func (r *pkgRig) state(t *testing.T) string { return snapshot(t, r.s.root) + snapshot(t, r.tmp) }

var goosCases = []string{"windows", "darwin"}

// TestSetupYesWithoutWithPackagesRunsNoProgramAndExitsOne: horus setup demo
// -yes on a machine without ComfyUI and without the package. The plan has a
// package step, so -yes without -with-packages exits 1. Its text names the
// package, the command, and the two ways to go on. The fake package manager
// gets no run, no other program runs, no file changes and nothing is
// downloaded.
func TestSetupYesWithoutWithPackagesRunsNoProgramAndExitsOne(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newPkgRig(t, goos, true)
			before := r.state(t)
			code, out := r.s.runSetup(t, r.sm, "demo", "-yes")
			pkg := map[string]string{"windows": "7-Zip", "darwin": "Python 3.12"}[goos]
			for _, w := range []string{
				"Nothing changed. The plan installs a system package with its package manager:\n",
				"\n  " + pkg + ": " + r.manager + " " + r.args + "\n",
				"\n  - run horus setup demo -yes -with-packages\n",
				"\n  - install " + pkg + " by hand from https://",
			} {
				if !strings.Contains(out, w) {
					t.Errorf("the output lacks %q", w)
				}
			}
			if code != 1 || strings.Contains(out, "Running the plan.") {
				t.Errorf("exit %d, want 1:\n%s", code, out)
			}
			if got := r.recorded(); got != "" {
				t.Errorf("the package manager ran: %q", got)
			}
			if r.comfy != nil && r.comfy.Recorded() != "" {
				t.Errorf("comfy ran: %q", r.comfy.Recorded())
			}
			if r.install != nil && r.install.runs != 0 {
				t.Errorf("ComfyUI was installed %d times", r.install.runs)
			}
			if after := r.state(t); after != before || r.s.requests() != 0 {
				t.Errorf("%d requests; files before:\n%s\nafter:\n%s", r.s.requests(), before, after)
			}
		})
	}
}

// TestSetupYesWithPackagesRunsTheFakePackageManagerThenAPlanHasZeroSteps:
// horus setup demo -yes -with-packages on the same machine. The fake winget
// (or brew) runs one time with exactly the arguments of the spec and makes
// the program. On Windows 7z is in the 7-Zip folder of %ProgramFiles%, which
// is not on the PATH, and the install of ComfyUI finds it. On macOS comfy-cli
// is installed with the new python3.12, and the comfy-cli backend runs the
// fake comfy of that venv. Then the files are downloaded, and the next
// horus setup prints "Plan: 0 steps".
func TestSetupYesWithPackagesRunsTheFakePackageManagerThenAPlanHasZeroSteps(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newPkgRig(t, goos, true)
			code, out := r.s.runSetup(t, r.sm, "demo", "-yes", "-with-packages")
			if code != 0 || !strings.Contains(out, "OK: this machine is set up for workflow 'demo'.") {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			if got, want := r.recorded(), r.args+"\n"; got != want {
				t.Fatalf("the package manager got %q, want %q", got, want)
			}
			if !strings.Contains(out, "fake package manager: installed\n") || !platform.IsFile(r.put) {
				t.Fatalf("no output of the package manager, or no %s:\n%s", r.put, out)
			}
			step := strings.Index(out, "OK: "+map[string]string{"windows": "7-Zip", "darwin": "Python 3.12"}[goos]+" is installed: "+r.put+"\n")
			vae := strings.Index(out, "-- vae -> ")
			if step < 0 || vae < step {
				t.Fatalf("the package step must run first, then the downloads:\n%s", out)
			}
			switch goos {
			case "windows":
				if r.install.runs != 1 {
					t.Errorf("ComfyUI was installed %d times, want 1", r.install.runs)
				}
			case "darwin":
				ui := filepath.Join(full(t, r.s.comfy), "ComfyUI")
				if got, want := r.comfy.Recorded(), "--workspace "+ui+" install --m-series --version 0.44.0\nstdin closed\n"; got != want {
					t.Errorf("the fake comfy got %q, want %q", got, want)
				}
				if !strings.Contains(out, "OK: comfy-mcp is installed in ") {
					t.Errorf("no comfy-cli step:\n%s", out)
				}
			}
			if code, out = r.s.runSetup(t, r.sm, "demo"); code != 0 || !strings.HasSuffix(out, "Plan: 0 steps. Nothing is missing.\n") {
				t.Fatalf("the next setup: exit %d:\n%s", code, out)
			}
			if got := r.recorded(); got != r.args+"\n" {
				t.Errorf("the next setup ran the package manager: %q", got)
			}
		})
	}
}

// TestSetupYesWithoutAPackageManagerRunsNothingAndExitsOne: without winget
// (or brew) on the PATH the plan has a step for a person that names the
// package and where to get it. -yes -with-packages then exits 1 and changes
// nothing.
func TestSetupYesWithoutAPackageManagerRunsNothingAndExitsOne(t *testing.T) {
	for _, c := range []struct{ goos, manager, pkg, url string }{
		{"windows", "winget", "7-Zip", "https://www.7-zip.org/"},
		{"darwin", "brew", "Python 3.12", "https://www.python.org/downloads/"},
	} {
		t.Run(c.goos, func(t *testing.T) {
			r := newPkgRig(t, c.goos, false)
			person := "  1. a step for a person: " + c.manager + " was not found. Install " + c.pkg + " by hand from " + c.url + "\n"
			code, out := r.s.runSetup(t, r.sm, "demo")
			if code != 0 || !strings.Contains(out, person) || !strings.Contains(out, "MISSING   package manager "+c.manager) {
				t.Fatalf("the plan: exit %d:\n%s", code, out)
			}
			before := r.state(t)
			code, out = r.s.runSetup(t, r.sm, "demo", "-yes", "-with-packages")
			want := "FAIL: " + c.manager + " was not found, so horus cannot install " + c.pkg + ". Nothing changed.\n" +
				"Install " + c.pkg + " by hand from " + c.url + ", then run horus setup demo -yes again.\n"
			if code != 1 || !strings.HasSuffix(out, want) {
				t.Fatalf("exit %d, want 1 and the end %q:\n%s", code, want, out)
			}
			if after := r.state(t); after != before || r.s.requests() != 0 || r.recorded() != "" {
				t.Errorf("%d requests; files before:\n%s\nafter:\n%s", r.s.requests(), before, after)
			}
			if r.comfy != nil && r.comfy.Recorded() != "" {
				t.Errorf("comfy ran: %q", r.comfy.Recorded())
			}
		})
	}
}

// TestSetupOfAMachineWithComfyUIGetsThePlanOfToday: with ComfyUI installed,
// the plan has no package step, no comfy-cli step and no line of their
// facts, also without the package and its package manager.
func TestSetupOfAMachineWithComfyUIGetsThePlanOfToday(t *testing.T) {
	for _, goos := range goosCases {
		t.Run(goos, func(t *testing.T) {
			r := newPkgRig(t, goos, false)
			must(t, os.MkdirAll(filepath.Join(r.s.comfy, "ComfyUI"), 0o755))
			must(t, os.WriteFile(filepath.Join(r.s.comfy, "ComfyUI", "main.py"), []byte("# fake\n"), 0o644))
			code, out := r.s.runSetup(t, r.sm, "demo")
			if code != 0 || !strings.Contains(out, "PRESENT   ComfyUI in "+r.s.comfy+"\n") || !strings.Contains(out, "Plan: 2 step(s):\n  1. download vae ") ||
				!strings.Contains(out, "Nothing changed. Run horus setup demo -yes to run the plan.") {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			for _, no := range []string{"7-Zip", "Python 3.12", "comfy-cli", "package manager", "-with-packages"} {
				if strings.Contains(out, no) {
					t.Errorf("the plan names %q:\n%s", no, out)
				}
			}
		})
	}
}
