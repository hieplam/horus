"""Unit tests of the runner of the goal check (goal_runner.py). No network, no Go.
Run: python -m unittest discover -s tools"""

import contextlib
import io
import sys
import types
import unittest

import goal_runner as r


def suite(rows, checks, **kw):
    """A suite module made in memory: the fields that tools/goal_runner.py documents."""
    mod = types.ModuleType("fake_suite")
    mod.TITLE, mod.SPEC, mod.ROWS, mod.CHECKS = "a fake", "docs/specs/fake.md", rows, checks
    for k, v in kw.items():
        setattr(mod, k, v)
    return mod


def run_main(*argv):
    """main() with argv: (exit code, output). argparse errors exit with code 2."""
    out = io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
        try:
            code = r.main(list(argv))
        except SystemExit as exc:
            code = exc.code
    return code, out.getvalue()


class RunRows(unittest.TestCase):
    def rows(self, s, rows, real=False):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            passed, not_real = r.run_rows(s, rows, types.SimpleNamespace(real=real))
        return passed, not_real, out.getvalue().splitlines()

    def test_each_row_prints_one_line_and_counts_a_pass(self):
        s = suite(["a", "b"], {"a": lambda a: (True, "ok"), "b": lambda a: (False, "bad")})
        passed, _, lines = self.rows(s, s.ROWS)
        self.assertEqual(passed, 1)
        self.assertEqual(lines, ["PASS  a - ok", "FAIL  b - bad"])

    def test_a_row_without_a_check_fails(self):
        s = suite(["a"], {})
        passed, _, lines = self.rows(s, s.ROWS)
        self.assertEqual(passed, 0)
        self.assertEqual(lines, ["FAIL  a - the suite has no check for this row"])

    def test_a_check_that_raises_fails_and_the_next_row_runs(self):
        def boom(a):
            raise RuntimeError("x")
        s = suite(["a", "b"], {"a": boom, "b": lambda a: (True, "ok")})
        passed, _, lines = self.rows(s, s.ROWS)
        self.assertEqual(passed, 1)
        self.assertEqual(lines[0], "FAIL  a - check failed to run: RuntimeError: x")

    def test_a_row_whose_real_part_did_not_run_is_listed(self):
        s = suite(["a", "b"], {"a": lambda a: (True, "x; real part not run (needs --real)"), "b": lambda a: (True, "y")})
        _, not_real, _ = self.rows(s, s.ROWS)
        self.assertEqual(not_real, ["a"])


class JoinReal(unittest.TestCase):
    def test_without_real_the_row_stands_on_its_part_and_says_so(self):
        ok, detail = r.join_real(types.SimpleNamespace(real=False), True, "x", lambda a: (False, "no"), "on macOS")
        self.assertEqual((ok, detail), (True, "x; real part not run (needs --real on macOS)"))

    def test_with_real_the_real_part_decides(self):
        ok, detail = r.join_real(types.SimpleNamespace(real=True), True, "x", lambda a: (False, "no"), "on macOS")
        self.assertEqual((ok, detail), (False, "x; real: no"))

    def test_a_failed_part_never_runs_the_real_part(self):
        def must_not_run(a):
            raise AssertionError("ran")
        self.assertEqual(r.join_real(types.SimpleNamespace(real=True), False, "x", must_not_run, ""), (False, "x"))


class Suites(unittest.TestCase):
    def test_the_four_phases_are_suites_and_common_is_not(self):
        names = r.suite_names()
        for n in ("phase1", "phase2", "phase3", "phase4"):
            self.assertIn(n, names)
        self.assertNotIn("common", names)

    def test_each_suite_has_its_fields_and_a_check_for_each_row(self):
        for name in r.suite_names():
            s = r.load_suite(name)
            self.assertEqual(list(s.CHECKS), list(s.ROWS), name)
            self.assertTrue((r.ROOT / s.SPEC).is_file(), f"{name}: no spec {s.SPEC}")

    def test_an_unknown_suite_is_an_error(self):
        with self.assertRaises(ValueError):
            r.load_suite("nope")


class CommandLine(unittest.TestCase):
    def test_list_prints_each_suite_with_its_spec(self):
        code, out = run_main("--list")
        self.assertEqual(code, 0)
        self.assertIn("phase4: release, install line, full setup and uninstall (docs/specs/2026-11-15-release-install.md, 8 rows)", out)

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
