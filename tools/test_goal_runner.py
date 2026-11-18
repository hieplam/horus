"""Unit tests of the runner of the goal check (goal_runner.py) and of the suite file format. No network, no Go.
Run: python -m unittest discover -s tools"""

import contextlib
import io
import tempfile
import types
import unittest
from pathlib import Path

import goal_runner as runner

FEATURE = '''
title = "a new feature"
spec = "docs/specs/feature-x.md"
real_where = "on the GPU machine"

[[rows]]
name = "python-runs"

[[rows.parts]]
check = "command"
cmd = ["{python}", "-c", "print('feature ok')"]
expect = "^feature ok$"

[[rows]]
name = "files"

[[rows.parts]]
check = "files"
paths = ["AGENTS.md"]

[[rows.parts]]
check = "files"
paths = ["no-such-file.txt"]

[[rows]]
name = "image"

[[rows.parts]]
check = "command"
cmd = ["{python}", "-c", "pass"]

[rows.real]
check = "mcp_image"
needs = "on the GPU machine"
'''


def opts(**kw):
    base = {"server_cmd": None, "no_ci": True, "real": False, "real_out": None, "phase": None, "only": None}
    base.update(kw)
    return types.SimpleNamespace(**base)


def run_main(*argv):
    """main() with argv: (exit code, output). argparse errors exit with code 2."""
    out = io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
        try:
            code = runner.main(list(argv))
        except SystemExit as exc:
            code = exc.code
    return code, out.getvalue()


