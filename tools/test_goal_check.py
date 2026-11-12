"""Unit tests of the CI judgement of goal_check.py. No network. Run: python -m unittest discover -s tools"""

import argparse
import tempfile
import unittest
import unittest.mock
from pathlib import Path

import goal_check as g


def jobs(**overrides):
    """All 7 jobs of ci.yml green, with the given conclusions changed. A value None removes the job."""
    names = ["contract windows-latest", "contract macos-latest", "workflow-parity windows-latest",
             "workflow-parity macos-latest", "go windows/amd64", "go darwin/arm64", "go linux/amd64"]
    out = []
    for n in names:
        key = n.replace(" ", "_").replace("-", "_").replace("/", "_")
        c = overrides.get(key, "success")
        if c is not None:
            out.append({"name": n, "status": "completed", "conclusion": c})
    return out


class JudgeJobs(unittest.TestCase):
    def test_all_green_passes_same_behaviour_and_process_tree(self):
        self.assertTrue(g.judge_jobs(jobs(), g.SAME_BEHAVIOUR_JOBS)[0])
        self.assertTrue(g.judge_jobs(jobs(), g.PROCESS_TREE_JOBS)[0])

    def test_red_parity_job_fails_same_behaviour_and_names_it(self):
        ok, detail = g.judge_jobs(jobs(workflow_parity_macos_latest="failure"), g.SAME_BEHAVIOUR_JOBS)
        self.assertFalse(ok)
        self.assertIn("workflow-parity macos-latest", detail)

    def test_missing_darwin_job_fails_both_rows_and_names_it(self):
        for required in (g.SAME_BEHAVIOUR_JOBS, g.PROCESS_TREE_JOBS):
            ok, detail = g.judge_jobs(jobs(go_darwin_arm64=None), required)
            self.assertFalse(ok)
            self.assertIn("go darwin/arm64 (missing)", detail)

    def test_running_and_cancelled_jobs_fail(self):
        running = jobs()
        running[2] = {"name": "workflow-parity windows-latest", "status": "in_progress", "conclusion": None}
        ok, detail = g.judge_jobs(running, g.SAME_BEHAVIOUR_JOBS)
        self.assertFalse(ok)
        self.assertIn("workflow-parity windows-latest (in_progress)", detail)
        ok, detail = g.judge_jobs(jobs(go_windows_amd64="cancelled"), g.PROCESS_TREE_JOBS)
        self.assertFalse(ok)
        self.assertIn("go windows/amd64 (cancelled)", detail)

    def test_process_tree_ignores_a_red_parity_job(self):
        self.assertTrue(g.judge_jobs(jobs(workflow_parity_macos_latest="failure"), g.PROCESS_TREE_JOBS)[0])

    def test_empty_job_list_fails(self):
        self.assertFalse(g.judge_jobs([], g.SAME_BEHAVIOUR_JOBS)[0])
        self.assertFalse(g.judge_jobs([], g.PROCESS_TREE_JOBS)[0])

    def test_job_names_match_the_workflow_file(self):
        # The names that the tool looks for must be the names that ci.yml produces.
        text = (g.ROOT / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
        for fragment in ("name: go ${{ matrix.goos }}/${{ matrix.goarch }}", "name: contract ${{ matrix.os }}",
                         "name: workflow-parity ${{ matrix.os }}"):
            self.assertIn(fragment, text)


# ---------------------------------------------------------------------------------------------
# Phase 3. Each row gets at least one stub that must FAIL, and one input that must PASS, so a
# check that always fails proves nothing either. The Go rows run the real go program on a small
# module in a temporary folder: no network, no ComfyUI.
# ---------------------------------------------------------------------------------------------

OLD_MCP_JSON = r"""{
  "mcpServers": {
    "comfy-mcp": {
      "type": "stdio",
      "command": "${USERPROFILE}\\.venvs\\comfy-mcp\\Scripts\\comfy-mcp.exe",
      "args": [],
      "env": {
        "COMFY_BIN": "${USERPROFILE}\\.venvs\\comfy-mcp\\Scripts\\comfy.exe",
        "COMFY_LOCAL_URL": "${COMFY_LOCAL_URL:-http://127.0.0.1:8188}",
        "PYTHONUTF8": "1",
        "PYTHONIOENCODING": "utf-8"
      }
    },
    "horus": {"type": "stdio", "command": "horus", "args": ["mcp"]}
  }
}"""
NEW_MCP_JSON = """{
  "mcpServers": {
    "comfy-mcp": {"type": "stdio", "command": "horus", "args": ["comfy-mcp"],
                  "env": {"COMFY_LOCAL_URL": "${COMFY_LOCAL_URL:-http://127.0.0.1:8188}"}},
    "horus": {"type": "stdio", "command": "horus", "args": ["mcp"]}
  }
}"""
GOOD = 'if len("MPS backend out of memory") == 0 {\n\t\tt.Fatal("empty")\n\t}'  # a body that asserts and passes


def ns(**kw):
    base = {"server_cmd": ["horus-fake"], "no_ci": True, "real": False, "real_out": None, "phase": 3, "only": None}
    base.update(kw)
    return argparse.Namespace(**base)


def all_pins():
    seen = {}
    for pins in g.PINNED3.values():
        for pkg, name, _ in pins:
            seen[(pkg, name)] = GOOD
    return seen


class GoStub(unittest.TestCase):
    """A temporary Go module that stands in for the repo root. tests maps (package, name) to a body."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.old_root = g.ROOT
        g.ROOT = self.root
        (self.root / "go.mod").write_text("module stub\n\ngo 1.21\n", encoding="utf-8")

    def tearDown(self):
        g.ROOT = self.old_root
        self.tmp.cleanup()

    def module(self, tests, empty_pkgs=()):
        by_pkg = {}
        for (pkg, name), body in tests.items():
            by_pkg.setdefault(pkg, []).append((name, body))
        for pkg in set(by_pkg) | set(empty_pkgs):
            d = self.root / pkg
            d.mkdir(parents=True, exist_ok=True)
            name = "main" if pkg.startswith("cmd/") else Path(pkg).name
            (d / "doc.go").write_text(f"package {name}\n" + ("\nfunc main() {}\n" if name == "main" else ""),
                                      encoding="utf-8")
            funcs = "".join(f"\nfunc {n}(t *testing.T) {{\n\t{b}\n}}\n" for n, b in by_pkg.get(pkg, []))
            if funcs:
                (d / "x_test.go").write_text(f'package {name}\n\nimport "testing"\n{funcs}', encoding="utf-8")


@unittest.skipUnless(g.go_exe(), "needs the go program")
class PinnedTests(GoStub):
    def test_all_pinned_tests_pass(self):
        self.module(all_pins())
        for row, pins in g.PINNED3.items():
            ok, detail = g.pinned_tests(pins)
            self.assertTrue(ok, f"{row}: {detail}")

    def test_package_with_no_test_file_fails(self):  # issue #80 of Phase 2
        self.module({}, empty_pkgs=["internal/backend", "cmd/horus"])
        ok, detail = g.pinned_tests(g.PINNED3["mac-backend"])
        self.assertFalse(ok)
        self.assertIn("internal/backend has no test files", detail)

    def test_missing_package_fails(self):
        ok, detail = g.pinned_tests(g.PINNED3["out-of-memory"])
        self.assertFalse(ok)
        self.assertIn("internal/comfy is missing", detail)

    def test_missing_name_fails_and_names_the_test_and_its_task(self):
        tests = all_pins()
        del tests[("internal/backend", "TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart")]
        self.module(tests)
        ok, detail = g.pinned_tests(g.PINNED3["mac-backend"])
        self.assertFalse(ok)
        self.assertIn("TestComfyCLIStopRefusesAComfyUIThatHorusDidNotStart (not in internal/backend, T2)", detail)

    def test_empty_body_fails(self):
        tests = all_pins()
        tests[("internal/platform", "TestInTCCFolder")] = ""
        self.module(tests)
        ok, detail = g.pinned_tests(g.PINNED3["tcc-folders"])
        self.assertFalse(ok)
        self.assertIn("TestInTCCFolder (asserts nothing", detail)

    def test_skipped_test_fails(self):
        tests = all_pins()
        tests[("internal/comfy", "TestJobErrorOfTheMPSOutOfMemoryText")] = 't.Skip("later")\n\t' + GOOD
        self.module(tests)
        ok, detail = g.pinned_tests(g.PINNED3["out-of-memory"])
        self.assertFalse(ok)
        self.assertIn("TestJobErrorOfTheMPSOutOfMemoryText (did not pass", detail)

    def test_failing_test_fails(self):
        tests = all_pins()
        tests[("cmd/horus", "TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps")] = 't.Fatal("red")'
        self.module(tests)
        ok, detail = g.pinned_tests(g.PINNED3["mac-setup"])
        self.assertFalse(ok)
        self.assertIn("TestSetupYesWithTheFakeComfyThenASecondPlanHasZeroSteps (did not pass", detail)

    def test_a_name_that_is_only_a_prefix_does_not_count(self):
        tests = all_pins()
        del tests[("internal/platform", "TestInTCCFolder")]
        tests[("internal/platform", "TestInTCCFolderOther")] = GOOD
        self.module(tests)
        ok, detail = g.pinned_tests(g.PINNED3["tcc-folders"])
        self.assertFalse(ok)
        self.assertIn("TestInTCCFolder (not in internal/platform", detail)


@unittest.skipUnless(g.go_exe(), "needs the go program")
class Rows3OnGo(GoStub):
    def test_mac_backend_needs_the_darwin_job(self):
        self.module(all_pins())
        old = g.ci_jobs
        g._CI_CACHE.clear()
        try:
            g.ci_jobs = lambda sha: (jobs(go_darwin_arm64="failure"), "")
            ok, detail = g.row3_mac_backend(ns(no_ci=False))
            self.assertFalse(ok)
            self.assertIn("go darwin/arm64 (failure)", detail)
            g._CI_CACHE.clear()
            g.ci_jobs = lambda sha: (jobs(), "")
            ok, detail = g.row3_mac_backend(ns(no_ci=False))
            self.assertTrue(ok, detail)
            self.assertIn("real part not run", detail)
        finally:
            g.ci_jobs = old
            g._CI_CACHE.clear()

    def test_mac_backend_with_real_on_windows_fails_and_says_it_needs_macos(self):
        self.module(all_pins())
        old = g.this_os
        g.this_os = lambda: "win32"
        try:
            ok, detail = g.row3_mac_backend(ns(real=True))
        finally:
            g.this_os = old
        self.assertFalse(ok)
        self.assertIn("needs macOS", detail)

    def test_mac_setup_without_the_settings_test_fails(self):
        tests = all_pins()
        del tests[("internal/settings", "TestComfyDirDefaultIsTheFolderOfPlatform")]
        self.module(tests)
        ok, detail = g.row3_mac_setup(ns())
        self.assertFalse(ok)
        self.assertIn("TestComfyDirDefaultIsTheFolderOfPlatform", detail)

    def test_tcc_folders_with_only_the_default_folder_test_names_the_warn_tests(self):  # the state after T1
        self.module({("internal/platform", "TestDefaultComfyDirOfEachOS"): GOOD})
        ok, detail = g.row3_tcc(ns())
        self.assertFalse(ok)
        for name in ("TestInTCCFolder", "TestTCCCheckWarnsForEachPathInATCCFolder", "TestDoctorWarnLinesKeepExitCodeZero"):
            self.assertIn(name, detail)
        self.assertNotIn("TestDefaultComfyDirOfEachOS", detail)

    def test_out_of_memory_needs_the_mps_text_in_both_tests(self):
        tests = all_pins()
        tests[("cmd/horus", "TestImageMPSOutOfMemoryThroughTheAdapter")] = 'if 1 != 1 {\n\t\tt.Fatal("CUDA out of memory")\n\t}'
        self.module(tests)
        ok, detail = g.row3_oom(ns())
        self.assertFalse(ok)
        self.assertIn("TestImageMPSOutOfMemoryThroughTheAdapter (has no text 'MPS backend out of memory'", detail)
        self.module(all_pins())
        self.assertTrue(g.row3_oom(ns())[0])

    def test_one_config_fails_on_the_old_mcp_json_and_on_missing_tests(self):
        self.module(all_pins())
        (self.root / ".mcp.json").write_text(OLD_MCP_JSON, encoding="utf-8")
        ok, detail = g.row3_one_config(ns())
        self.assertFalse(ok)
        self.assertIn("2 Windows-only paths", detail)
        (self.root / ".mcp.json").write_text(NEW_MCP_JSON, encoding="utf-8")
        ok, detail = g.row3_one_config(ns())
        self.assertTrue(ok, detail)
        (self.root / "cmd" / "horus" / "x_test.go").unlink()
        ok, detail = g.row3_one_config(ns())
        self.assertFalse(ok)
        self.assertIn("TestComfyMCPPassesTheStreamsComfyBinAndTheExitCode", detail)

    def test_one_config_without_mcp_json_fails(self):
        self.module(all_pins())
        ok, detail = g.row3_one_config(ns())
        self.assertFalse(ok)
        self.assertIn("missing: .mcp.json", detail)


class McpJson(unittest.TestCase):
    def test_old_file_has_two_windows_only_paths_and_a_wrong_command(self):
        import json
        self.assertEqual(g.windows_only_paths(json.loads(OLD_MCP_JSON)), 2)
        problems = g.mcp_json_problems(OLD_MCP_JSON)
        self.assertIn("2 Windows-only paths", problems)
        self.assertIn("the entry comfy-mcp is not `horus comfy-mcp`", problems)

    def test_new_file_has_no_problem(self):
        self.assertEqual(g.mcp_json_problems(NEW_MCP_JSON), [])

    def test_a_path_command_without_windows_marks_still_fails(self):
        stub = NEW_MCP_JSON.replace('"command": "horus", "args": ["comfy-mcp"]', '"command": "comfy-mcp", "args": []')
        self.assertEqual(g.mcp_json_problems(stub), ["the entry comfy-mcp is not `horus comfy-mcp`"])

    def test_not_json_fails(self):
        self.assertTrue(g.mcp_json_problems("{"))


class SetupDoc(unittest.TestCase):
    OLD = ("# Flow 1: set up a fresh Windows PC\nTwo commands are Windows-only in this phase.\n"
           "**The short way.** `horus setup it-illustration` does steps 1 and 4 in one command.\n")
    HELP = {"internal/cli/setup.go": 'x := "HORUS_COMFYUI_DIR, then ~/ComfyUI_windows_portable"\n'}

    def test_the_four_lines_of_issue_91_and_no_macos_section_fail(self):
        problems = g.setup_doc_problems(self.OLD, self.HELP)
        self.assertEqual(problems, ["docs/setup.md:2", "docs/setup.md:3", "internal/cli/setup.go:1",
                                    "docs/setup.md has no macOS section"])

    def test_fixed_lines_without_a_macos_section_fail(self):
        self.assertEqual(g.setup_doc_problems("# Set up Windows\nThree commands are Windows-only.\n", {}),
                         ["docs/setup.md has no macOS section"])

    def test_fixed_page_with_a_macos_section_passes(self):
        self.assertEqual(g.setup_doc_problems("# Set up\n## Step 1 on macOS\n", {"internal/cli/setup.go": "<home>"}), [])

    def test_row_reads_the_files_of_the_root(self):
        with tempfile.TemporaryDirectory() as tmp:
            old, g.ROOT = g.ROOT, Path(tmp)
            try:
                (g.ROOT / "docs").mkdir()
                (g.ROOT / "internal" / "cli").mkdir(parents=True)
                (g.ROOT / "docs" / "setup.md").write_text(self.OLD, encoding="utf-8")
                (g.ROOT / "internal" / "cli" / "setup.go").write_text(self.HELP["internal/cli/setup.go"], encoding="utf-8")
                ok, detail = g.row3_setup_doc(ns())
                self.assertFalse(ok)
                self.assertTrue(detail.startswith("3 lines that the commands contradict"), detail)
                (g.ROOT / "docs" / "setup.md").write_text("# Set up\n## macOS\n", encoding="utf-8")
                (g.ROOT / "internal" / "cli" / "setup.go").write_text("<home>/horus\n", encoding="utf-8")
                self.assertTrue(g.row3_setup_doc(ns())[0])
                (g.ROOT / "docs" / "setup.md").unlink()
                self.assertEqual(g.row3_setup_doc(ns()), (False, "missing: docs/setup.md"))
            finally:
                g.ROOT = old


class Patch:
    """Replace attributes of goal_check for one test, and put them back."""

    def __init__(self, **kw):
        self.kw, self.old = kw, {}

    def __enter__(self):
        for k, v in self.kw.items():
            self.old[k] = getattr(g, k)
            setattr(g, k, v)

    def __exit__(self, *exc):
        for k, v in self.old.items():
            setattr(g, k, v)


class MacImage(unittest.TestCase):
    def test_without_real_fails(self):
        self.assertEqual(g.row3_mac_image(ns()), (False, "needs --real on macOS"))

    def test_real_on_windows_fails(self):
        with Patch(this_os=lambda: "win32"):
            ok, detail = g.row3_mac_image(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("needs macOS", detail)

    def test_an_image_from_a_comfyui_that_already_ran_fails(self):
        envelope = {"ok": True, "result": {"path": "x.png", "started_backend": False}}
        with Patch(this_os=lambda: "darwin", answers=lambda url: False,
                   real_image=lambda a: (True, "1536x1024 x.png", envelope)):
            ok, detail = g.row3_mac_image(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("started_backend is not true", detail)

    def test_a_comfyui_that_horus_did_not_start_fails_before_the_image(self):
        called = []
        with Patch(this_os=lambda: "darwin", answers=lambda url: True, wait_for=lambda want, url, s: False,
                   run=lambda cmd, env=None, timeout=900: (1, "refused"),
                   real_image=lambda a: called.append(1)):
            ok, detail = g.row3_mac_image(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("horus did not start", detail)
        self.assertEqual(called, [])

    def test_a_cold_image_through_the_backend_passes(self):
        envelope = {"ok": True, "result": {"path": "x.png", "started_backend": True}}
        with Patch(this_os=lambda: "darwin", answers=lambda url: False,
                   real_image=lambda a: (True, "1536x1024 x.png", envelope)):
            ok, detail = g.row3_mac_image(ns(real=True))
        self.assertTrue(ok, detail)


class FakeMac:
    """A fake horus and comfy for the real part of mac-backend. refuse=False is a stub stop-comfyui that
    also stops a ComfyUI that a person started; start=False is a stub start-comfyui that starts nothing."""

    def __init__(self, refuse=True, start=True):
        self.refuse, self.start, self.up, self.by, self.calls = refuse, start, False, None, []

    def run(self, cmd, env=None, timeout=900):
        self.calls.append(cmd)
        if cmd[0] == "horus-fake" and cmd[1] == "start-comfyui":
            if self.start:
                self.up, self.by = True, "horus"
            return 0, "OK"
        if cmd[0] == "horus-fake" and cmd[1] == "stop-comfyui":
            if self.up and self.by != "horus" and self.refuse:
                return 1, "horus did not start the ComfyUI that answers"
            self.up = False
            return 0, "OK: stopped"
        if "launch" in cmd:
            self.up, self.by = True, "hand"
            return 0, ""
        if cmd[-1] == "stop":
            self.up = False
            return 0, ""
        return 0, ""

    def patch(self):
        return Patch(this_os=lambda: "darwin", run=self.run, answers=lambda url: self.up,
                     wait_for=lambda want, url, s: self.up == want)


class RealMacBackend(unittest.TestCase):
    def test_start_stop_and_refusal_pass(self):
        fake = FakeMac()
        with fake.patch():
            ok, detail = g.real_mac_backend(ns(real=True))
        self.assertTrue(ok, detail)
        self.assertFalse(fake.up)  # the hand-started ComfyUI is stopped at the end

    def test_a_stop_that_does_not_refuse_fails(self):
        fake = FakeMac(refuse=False)
        with fake.patch():
            ok, detail = g.real_mac_backend(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("did not refuse", detail)

    def test_a_start_that_starts_nothing_fails(self):
        fake = FakeMac(start=False)
        with fake.patch():
            ok, detail = g.real_mac_backend(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("horus start-comfyui", detail)

    def test_on_windows_it_runs_nothing(self):
        fake = FakeMac()
        with fake.patch(), Patch(this_os=lambda: "win32"):
            ok, detail = g.real_mac_backend(ns(real=True))
        self.assertFalse(ok)
        self.assertEqual(fake.calls, [])


class RealDoctorRows(unittest.TestCase):
    def test_mac_setup_fails_with_horus_comfyui_dir_set(self):
        with Patch(this_os=lambda: "darwin"), unittest.mock.patch.dict("os.environ", {"HORUS_COMFYUI_DIR": "/x"}):
            ok, detail = g.real_mac_setup(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("HORUS_COMFYUI_DIR is set", detail)

    def test_mac_setup_fails_on_a_plan_with_steps_and_passes_on_zero(self):
        def fake(steps):
            def run(cmd, env=None, timeout=900):
                if cmd[1] == "doctor":
                    return 0, "PASS  ComfyUI folder - ok\nAll checks passed.\n"
                return 0, f"Plan: {steps} steps."
            return run
        env = {k: v for k, v in g.os.environ.items() if k != "HORUS_COMFYUI_DIR"}
        with tempfile.TemporaryDirectory() as tmp, unittest.mock.patch.dict("os.environ", env, clear=True):
            with Patch(ROOT=Path(tmp), this_os=lambda: "darwin", answers=lambda url: True, run=fake(2)):
                self.assertFalse(g.real_mac_setup(ns(real=True))[0])
            with Patch(ROOT=Path(tmp), this_os=lambda: "darwin", answers=lambda url: True, run=fake(0)):
                ok, detail = g.real_mac_setup(ns(real=True))
                self.assertTrue(ok, detail)

    def test_tcc_needs_a_warn_line_that_names_the_repo(self):
        home = Path(tempfile.gettempdir()).resolve() / "goalhome"
        repo = home / "Desktop" / "horus"
        out = {"text": "PASS  ComfyUI folder - ok\nAll checks passed.\n"}
        run = lambda cmd, env=None, timeout=900: (0, out["text"])
        with Patch(ROOT=repo, this_os=lambda: "darwin", answers=lambda url: True, run=run), \
                unittest.mock.patch.object(g.Path, "home", lambda: home):
            self.assertFalse(g.real_tcc(ns(real=True))[0])
            out["text"] = f"WARN  TCC folder - {repo} is inside ~/Desktop\nAll checks passed.\n"
            ok, detail = g.real_tcc(ns(real=True))
            self.assertTrue(ok, detail)
            out["text"] += "FAIL  ComfyUI server - no answer\n"
            self.assertFalse(g.real_tcc(ns(real=True))[0])

    def test_one_config_fails_on_a_fail_line(self):
        with Patch(this_os=lambda: "win32", run=lambda cmd, env=None, timeout=900: (1, "FAIL  comfy-mcp - missing\n")):
            ok, detail = g.real_one_config(ns(real=True))
        self.assertFalse(ok)
        self.assertIn("1 FAIL lines", detail)


class TccFolder(unittest.TestCase):
    def test_inside_and_outside(self):
        home = str(Path(tempfile.gettempdir()) / "h")
        self.assertTrue(g.in_tcc_folder(home + "/Desktop/horus", home))
        self.assertTrue(g.in_tcc_folder(home + "/Downloads", home))
        self.assertFalse(g.in_tcc_folder(home + "/horus", home))
        self.assertFalse(g.in_tcc_folder(home + "/DesktopX/horus", home))


class SameBehaviour3(unittest.TestCase):
    """Each fact of the row is replaced by a fake; one bad fact must fail the row."""

    def facts(self, **kw):
        f = {"contract": (True, "local 30 of 30; CI part skipped (--no-ci)"), "parity": "parity: 17 of 17 cases equal",
             "gotest": 0, "archtest": (True, "3 named tests pass", ""), "count": (215, ""), "changed": [],
             "allow": [], "jobs": jobs()}
        f.update(kw)

        def run(cmd, env=None, timeout=900):
            if "tests/workflow_parity.py" in cmd:
                return 0, f["parity"]
            if cmd[-1] == "./...":
                return f["gotest"], "FAIL x" if f["gotest"] else "ok"
            return 0, "abc123"
        g._CI_CACHE.clear()
        return Patch(row_contract=lambda a, *r: f["contract"], run=run, go_exe=lambda: "go",
                     go_test_ran=lambda p, n=None: f["archtest"], go_test_count=lambda: f["count"],
                     changed_contract_files=lambda base="": f["changed"], allow_files=lambda: f["allow"],
                     ci_jobs=lambda sha: (f["jobs"], ""))

    def check(self, **kw):
        with self.facts(**kw):
            return g.row3_same(ns(no_ci=False))

    def test_all_good_passes(self):
        ok, detail = self.check()
        self.assertTrue(ok, detail)

    def test_each_bad_fact_fails(self):
        for kw in ({"contract": (False, "local 29 of 30, needs 30 of 30")}, {"parity": "parity: 16 of 17 cases equal"},
                   {"gotest": 1}, {"archtest": (False, "no passing test: TestPlanted", "")}, {"count": (214, "")},
                   {"changed": ["M\ttests/mcp_contract.py"]}, {"allow": ["allow_token.txt"]},
                   {"jobs": jobs(contract_macos_latest="failure")}, {"jobs": jobs(go_linux_amd64="failure")}):
            ok, detail = self.check(**kw)
            self.assertFalse(ok, f"{kw} passed: {detail}")

    def test_real_on_windows_runs_the_phase_2_rows(self):
        with self.facts(), Patch(this_os=lambda: "win32", row2_backend=lambda a: (False, "needs the 8189 ComfyUI"),
                                 row2_commands=lambda a: (True, "0 FAIL lines")):
            ok, detail = g.row3_same(ns(no_ci=False, real=True))
        self.assertFalse(ok)
        self.assertIn("backend: FAIL", detail)


class JudgeJobsStrict(unittest.TestCase):
    def test_a_red_job_that_the_row_does_not_name_fails(self):  # the CI hole of issue #81
        ok, detail = g.judge_jobs_strict(jobs(go_linux_amd64="failure"), g.MAC_BACKEND_JOBS)
        self.assertFalse(ok)
        self.assertIn("go linux/amd64 (failure)", detail)

    def test_a_skipped_job_that_the_row_does_not_name_is_no_failure(self):
        self.assertTrue(g.judge_jobs_strict(jobs(go_linux_amd64="skipped"), g.MAC_BACKEND_JOBS)[0])

    def test_the_named_job_must_be_there(self):
        ok, detail = g.judge_jobs_strict(jobs(go_darwin_arm64=None), g.MAC_BACKEND_JOBS)
        self.assertFalse(ok)
        self.assertIn("go darwin/arm64 (missing)", detail)


class Main3(unittest.TestCase):
    def test_every_row_has_a_check_and_a_row_never_raises(self):
        self.assertEqual(list(g.CHECKS3), g.ROWS3)
        self.assertEqual(len(g.ROWS3), 8)

    def test_every_pinned_name_is_unique_in_its_package_and_names_a_task(self):
        for pins in g.PINNED3.values():
            for pkg, name, task in pins:
                self.assertRegex(name, r"^Test[A-Z]\w+$")
                self.assertIn(task, ("T1", "T2", "T3", "T4", "T5"))


if __name__ == "__main__":
    unittest.main()
