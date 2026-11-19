// Tests of the settings.
package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hieplam/horus/internal/platform"
)

func sandbox(t *testing.T, env string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range settingVars {
		t.Setenv(k, "")
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

// makeClone makes dir a horus clone: it has workflows/_template/README.md.
func makeClone(t *testing.T, dir string) {
	t.Helper()
	tpl := filepath.Join(dir, "workflows", "_template")
	if err := os.MkdirAll(tpl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "README.md"), []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRootFindsTheCloneAboveTheCurrentFolder(t *testing.T) {
	top := t.TempDir()
	makeClone(t, top)
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

// useHome makes home the home folder of this process on every OS.
func useHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// TestComfyDirDefaultIsTheFolderOfPlatform: without HORUS_COMFYUI_DIR, the
// install folder is the default folder of this OS from internal/platform
// (<home>/horus on macOS). The .env file and the environment win over it.
func TestComfyDirDefaultIsTheFolderOfPlatform(t *testing.T) {
	sandbox(t, "")
	home := t.TempDir()
	useHome(t, home)
	t.Setenv("HORUS_COMFYUI_DIR", "")
	if got, want := ComfyDir(), platform.DefaultComfyDir(home); got != want {
		t.Errorf("ComfyDir = %s, want the folder of platform %s", got, want)
	}
	if got := ComfyMCPVenv(); got != filepath.Join(home, ".venvs", "comfy-mcp") {
		t.Errorf("ComfyMCPVenv = %s", got)
	}
	sandbox(t, "HORUS_COMFYUI_DIR=/from/dotenv\n")
	if got := ComfyDir(); got != "/from/dotenv" {
		t.Errorf(".env value: ComfyDir = %s", got)
	}
	t.Setenv("HORUS_COMFYUI_DIR", "/from/env")
	if got := ComfyDir(); got != "/from/env" {
		t.Errorf("environment value: ComfyDir = %s", got)
	}
}

// settingVars is each environment variable that the package reads. A test of
// a setting must not depend on the machine, so noClone clears all of them.
var settingVars = []string{
	"HORUS_HOME", "HORUS_ROOT", "HORUS_WORKFLOWS_DIR", "HORUS_OUTPUT_DIR",
	"HORUS_COMFYUI_URL", "HORUS_COMFYUI_DIR", "HORUS_AUTOSTART", "HORUS_GENERATE_TIMEOUT",
	"HORUS_DATABASE_URL", "HORUS_WORKER_CONCURRENCY", "HORUS_JOB_RETENTION_DAYS",
}

// noClone makes this process a machine with only the program: a new home
// folder, HORUS_HOME and HORUS_ROOT not set, and a current folder outside a
// clone. It returns the home folder.
func noClone(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	useHome(t, home)
	for _, k := range settingVars {
		t.Setenv(k, "")
	}
	cwd := filepath.Join(home, "somewhere")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	Reset()
	t.Cleanup(Reset)
	if r := Root(); r != "" && !strings.HasPrefix(r, home) {
		t.Skipf("the temp folder is inside a horus clone (%s)", r)
	}
	return home
}

// TestRootWithoutACloneIsEmpty: the old rule took the folder of the
// executable as the repo root. The new rule has no repo root outside a clone.
func TestRootWithoutACloneIsEmpty(t *testing.T) {
	noClone(t)
	if got := Root(); got != "" {
		t.Fatalf("Root = %q, want no repo root", got)
	}
}

// TestAFolderWithOnlyAWorkflowsFolderIsNotAClone: a folder with a workflows
// folder but no workflows/_template/README.md is not the repo root, so the
// workflows folder is the copy of the built-in workflows. With the template
// README it is a clone.
func TestAFolderWithOnlyAWorkflowsFolderIsNotAClone(t *testing.T) {
	home := noClone(t)
	top := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(top, "workflows", "mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "a")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	Reset()
	if IsClone(top) {
		t.Fatal("IsClone: a folder with only a workflows folder must not be a clone")
	}
	if got := Root(); got != "" {
		t.Fatalf("Root = %q, want no repo root", got)
	}
	want := filepath.Join(home, "horus", "builtin-workflows")
	if got := WorkflowsDir(); got != want {
		t.Fatalf("WorkflowsDir = %q, want the copy of the built-in workflows %q", got, want)
	}
	makeClone(t, top)
	Reset()
	if !IsClone(top) {
		t.Fatal("IsClone: a folder with workflows/_template/README.md is a clone")
	}
	got, _ := filepath.EvalSymlinks(Root())
	wantRoot, _ := filepath.EvalSymlinks(top)
	if got != wantRoot {
		t.Fatalf("Root = %q, want the clone %q", got, wantRoot)
	}
}

// TestEnvFileOfTheUserFolderGivesASetting: without a repo root, horus reads
// <user folder>/.env. The environment still wins.
func TestEnvFileOfTheUserFolderGivesASetting(t *testing.T) {
	home := noClone(t)
	user := filepath.Join(home, "horus")
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(user, ".env")
	if err := os.WriteFile(env, []byte("HORUS_COMFYUI_URL=http://from-user-env:3/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	Reset()
	if got := EnvPath(); got != env {
		t.Errorf("EnvPath = %q, want %q", got, env)
	}
	if got := ComfyURL(); got != "http://from-user-env:3" {
		t.Errorf("ComfyURL = %q, want the value of %s", got, env)
	}
	t.Setenv("HORUS_COMFYUI_URL", "http://from-env:4")
	if got := ComfyURL(); got != "http://from-env:4" {
		t.Errorf("the environment must win: ComfyURL = %q", got)
	}
	// HORUS_HOME moves the user folder, and its .env.
	other := filepath.Join(home, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".env"), []byte("HORUS_AUTOSTART=no\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HORUS_HOME", other)
	Reset()
	if UserDir() != other || EnvPath() != filepath.Join(other, ".env") || Autostart() {
		t.Errorf("HORUS_HOME: UserDir %q, EnvPath %q, Autostart %v", UserDir(), EnvPath(), Autostart())
	}
}

// TestOutputDirWithoutACloneIsTheOutputsFolderOfTheUserFolder: without a
// repo root and without HORUS_OUTPUT_DIR, an image goes to
// <user folder>/outputs.
func TestOutputDirWithoutACloneIsTheOutputsFolderOfTheUserFolder(t *testing.T) {
	home := noClone(t)
	if got, want := OutputDir(), filepath.Join(home, "horus", "outputs"); got != want {
		t.Errorf("OutputDir = %q, want %q", got, want)
	}
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(home, "mine"))
	if got := OutputDir(); got != filepath.Join(home, "mine") {
		t.Errorf("HORUS_OUTPUT_DIR must win: OutputDir = %q", got)
	}
}

// TestHorusWorkflowsDirReplacesTheBuiltInWorkflows: without a clone the
// workflows folder is the copy of the built-in workflows, with
// it-illustration. HORUS_WORKFLOWS_DIR replaces it (and no copy is written),
// and so does a clone.
func TestHorusWorkflowsDirReplacesTheBuiltInWorkflows(t *testing.T) {
	home := noClone(t)
	copyOf := filepath.Join(home, "horus", "builtin-workflows")

	mine := filepath.Join(home, "mine")
	t.Setenv("HORUS_WORKFLOWS_DIR", mine)
	if got := WorkflowsDir(); got != mine {
		t.Fatalf("WorkflowsDir = %q, want HORUS_WORKFLOWS_DIR %q", got, mine)
	}
	if _, err := os.Stat(copyOf); !os.IsNotExist(err) {
		t.Fatalf("HORUS_WORKFLOWS_DIR is set, but horus wrote the copy: %v", err)
	}

	t.Setenv("HORUS_WORKFLOWS_DIR", "")
	Reset()
	if got := WorkflowsDir(); got != copyOf {
		t.Fatalf("WorkflowsDir = %q, want the copy %q", got, copyOf)
	}
	if !platform.IsFile(filepath.Join(copyOf, "it-illustration", "manifest.json")) ||
		!platform.IsFile(filepath.Join(copyOf, "it-illustration", "workflow.json")) ||
		!platform.IsFile(filepath.Join(copyOf, "it-illustration", "models.json")) {
		t.Fatalf("the copy has no it-illustration with its manifest, graph and model list")
	}
	if _, err := os.Stat(filepath.Join(copyOf, "_template")); !os.IsNotExist(err) {
		t.Errorf("the copy has _template: %v", err)
	}

	clone := filepath.Join(home, "clone")
	makeClone(t, clone)
	t.Chdir(clone)
	Reset()
	got, _ := filepath.EvalSymlinks(WorkflowsDir())
	want, _ := filepath.EvalSymlinks(filepath.Join(clone, "workflows"))
	if got != want {
		t.Fatalf("in a clone WorkflowsDir = %q, want %q", got, want)
	}
}

// TestAStampOfAnotherVersionMakesTheProgramWriteTheCopyAgain: a copy whose
// stamp file names another version is replaced. A copy with the right stamp
// is kept as it is.
func TestAStampOfAnotherVersionMakesTheProgramWriteTheCopyAgain(t *testing.T) {
	home := noClone(t)
	dir := filepath.Join(home, "horus", "builtin-workflows")
	if err := os.MkdirAll(filepath.Join(dir, "gone"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stampFile), []byte("another-version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BuiltinWorkflowsDir(); got != dir {
		t.Fatalf("BuiltinWorkflowsDir = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone")); !os.IsNotExist(err) {
		t.Errorf("the old copy is still there: %v", err)
	}
	if s := readStamp(dir); s == "another-version" || s == "" {
		t.Errorf("stamp = %q, want the stamp of this program", s)
	}
	if !platform.IsFile(filepath.Join(dir, "it-illustration", "manifest.json")) {
		t.Fatal("the new copy has no it-illustration/manifest.json")
	}

	// The right stamp: the copy is not written again.
	mark := filepath.Join(dir, "it-illustration", "mark")
	if err := os.WriteFile(mark, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	Reset()
	BuiltinWorkflowsDir()
	if !platform.IsFile(mark) {
		t.Error("a copy with the right stamp was written again")
	}
	left, _ := filepath.Glob(filepath.Join(home, "horus", "builtin-workflows.tmp-*"))
	if len(left) != 0 {
		t.Errorf("temporary folders are left: %v", left)
	}
}

// TestTwoProgramsThatWriteTheCopyAtTheSameTimeBothListTheWorkflow starts the
// test program 4 times at once, with one empty user folder. Each one gets
// the workflows folder and lists it. Each must list it-illustration.
func TestTwoProgramsThatWriteTheCopyAtTheSameTimeBothListTheWorkflow(t *testing.T) {
	if os.Getenv("HORUS_TEST_LIST_COPY") == "1" {
		return // the child part runs in TestHelperListTheCopy
	}
	home := noClone(t)
	const n = 4
	outs := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperListTheCopy$", "-test.count=1")
			cmd.Env = append(os.Environ(), "HORUS_TEST_LIST_COPY=1", "HORUS_HOME="+filepath.Join(home, "horus"))
			outs[i], errs[i] = cmd.CombinedOutput()
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil || !strings.Contains(string(outs[i]), "LISTED it-illustration") {
			t.Errorf("program %d: %v\n%s", i+1, errs[i], outs[i])
		}
	}
	left, _ := filepath.Glob(filepath.Join(home, "horus", "builtin-workflows.*"))
	if len(left) != 0 {
		t.Errorf("temporary folders are left: %v", left)
	}
}

// TestHelperListTheCopy is the child of the test above. It does nothing in
// a normal run.
func TestHelperListTheCopy(t *testing.T) {
	if os.Getenv("HORUS_TEST_LIST_COPY") != "1" {
		return
	}
	t.Setenv("HORUS_ROOT", "")
	t.Setenv("HORUS_WORKFLOWS_DIR", "")
	Reset()
	dir := WorkflowsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && platform.IsFile(filepath.Join(dir, e.Name(), "manifest.json")) {
			fmt.Println("LISTED", e.Name())
		}
	}
}

// TestWorkerSettings: the defaults of the worker settings, a value from the
// environment, and a value that is not a whole number of 1 or more.
func TestWorkerSettings(t *testing.T) {
	sandbox(t, "HORUS_JOB_RETENTION_DAYS=7\n")
	if got := DatabaseURL(); got != "" {
		t.Errorf("DatabaseURL without the setting: %q", got)
	}
	if n, err := WorkerConcurrency(); err != nil || n != 1 {
		t.Errorf("WorkerConcurrency default: %d, %v", n, err)
	}
	if n, err := JobRetentionDays(); err != nil || n != 7 {
		t.Errorf("JobRetentionDays from .env: %d, %v", n, err)
	}
	t.Setenv("HORUS_DATABASE_URL", " postgres://horus@horus-db:5432/horus ")
	t.Setenv("HORUS_WORKER_CONCURRENCY", "3")
	if got := DatabaseURL(); got != "postgres://horus@horus-db:5432/horus" {
		t.Errorf("DatabaseURL: %q", got)
	}
	if n, err := WorkerConcurrency(); err != nil || n != 3 {
		t.Errorf("WorkerConcurrency: %d, %v", n, err)
	}
	for _, v := range []string{"0", "-2", "two"} {
		t.Setenv("HORUS_WORKER_CONCURRENCY", v)
		if _, err := WorkerConcurrency(); err == nil || !strings.Contains(err.Error(), "HORUS_WORKER_CONCURRENCY must be a whole number of 1 or more") {
			t.Errorf("WorkerConcurrency %q: %v", v, err)
		}
	}
}
