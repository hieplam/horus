#!/usr/bin/env python3
"""Write the golden file of the graph conversion, made from the Python code.

    <horus venv python> tools/make_graph_golden.py

It writes tests/fixtures/ui_to_api.golden.json: the output of the Python
ui_to_api (horus/comfy.py) for workflows/it-illustration/workflow.json and the
node definitions in tests/fixtures/object_info.json. The Go test
TestUIToAPIGolden (internal/comfy) compares the Go ui_to_api with this file.

Task G10 deletes this tool together with the Python package.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "tests" / "fixtures" / "ui_to_api.golden.json"


def main() -> int:
    sys.path.insert(0, str(ROOT))
    from horus.comfy import ui_to_api

    graph = json.loads((ROOT / "workflows" / "it-illustration" / "workflow.json").read_text(encoding="utf-8"))
    info = json.loads((ROOT / "tests" / "fixtures" / "object_info.json").read_text(encoding="utf-8"))
    api = ui_to_api(graph, info)
    OUT.write_text(json.dumps(api, indent=1, ensure_ascii=False) + "\n", encoding="utf-8", newline="\n")
    print(f"wrote {OUT.relative_to(ROOT).as_posix()}: {len(api)} nodes")
    return 0


if __name__ == "__main__":
    sys.exit(main())
