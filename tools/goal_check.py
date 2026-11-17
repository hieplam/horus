#!/usr/bin/env python3
"""The goal check: it runs the goal rows of one suite and prints PASS or FAIL for each row.

    python tools/goal_check.py --list                     the suites
    python tools/goal_check.py --suite <name> [options]   the rows of tools/suites/<name>.py
    python tools/goal_check.py --phase 1|2|3|4 [options]  the same as --suite phase<N> (no option: phase1)

tools/goal_runner.py runs the rows and documents the fields of a suite. A new spec gets a new
suite file in tools/suites/. Run `python tools/goal_check.py -h` for the options.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import goal_runner  # noqa: E402

if __name__ == "__main__":
    sys.exit(goal_runner.main())
