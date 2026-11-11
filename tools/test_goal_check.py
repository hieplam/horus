"""Unit tests of the CI judgement of goal_check.py. No network. Run: python -m unittest discover -s tools"""

import unittest

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


if __name__ == "__main__":
    unittest.main()
