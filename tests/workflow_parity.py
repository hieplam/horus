#!/usr/bin/env python3
"""Parity test of `horus workflow` against scripts/workflow.ps1. No GPU, no network.

    python tests/workflow_parity.py [--record | --live] [--horus <path>] [--case <name>]

Each case builds a fresh sandbox (make_sandbox of tests/mcp_contract.py: a
temporary folder with a copy of the workflows and tiny stand-in model files),
starts a fake ComfyUI that answers /object_info when the case needs it, and runs
one or more steps of the workflow command with -json.

For each step the tool keeps the exit code, the result (the HORUS_JSON line, or
the JSON list of `list -json`) and the other output lines. After the last step
it keeps the files of the sandbox's ComfyUI folder (size and SHA-256, with
CRLF counted as LF) and the text of extra_model_paths.yaml. Paths are normalised: the sandbox folder becomes
<sandbox> and every backslash becomes a slash, so the golden files hold no user
path and no OS separator.

Modes:
  default   run `horus workflow` and compare with the golden files.
  --record  run workflow.ps1 and write the golden files (Windows only).
  --live    also run workflow.ps1 and check that the golden files are still true.

It prints EQUAL or DIFFERENT for each case, then "parity: N of M cases equal".
Exit code 0 only when all cases are equal. N counts only cases that it compared.

Allowed differences (rule 3 of the spec): the messages of workflow.ps1 name
.ps1 scripts and PowerShell flags. The table REWRITE turns them into the horus
subcommand and the Go flag before the comparison. Nothing else is rewritten.
"""

from __future__ import annotations

import argparse
import difflib
import hashlib
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "tests"))
from mcp_contract import make_sandbox  # noqa: E402

GOLDEN = ROOT / "tests" / "fixtures" / "workflow_parity"
FIXTURE = json.loads((ROOT / "tests" / "fixtures" / "object_info.json").read_text(encoding="utf-8"))
MARK = "HORUS_JSON "
GO_DIRS = [r"C:\Program Files\Go\bin", "/usr/local/go/bin"]

# PowerShell text -> horus text. Rule 3 of the spec: a message that named a .ps1
# script names the horus subcommand, and flags take Go style.
REWRITE = [
    (".\\scripts\\workflow.ps1", "horus workflow"),
    (".\\scripts\\start-comfyui.ps1", "horus start-comfyui"),
    (".\\scripts\\install-comfyui.ps1", "horus install-comfyui"),
    ("-ComfyUIDir", "-comfyui-dir"),
    ("-Force", "-force"),
    ("-Yes", "-yes"),
    ("-Only", "-only"),
]

# The flags of a step, in the form of each program.
PS_FLAGS = {"quick": "-Quick", "yes": "-Yes", "force": "-Force", "only": "-Only"}
GO_FLAGS = {"quick": "-quick", "yes": "-yes", "force": "-force", "only": "-only"}

IT, ABSENT = "it-illustration", "absent-style"


# --- Sandbox changes, made in code before the steps -------------------------------------------

def models_dir(sb: dict) -> Path:
    return Path(sb["models"])


def grow_vae(sb: dict) -> None:
    """The VAE file on disk has one byte more than models.json says."""
    p = models_dir(sb) / "vae" / "flux2-vae.safetensors"
    p.write_bytes(p.read_bytes() + b"!")


def swap_text_encoder(sb: dict) -> None:
    """The text encoder file has the right size and other bytes."""
    p = models_dir(sb) / "text_encoders" / "qwen3vl_4b_int8_convrot.safetensors"
    data = p.read_bytes()
    p.write_bytes(bytes(reversed(data)))


def unlisted_file(sb: dict) -> None:
    """workflow.json of it-illustration loads one more file that models.json does not list."""
    p = Path(sb["workflows"]) / IT / "workflow.json"
    graph = json.loads(p.read_text(encoding="utf-8"))
    graph.setdefault("extra", {})["parity_test"] = "loras\\extra-lora.safetensors"
    p.write_text(json.dumps(graph), encoding="utf-8")


