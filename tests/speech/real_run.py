"""real_run: an MCP client for the goal row `speech-real` (docs/specs/2026-11-20-text-to-speech.md).

It starts `<horus> mcp` over stdio and calls the tools of the horus MCP server, as an agent does. It prints one
JSON document on its standard output: the raw `structuredContent` of each call. It measures nothing and judges
nothing: the goal suite (goals/2026-11-20-text-to-speech) measures the loudness with ffmpeg and computes the
share of wrong words itself.

    python tests/speech/real_run.py install --horus <horus program>
    python tests/speech/real_run.py speak   --horus <horus program> --out <folder>

`install` calls `install_workflow` for `vi-speech` and gives both consents (`accept_setup`, `accept_license`).
The owner approved the three models and their licenses on 2026-11-20. It uses the real speech folder of the
user (`HORUS_SPEECH_DIR`, default `<user folder>/speech`).
`speak` calls `generate_speech` for each sentence of tests/speech/sentences.json, with the default voice.

Needs the Python package `mcp` of tests/requirements.txt.
"""
from __future__ import annotations

import argparse
import asyncio
import json
import os
import sys
from pathlib import Path

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

HERE = Path(__file__).resolve().parent
WORKFLOW = "vi-speech"
LIMIT = 3600.0  # seconds


async def call(session: ClientSession, name: str, args: dict) -> dict:
    res = await session.call_tool(name, args, read_timeout_seconds=LIMIT)
    if res.structured_content is None:
        sys.exit(f"real_run: {name} gave no structuredContent: {res.content}")
    return res.structured_content


async def run(args: argparse.Namespace) -> dict:
    env = dict(os.environ)
    if args.output_dir:
        env["HORUS_OUTPUT_DIR"] = str(Path(args.output_dir).resolve())
    params = StdioServerParameters(command=str(Path(args.horus).resolve()), args=["mcp"], env=env)
    async with stdio_client(params, errlog=sys.stderr) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            names = [t.name for t in (await session.list_tools()).tools]
            out: dict = {"tools": names}
            if args.mode == "install":
                first = await call(session, "install_workflow", {"workflow": WORKFLOW})
                out["first"] = first
                second = await call(session, "install_workflow", {"workflow": WORKFLOW, "accept_setup": True})
                out["second"] = second
                out["result"] = await call(session, "install_workflow",
                                           {"workflow": WORKFLOW, "accept_setup": True, "accept_license": True})
                return out
            sentences = json.loads((HERE / "sentences.json").read_text(encoding="utf-8"))["sentences"]
            out["calls"] = []
            for s in sentences:
                target = Path(args.out).resolve() / f"{s['id']}.wav"
                call_args = {"workflow": WORKFLOW, "text": s["text"], "out_path": str(target), "overwrite": True}
                out["calls"].append({"id": s["id"], "text": s["text"], "answer": await call(session, "generate_speech", call_args)})
            return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("mode", choices=["install", "speak"])
    ap.add_argument("--horus", required=True, help="the horus program")
    ap.add_argument("--out", help="speak: the folder for the audio files")
    ap.add_argument("--output-dir", help="HORUS_OUTPUT_DIR of the server")
    args = ap.parse_args()
    if args.mode == "speak":
        if not args.out:
            sys.exit("real_run: speak needs --out")
        Path(args.out).mkdir(parents=True, exist_ok=True)
    sys.stdout.reconfigure(encoding="utf-8")
    print(json.dumps(asyncio.run(run(args)), ensure_ascii=False, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