class SuiteFolder(unittest.TestCase):
    """A temporary tools/suites folder with the given suite files."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.old = runner.SUITES_DIR
        runner.SUITES_DIR = Path(self.tmp.name)

    def tearDown(self):
        runner.SUITES_DIR = self.old
        self.tmp.cleanup()

    def write(self, name, text):
        (runner.SUITES_DIR / f"{name}.toml").write_text(text, encoding="utf-8")


class NewSuite(SuiteFolder):
    """A new feature needs only a new suite file: no change to the runner and no Python code."""

    def test_a_new_suite_file_is_listed_and_runs(self):
        self.write("feature-x", FEATURE)
        code, out = run_main("--list")
        self.assertEqual(code, 0)
        self.assertIn("feature-x: a new feature (docs/specs/feature-x.md, 3 rows)", out)
        code, out = run_main("--suite", "feature-x")
        lines = out.splitlines()
        self.assertEqual(lines[0], "PASS  python-runs - {python} -c print('feature ok'): exit 0")
        self.assertEqual(lines[1], "FAIL  files - 1 files present; missing: no-such-file.txt")
        self.assertEqual(lines[2], "PASS  image - {python} -c pass: exit 0; real part not run (needs --real on the GPU machine)")
        self.assertEqual(lines[3], "goal: 2 of 3 rows passed")
        self.assertEqual(lines[4], "note: the real part did not run for image (run with --real on the GPU machine)")
        self.assertEqual(code, 1)


class ReadmeExample(SuiteFolder):
    def test_the_example_of_the_readme_is_a_suite_that_loads(self):
        readme = (Path(self.old) / "README.md").read_text(encoding="utf-8")
        example = readme.split("```toml\n", 1)[1].split("```", 1)[0]
        self.write("example", example)
        s = runner.load_suite("example")
        self.assertEqual([r["name"] for r in s["rows"]], ["version"])


class BadSuite(SuiteFolder):
    def problem(self, text):
        self.write("bad", text)
        with self.assertRaises(runner.SuiteError) as ctx:
            runner.load_suite("bad")
        return str(ctx.exception)

    def test_an_unknown_check_type_names_the_row_and_the_part(self):
        msg = self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n[[rows.parts]]\ncheck = "nope"\n')
        self.assertIn("row r1, part 1: unknown check type: nope", msg)

    def test_an_unknown_module_is_an_unknown_check_type(self):
        msg = self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n[[rows.parts]]\ncheck = "nomodule.f"\n')
        self.assertIn("unknown check type: nomodule.f", msg)

    def test_a_wrong_parameter_names_the_check(self):
        msg = self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n[[rows.parts]]\ncheck = "files"\npath = ["x"]\n')
        self.assertIn("row r1, part 1: check files:", msg)

    def test_a_missing_parameter_is_an_error(self):
        msg = self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n[[rows.parts]]\ncheck = "ci_green"\n')
        self.assertIn("check ci_green:", msg)

    def test_a_row_needs_a_part(self):
        self.assertIn("at least one", self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n'))

    def test_two_rows_with_one_name_are_an_error(self):
        part = '[[rows.parts]]\ncheck = "files"\npaths = []\n'
        msg = self.problem(f'title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n{part}[[rows]]\nname = "r1"\n{part}')
        self.assertIn("the name is used two times", msg)

    def test_a_real_part_needs_where_it_runs(self):
        msg = self.problem('title = "t"\nspec = "s"\n[[rows]]\nname = "r1"\n[[rows.parts]]\ncheck = "files"\npaths = []\n'
                           '[rows.real]\ncheck = "mcp_image"\n')
        self.assertIn("real part: needs", msg)

    def test_a_suite_needs_its_title_spec_and_rows(self):
        self.assertIn("has no title, spec, rows", self.problem("x = 1\n"))

    def test_a_broken_suite_exits_two_and_list_names_it(self):
        self.write("bad", "title = \n")
        self.assertEqual(run_main("--suite", "bad")[0], 2)
        code, out = run_main("--list")
        self.assertEqual(code, 2)
        self.assertIn("bad: cannot run:", out)


class CheckRow(unittest.TestCase):
    def suite(self, rows, **kw):
        return {"name": "s", "title": "t", "spec": "s", "rows": rows, **kw}

    def test_the_parts_join_and_one_failed_part_fails_the_row(self):
        row = {"name": "r", "parts": [{"check": "files", "paths": ["AGENTS.md"]}, {"check": "files", "paths": ["nope"]}]}
        self.assertEqual(runner.check_row(self.suite([row]), row, opts()), (False, "1 files present; missing: nope"))

    def test_stop_at_fail_stops_at_the_first_failed_part(self):
        row = {"name": "r", "stop_at_fail": True,
               "parts": [{"check": "files", "paths": ["nope"]}, {"check": "files", "paths": ["AGENTS.md"]}]}
        self.assertEqual(runner.check_row(self.suite([row]), row, opts()), (False, "missing: nope"))

    def test_a_check_that_raises_fails_the_row(self):
        row = {"name": "r", "parts": [{"check": "command", "cmd": "not a list"}]}
        ok, detail = runner.check_row(self.suite([row]), row, opts())
        self.assertFalse(ok)
        self.assertTrue(detail.startswith("check failed to run: "), detail)

    def test_the_real_part_runs_only_with_real_and_uses_the_needs_of_the_suite(self):
        row = {"name": "r", "parts": [{"check": "files", "paths": []}], "real": {"check": "files", "paths": ["nope"]}}
        s = self.suite([row], real_needs="on the PC")
        self.assertEqual(runner.check_row(s, row, opts()), (True, "0 files present; real part not run (needs --real on the PC)"))
        self.assertEqual(runner.check_row(s, row, opts(real=True)), (False, "0 files present; real: missing: nope"))

    def test_a_failed_part_never_runs_the_real_part(self):
        row = {"name": "r", "parts": [{"check": "files", "paths": ["nope"]}], "real": {"check": "command", "cmd": "boom"}}
        s = self.suite([row], real_needs="on the PC")
        self.assertEqual(runner.check_row(s, row, opts(real=True)), (False, "missing: nope"))


class Suites(unittest.TestCase):
    def test_the_four_phases_are_suites(self):
        for n in ("phase1", "phase2", "phase3", "phase4"):
            self.assertIn(n, runner.suite_names())

    def test_each_suite_loads_and_names_a_spec_that_exists(self):
        for name in runner.suite_names():
            s = runner.load_suite(name)
            self.assertTrue((runner.RUNNER_DIR.parent / s["spec"]).is_file(), f"{name}: no spec {s['spec']}")

    def test_an_unknown_suite_is_an_error(self):
        with self.assertRaises(runner.SuiteError):
            runner.load_suite("nope")


class CommandLine(unittest.TestCase):
    def test_an_unknown_suite_exits_two(self):
        code, out = run_main("--suite", "nope")
        self.assertEqual(code, 2)
        self.assertIn("unknown suite: nope", out)

    def test_suite_and_phase_must_name_the_same_suite(self):
        code, out = run_main("--suite", "phase3", "--phase", "4")
        self.assertEqual(code, 2)
        self.assertIn("name different suites", out)

    def test_an_unknown_row_exits_two(self):
        code, out = run_main("--suite", "phase4", "--only", "nope")
        self.assertEqual(code, 2)
        self.assertIn("unknown row(s) for this suite: nope", out)

    def test_the_summary_counts_the_rows_of_only(self):
        code, out = run_main("--suite", "phase4", "--only", "readme")
        lines = out.splitlines()
        self.assertTrue(lines[0].startswith(("PASS  readme - ", "FAIL  readme - ")), out)
        self.assertRegex(lines[1], r"^goal: [01] of 1 rows passed$")
        self.assertEqual(code, 0 if lines[0].startswith("PASS") else 1)


if __name__ == "__main__":
    unittest.main()