def no_comfyui(sb: dict) -> None:
    (Path(sb["comfy"]) / "ComfyUI" / "main.py").unlink()


def part_file(sb: dict) -> None:
    """An unfinished download of the absent model of absent-style."""
    p = models_dir(sb) / "diffusion_models" / "absent-model.safetensors.part"
    p.write_bytes(b"x" * 1234)


def server_without_vae_loader() -> dict:
    info = dict(FIXTURE)
    info.pop("VAELoader", None)
    return info


# name, steps, sandbox change, server ("fixture", "none", or a function that returns /object_info).
# A step is (action, workflow or None, [flags]); a flag is a name or (name, value).
CASES = [
    ("list", [("list", None, [])], None, "fixture"),
    ("check-quick", [("check", IT, ["quick"])], None, "none"),
    ("check", [("check", IT, [])], None, "fixture"),
    ("install-installed", [("install", IT, [])], None, "fixture"),
    ("check-absent", [("check", ABSENT, [])], None, "fixture"),
    ("remove-plan", [("remove", IT, [])], None, "fixture"),
    ("remove-yes", [("remove", IT, ["yes"])], None, "fixture"),
    ("remove-yes-force", [("remove", IT, ["yes", "force"])], None, "fixture"),
    ("list-after-remove", [("remove", IT, ["yes", "force"]), ("list", None, [])], None, "fixture"),
    ("check-only-vae", [("check", IT, [("only", "vae")])], None, "fixture"),
    # More cases than the plan names: the statuses that the 10 cases above do not reach.
    ("check-bad-size", [("check", IT, [])], grow_vae, "fixture"),
    ("check-bad-hash", [("check", IT, [])], swap_text_encoder, "fixture"),
    ("remove-refused", [("remove", IT, ["yes", "force"])], grow_vae, "fixture"),
    ("check-unlisted-no-node", [("check", IT, [])], unlisted_file, server_without_vae_loader),
    ("install-no-comfyui", [("install", IT, [])], no_comfyui, "fixture"),
    ("remove-part-file", [("remove", ABSENT, ["yes"])], part_file, "fixture"),
    ("list-text", [("list", None, ["text"])], None, "fixture"),
]


# --- Fake ComfyUI --------------------------------------------------------------------------------

class FakeComfy:
    def __init__(self, info: dict | None):
        self.info = info
        self.server = None
        if info is None:
            # A port that nothing listens on.
            s = socket.socket()
            s.bind(("127.0.0.1", 0))
            self.port = s.getsockname()[1]
            s.close()
            return
        body = json.dumps(info).encode()

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):  # noqa: N802
                if self.path.split("?")[0] == "/object_info":
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                else:
                    self.send_response(404)
                    self.send_header("Content-Length", "0")
                    self.end_headers()

            def log_message(self, *a):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self) -> None:
        if self.server:
            self.server.shutdown()
            self.server.server_close()


# --- Running one program -------------------------------------------------------------------------

def step_args(step, flags_of: dict, json_flag: str, wf_flag: str, wf: str) -> list[str]:
    action, name, flags = step
    out = [action] + ([name] if name else [])
    if "text" not in flags:
        out.append(json_flag)
    out += [wf_flag, wf]
    for f in flags:
        if f == "text":
            continue
        if isinstance(f, tuple):
            out += [flags_of[f[0]], f[1]]
        else:
            out.append(flags_of[f])
    return out


def child_env(tmp: Path, sb: dict, port: int) -> dict:
    env = {k: v for k, v in os.environ.items() if not k.startswith("HORUS_") and k != "HF_TOKEN"}
    env.update({
        "HORUS_ROOT": str(tmp),  # no .env of a real checkout is read
        "HORUS_WORKFLOWS_DIR": sb["workflows"], "HORUS_COMFYUI_DIR": sb["comfy"],
        "HORUS_MODELS_DIR": sb["models"], "HORUS_TEXT_ENCODER_DIR": str(Path(sb["models"]) / "text_encoders"),
        "HORUS_COMFYUI_URL": f"http://127.0.0.1:{port}", "HORUS_AUTOSTART": "0",
    })
    return env


