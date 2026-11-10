// Tests of the port of horus/settings.py.
package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sandbox(t *testing.T, env string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HORUS_ROOT", dir)
	Reset()
	t.Cleanup(Reset)
	return dir
}

func TestEnvironmentWinsOverDotEnvThenDefault(t *testing.T) {
	sandbox(t, "HORUS_COMFYUI_URL=http://from-dotenv:1/\nHORUS_OUTPUT_DIR=\"/from/dotenv\"\n")
	t.Setenv("HORUS_COMFYUI_URL", "")
	if got := ComfyURL(); got != "http://from-dotenv:1" {
		t.Errorf(".env value: got %q", got)
	}
	t.Setenv("HORUS_COMFYUI_URL", "http://from-env:2/")
	if got := ComfyURL(); got != "http://from-env:2" {
		t.Errorf("environment value: got %q", got)
	}
	if got := OutputDir(); got != "/from/dotenv" {
		t.Errorf("quoted .env value: got %q", got)
	}
	if got := Get("HORUS_NOT_SET_ANYWHERE", "dflt"); got != "dflt" {
		t.Errorf("default: got %q", got)
	}
}

func TestParseEnv(t *testing.T) {
	got := ParseEnv([]byte("# comment\n\n  A = 1 \nB='two'\nC=\"\"\nnoequals\nD=x=y\r\nE=\n"))
	want := map[string]string{"A": "1", "B": "two", "D": "x=y"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestGetRefusesTheToken(t *testing.T) {
	sandbox(t, "HF_TOKEN=secret-value\n")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Get(HF_TOKEN) did not refuse")
		}
		if s, _ := r.(string); s == "" || strings.Contains(s, "secret-value") {
			t.Fatalf("refusal message is wrong: %v", r)
		}
	}()
	Get("HF_TOKEN", "")
}

func TestDefaultsUseRepoRoot(t *testing.T) {
	dir := sandbox(t, "")
	for _, k := range []string{"HORUS_WORKFLOWS_DIR", "HORUS_OUTPUT_DIR", "HORUS_AUTOSTART", "HORUS_GENERATE_TIMEOUT"} {
		t.Setenv(k, "")
	}
	if got := WorkflowsDir(); got != filepath.Join(dir, "workflows") {
		t.Errorf("WorkflowsDir = %q", got)
	}
	if got := OutputDir(); got != filepath.Join(dir, ".scratch", "outputs") {
		t.Errorf("OutputDir = %q", got)
	}
	if !Autostart() {
		t.Error("Autostart default must be on")
	}
	if n, err := GenerateTimeout(); err != nil || n != 1800 {
		t.Errorf("GenerateTimeout = %d, %v", n, err)
	}
	for _, off := range []string{"0", "false", "no"} {
		t.Setenv("HORUS_AUTOSTART", off)
		if Autostart() {
			t.Errorf("HORUS_AUTOSTART=%s must turn autostart off", off)
		}
	}
	t.Setenv("HORUS_GENERATE_TIMEOUT", "abc")
	if _, err := GenerateTimeout(); err == nil {
		t.Error("a bad timeout must give an error")
	}
}

func TestRootFindsWorkflowsFolderAboveTheCurrentFolder(t *testing.T) {
	top := t.TempDir()
	if err := os.MkdirAll(filepath.Join(top, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	t.Setenv("HORUS_ROOT", "")
	Reset()
	t.Cleanup(Reset)
	got, _ := filepath.EvalSymlinks(Root())
	want, _ := filepath.EvalSymlinks(top)
	if got != want {
		t.Fatalf("Root = %q, want %q", got, want)
	}

	t.Setenv("HORUS_ROOT", deep)
	Reset()
	if Root() != deep {
		t.Fatalf("HORUS_ROOT must win: Root = %q", Root())
	}
}

func TestRootFallsBackToTheExecutableFolder(t *testing.T) {
	t.Chdir(t.TempDir()) // no workflows/ at or above a fresh temp folder
	t.Setenv("HORUS_ROOT", "")
	Reset()
	t.Cleanup(Reset)
	exe, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	// A temp folder may sit below a folder with workflows/ on some machines; then that folder wins.
	if got := Root(); got != filepath.Dir(exe) {
		if _, err := os.Stat(filepath.Join(got, "workflows")); err != nil {
			t.Fatalf("Root = %q, want the executable folder %q", got, filepath.Dir(exe))
		}
	}
}
