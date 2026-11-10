#!/usr/bin/env python3
"""Write the golden files of the horus MCP server, made from the Python code.

    <horus venv python> tools/make_server_golden.py

It writes two files that the Go tests compare with:

    internal/server/testdata/tools.golden.json
        It starts the Python server (python -m horus.server) over stdio with
        the official MCP client and reads initialize and tools/list: the
        server name, the instructions text, and for each tool its name, title,
        description, annotations, input schema and output schema.
    internal/herr/testdata/errors.golden.json
        The table ERRORS of horus/errors.py: for each code, retryable,
        action_by and next_step, in table order.
    internal/catalog/testdata/it-illustration.golden.json
        catalog.summary and catalog.detail of workflows/it-illustration, as
        JSON text in key order. The folder of the workflow is written as
        <dir>, and every path uses "/".

Task G10 deletes this tool together with the Python server.
"""

from __future__ import annotations

import asyncio
import json
import sys
from pathlib import Path

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

ROOT = Path(__file__).resolve().parent.parent
TOOLS = ROOT / "internal" / "server" / "testdata" / "tools.golden.json"
ERRORS = ROOT / "internal" / "herr" / "testdata" / "errors.golden.json"
CATALOG = ROOT / "internal" / "catalog" / "testdata" / "it-illustration.golden.json"


async def dump_tools() -> dict:
    params = StdioServerParameters(command=sys.executable, args=["-m", "horus.server"], cwd=str(ROOT),
                                   env={"PYTHONUTF8": "1", "HORUS_AUTOSTART": "0"})
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as s:
            init = await s.initialize()
            tools = (await s.list_tools()).tools
            return {
                "server": init.server_info.name,
                "version": init.server_info.version,
                "instructions": init.instructions,
                "tools": [t.model_dump(by_alias=True, exclude_none=True, mode="json") for t in sorted(tools, key=lambda t: t.name)],
            }


def dump_errors() -> list:
    sys.path.insert(0, str(ROOT))
    from horus.errors import ERRORS as table
    return [{"code": code, "retryable": r, "action_by": a, "next_step": s} for code, (r, a, s) in table.items()]


def dump_catalog() -> dict:
    import os
    os.environ["HORUS_WORKFLOWS_DIR"] = str(ROOT / "workflows")
    sys.path.insert(0, str(ROOT))
    from horus import catalog
    w = catalog.get("it-illustration")

    def text(v) -> str:
        raw = json.dumps(v, ensure_ascii=False)
        return raw.replace(json.dumps(str(w.dir))[1:-1], "<dir>").replace("\\\\", "/")

    return {"summary": text(catalog.summary(w)), "detail": text(catalog.detail(w))}


def write(path: Path, data) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=1, ensure_ascii=False, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {path.relative_to(ROOT).as_posix()}")


def main() -> int:
    write(TOOLS, asyncio.run(dump_tools()))
    write(ERRORS, dump_errors())
    write(CATALOG, dump_catalog())
    return 0


if __name__ == "__main__":
    sys.exit(main())