def normalise(text: str, tmp: Path) -> str:
    for form in sorted({str(tmp), str(tmp.resolve()), os.path.realpath(tmp)}, key=len, reverse=True):
        text = re.sub(re.escape(form), "<sandbox>", text, flags=re.I)
        text = re.sub(re.escape(form.replace("\\", "/")), "<sandbox>", text, flags=re.I)
    text = text.replace("\\", "/")
    text = re.sub(r"127\.0\.0\.1:\d+", "127.0.0.1:<port>", text)  # the fake ComfyUI has a new port each run
    return re.sub(r"\.bak-\d{8}-\d{6}", ".bak-<time>", text)


def rewrite(text: str) -> str:
    for old, new in REWRITE:
        text = text.replace(old, new)
    return text


def norm_json(value, tmp: Path, ps: bool):
    if isinstance(value, str):
        return normalise(rewrite(value) if ps else value, tmp)
    if isinstance(value, list):
        return [norm_json(v, tmp, ps) for v in value]
    if isinstance(value, dict):
        return {k: norm_json(v, tmp, ps) for k, v in value.items()}
    return value


def run_step(cmd: list[str], step, env: dict, tmp: Path, ps: bool) -> dict:
    p = subprocess.run(cmd, cwd=str(tmp), env=env, stdin=subprocess.DEVNULL, capture_output=True, timeout=300)
    out = p.stdout.decode("utf-8", "replace")
    lines = [ln.rstrip("\r") for ln in out.split("\n")]
    while lines and lines[-1] == "":
        lines.pop()
    action, _, flags = step
    result, text = None, []
    if action == "list" and "text" not in flags and p.returncode == 0:
        try:
            result = json.loads(out)
        except ValueError:
            text = lines
    else:
        for ln in lines:
            if ln.startswith(MARK):
                result = json.loads(ln[len(MARK):])
            else:
                text.append(ln)
    text = [normalise(rewrite(ln) if ps else ln, tmp) for ln in text]
    return {"exit": p.returncode, "result": norm_json(result, tmp, ps), "lines": text}


def disk_state(tmp: Path, sb: dict) -> dict:
    comfy = Path(sb["comfy"])
    files = []
    for p in sorted(comfy.rglob("*")):
        if p.is_file():
            rel = normalise(str(p.relative_to(tmp)), tmp)
            # make_sandbox writes main.py in text mode: CRLF on Windows, LF elsewhere. Count LF line ends.
            data = p.read_bytes().replace(b"\r\n", b"\n")
            files.append(f"{re.sub(r'.bak-[0-9-]+$', '.bak-<time>', rel)} {len(data)} {hashlib.sha256(data).hexdigest()[:12]}")
    yaml = comfy / "ComfyUI" / "extra_model_paths.yaml"
    return {"files": files, "extra_model_paths.yaml": normalise(yaml.read_text(encoding="utf-8"), tmp) if yaml.is_file() else None}


def run_case(case, program: str, horus: str | None) -> dict:
    name, steps, change, server = case
    tmp = Path(tempfile.mkdtemp(prefix="horus-parity-")).resolve()
    fake = None
    try:
        sb = make_sandbox(tmp)
        if program == "ps":
            (tmp / "scripts").mkdir()
            for s in ("workflow.ps1", "download-model.ps1"):
                shutil.copy2(ROOT / "scripts" / s, tmp / "scripts" / s)  # repo root of the script = sandbox
        if change:
            change(sb)
        info = FIXTURE if server == "fixture" else (None if server == "none" else server())
        fake = FakeComfy(info)
        env = child_env(tmp, sb, fake.port)
        results = []
        for step in steps:
            if program == "ps":
                cmd = ["powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
                       "-File", str(tmp / "scripts" / "workflow.ps1")]
                cmd += step_args(step, PS_FLAGS, "-Json", "-WorkflowsDir", sb["workflows"])
            else:
                cmd = [horus, "workflow"] + step_args(step, GO_FLAGS, "-json", "-workflows-dir", sb["workflows"])
            results.append(run_step(cmd, step, env, tmp, program == "ps"))
        return {"case": name, "steps": [{"args": step_args(s, GO_FLAGS, "-json", "-workflows-dir", "<sandbox>/workflows"),
                                          **r} for s, r in zip(steps, results)],
                "after": disk_state(tmp, sb)}
    finally:
        if fake:
            fake.close()
        shutil.rmtree(tmp, ignore_errors=True)


# --- Main ----------------------------------------------------------------------------------------

def go_exe() -> str | None:
    found = shutil.which("go")
    if found:
        return found
    for d in GO_DIRS:
        for n in ("go.exe", "go"):
            if (Path(d) / n).is_file():
                return str(Path(d) / n)
    return None


def build_horus(out_dir: Path) -> str:
    go = go_exe()
    if not go:
        sys.exit("workflow_parity: go not found; pass --horus <path>")
    exe = out_dir / ("horus.exe" if os.name == "nt" else "horus")
    p = subprocess.run([go, "build", "-o", str(exe), "./cmd/horus"], cwd=ROOT, capture_output=True, text=True)
    if p.returncode != 0:
        sys.exit(f"workflow_parity: go build failed:\n{p.stdout}{p.stderr}")
    return str(exe)


def dump(obj) -> str:
    return json.dumps(obj, indent=1, ensure_ascii=False, sort_keys=False) + "\n"


def show_diff(want: dict, got: dict, label: str) -> None:
    a, b = dump(want).splitlines(), dump(got).splitlines()
    for ln in list(difflib.unified_diff(a, b, "golden", label, lineterm="", n=1))[:40]:
        print("    " + ln)


def main() -> int:
    ap = argparse.ArgumentParser()
    mode = ap.add_mutually_exclusive_group()
    mode.add_argument("--record", action="store_true", help="run workflow.ps1 and write the golden files")
    mode.add_argument("--live", action="store_true", help="also check the golden files against workflow.ps1")
    ap.add_argument("--horus", help="path of the horus binary (default: go build ./cmd/horus)")
    ap.add_argument("--case", action="append", help="run only this case (several allowed)")
    a = ap.parse_args()

    cases = [c for c in CASES if not a.case or c[0] in a.case]
    if not cases:
        sys.exit(f"workflow_parity: no case named {a.case}")
    if (a.record or a.live) and os.name != "nt":
        sys.exit("workflow_parity: --record and --live need Windows (powershell.exe)")

    if a.record:
        GOLDEN.mkdir(parents=True, exist_ok=True)
        for c in cases:
            res = run_case(c, "ps", None)
            (GOLDEN / f"{c[0]}.json").write_text(dump(res), encoding="utf-8", newline="\n")
            print(f"RECORDED  {c[0]}")
        print(f"recorded: {len(cases)} cases")
        return 0

    build_dir = Path(tempfile.mkdtemp(prefix="horus-parity-bin-"))
    try:
        horus = str(Path(a.horus).resolve()) if a.horus else build_horus(build_dir)
        equal = compared = 0
        for c in cases:
            gfile = GOLDEN / f"{c[0]}.json"
            if not gfile.is_file():
                print(f"DIFFERENT {c[0]}  -- no golden file {gfile.name}")
                continue
            want = json.loads(gfile.read_text(encoding="utf-8"))
            compared += 1
            ok = True
            if a.live:
                ps = run_case(c, "ps", None)
                if ps != want:
                    ok = False
                    print(f"DIFFERENT {c[0]}  -- workflow.ps1 no longer gives the golden file")
                    show_diff(want, ps, "workflow.ps1")
            got = run_case(c, "go", horus)
            if got != want:
                ok = False
                print(f"DIFFERENT {c[0]}")
                show_diff(want, got, "horus workflow")
            if ok:
                equal += 1
                print(f"EQUAL     {c[0]}")
        print(f"parity: {equal} of {compared} cases equal")
        return 0 if equal == compared == len(cases) else 1
    finally:
        shutil.rmtree(build_dir, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
