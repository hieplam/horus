"""The horus MCP server (stdio).

Tools:
    list_workflows      the image workflows on this machine, and if each one is installed
    describe_workflow   one workflow in full: sizes, how to write the subject, examples, license
    install_workflow    download and verify the model files of a workflow
    uninstall_workflow  delete the model files of a workflow
    generate_image      make one image with a workflow; waits until the PNG is on disk

Every tool returns the same envelope, in structuredContent and as JSON text:
    {"ok": true,  "result": {...}, "error": null}
    {"ok": false, "result": null,  "error": {"code", "message", "retryable", "action_by", "next_step", "details"}}
A failed call also sets isError. The error codes are in horus/errors.py and docs/mcp.md.

Run: horus-mcp   (or: python -m horus.server)
"""

from __future__ import annotations

import asyncio
import json
import logging
import sys
import traceback
from typing import Annotated, Any, Awaitable, Callable, Generic, Literal, TypeVar

from mcp.server.mcpserver import Context, MCPServer
from mcp_types import CallToolResult, TextContent, ToolAnnotations
from pydantic import BaseModel, Field

from . import __version__, catalog, models, settings
from .comfy import Comfy
from .errors import HorusError
from .generate import generate_image as _generate

log = logging.getLogger("horus")

INSTRUCTIONS = """\
horus makes images on this machine with local AI models. You do not need to know how.
1. Call list_workflows. Each workflow is one image model and graph; pick the one whose summary and good_for fit.
2. Call describe_workflow for that workflow once, and read subject.rules and subject.examples.
   horus adds no style: it sends your subject to the model exactly as written. You own the look,
   the palette, the light and the composition, so write them in the subject.
3. Call generate_image. It waits until the PNG is on disk (often 3 to 8 minutes, longer on a cold start)
   and returns its absolute path. Open the file and check it against what you asked.
Every result has ok, result and error. On ok=false, read error.code, error.action_by and error.next_step:
action_by "retry" = call again, "caller" = change your arguments, "human" = ask the user to do error.next_step.
Never install or uninstall a workflow without the user's consent: those tools ask for it."""

mcp = MCPServer(name="horus", instructions=INSTRUCTIONS, version=__version__)

T = TypeVar("T")


# -- result models ---------------------------------------------------------------------------------

class ErrorInfo(BaseModel):
    code: str = Field(description="Stable error code, for example WORKFLOW_NOT_INSTALLED. See docs/mcp.md.")
    message: str = Field(description="What went wrong, for a person.")
    retryable: bool = Field(description="True: the same call can pass when it is sent again.")
    action_by: Literal["caller", "retry", "human"] = Field(
        description="caller: change the arguments. retry: send the same call again. human: the user must act first.")
    next_step: str = Field(description="The exact thing to do next.")
    details: dict[str, Any] = Field(default_factory=dict, description="Facts: file names, allowed values, backend message.")


class Envelope(BaseModel, Generic[T]):
    ok: bool
    result: T | None = None
    error: ErrorInfo | None = None


class WorkflowSummary(BaseModel):
    name: str = Field(description="Pass this to the other tools.")
    title: str
    summary: str
    good_for: list[str]
    not_good_for: list[str]
    output: dict[str, Any] = Field(description="format, default_width, default_height")
    typical_seconds: list[int] | None = Field(description="Measured time for one image, [fastest, slowest].")
    install_state: Literal["installed", "partially_installed", "not_installed"]
    download_bytes: int = Field(description="Total size of the model files.")


class ListResult(BaseModel):
    workflows: list[WorkflowSummary]
    backend_running: bool = Field(description="False: the next generate_image starts the backend first (1 to 3 minutes more).")


class InstallFile(BaseModel):
    role: str
    file: str
    bytes: int
    state: str = Field(description="present, missing, wrong_size, downloaded, deleted, would_delete, kept_shared, absent")
    path: str | None = None
    detail: str | None = None


class DescribeResult(BaseModel):
    name: str
    title: str
    summary: str
    good_for: list[str]
    not_good_for: list[str]
    output: dict[str, Any] = Field(description="format, default size, min_side, max_side, multiple_of")
    subject: dict[str, Any] = Field(description="min_chars, max_chars, how_to_write, rules, examples (subject, image path, seed)")
    timing: dict[str, Any] | None
    typical_seconds: list[int] | None
    license: dict[str, Any] = Field(description="name, url, download_bytes")
    install_state: Literal["installed", "partially_installed", "not_installed"]
    files: list[InstallFile]


class InstallResult(BaseModel):
    workflow: str
    install_state: Literal["installed"]
    already_installed: bool
    files: list[InstallFile]
    seconds: float


class UninstallResult(BaseModel):
    workflow: str
    files: list[InstallFile]
    freed_bytes: int


class ImageResult(BaseModel):
    workflow: str
    path: str = Field(description="Absolute path of the PNG on this machine.")
    bytes: int
    width: int
    height: int
    seed: int = Field(description="Pass the same seed and subject again to get the same image.")
    seconds: float = Field(description="Wall time of this call.")
    backend_seconds: float | None = Field(description="Time the GPU spent on the job.")
    started_backend: bool = Field(description="True: this call had to start the backend first.")
    job_id: str


# -- helpers ---------------------------------------------------------------------------------------

def _ok(model: BaseModel, envelope: type[BaseModel]) -> CallToolResult:
    env = envelope(ok=True, result=model)
    data = env.model_dump(mode="json")
    return CallToolResult(content=[TextContent(type="text", text=json.dumps(data, ensure_ascii=False, indent=1))],
                          structured_content=data)


def _fail(err: HorusError) -> CallToolResult:
    data = {"ok": False, "result": None, "error": err.to_dict()}
    return CallToolResult(content=[TextContent(type="text", text=json.dumps(data, ensure_ascii=False, indent=1))],
                          structured_content=data, is_error=True)


async def _guard(ctx: Context | None, name: str, body: Callable[[], Awaitable[CallToolResult]]) -> CallToolResult:
    meta = getattr(getattr(ctx, "request_context", None), "meta", None) if ctx else None
    log.info("call %s progress_token=%s", name, getattr(meta, "progress_token", None))
    try:
        return await body()
    except HorusError as err:
        log.info("call %s failed: %s", name, err)
        return _fail(err)
    except Exception as exc:  # a bug: still return the envelope, never a bare traceback
        log.error("call %s crashed\n%s", name, traceback.format_exc())
        return _fail(HorusError("INTERNAL", f"{type(exc).__name__}: {exc}", {"tool": name}))


def _reporter(ctx: Context | None):
    async def report(progress: float, total: float | None, message: str) -> None:
        log.info("progress %s %s", progress, message)
        if ctx is None:
            return
        try:
            await ctx.report_progress(progress, total, message)
        except Exception:
            pass
    return report


def _size(n: int) -> str:
    return f"{n / 1e9:.1f} GB" if n >= 1e8 else f"{n / 1e6:.2f} MB"


def _file_rows(items: list[dict[str, Any]], kind: str, state_map: dict[str, str]) -> list[InstallFile]:
    rows = []
    for i in items:
        if i.get("kind") != kind or i["status"] not in state_map:
            continue
        rows.append(InstallFile(role=i.get("role") or "", file=i.get("file") or "", bytes=max(0, int(i.get("bytes", 0))),
                                state=state_map[i["status"]], path=i.get("path") or None, detail=i.get("detail") or None))
    return rows


def _status_rows(w: catalog.Workflow) -> list[InstallFile]:
    # A failed check (a missing file) is a normal answer here, so read the items and not res["ok"].
    run = models.run_script("check", w.name, ["-Quick"])
    items = (run.result or {}).get("items", [])
    return _file_rows(items, "file", {"OK": "present", "MISSING": "missing", "BAD_SIZE": "wrong_size"})


# -- tools -----------------------------------------------------------------------------------------

@mcp.tool(
    title="List image workflows",
    description=(
        "List the image workflows (model + graph) on this machine. Each entry has the name to pass "
        "to the other tools, a summary, what the workflow is good and bad for, the output size, the measured "
        "time per image, and install_state. Call this first."),
    annotations=ToolAnnotations(read_only_hint=True, idempotent_hint=True, open_world_hint=False),
)
async def list_workflows(ctx: Context) -> Annotated[CallToolResult, Envelope[ListResult]]:
    async def body() -> CallToolResult:
        flows = catalog.all_workflows()
        status = await asyncio.to_thread(models.list_status)
        running = await asyncio.to_thread(Comfy().is_up)
        rows = []
        for w in flows:
            row = status.get(w.name)
            rows.append(WorkflowSummary(**catalog.summary(w), install_state=models.install_state(row),
                                        download_bytes=int((row or {}).get("bytes") or w.license()["download_bytes"])))
        return _ok(ListResult(workflows=rows, backend_running=running), Envelope[ListResult])
    return await _guard(ctx, "list_workflows", body)


@mcp.tool(
    title="Describe an image workflow",
    description=(
        "Everything a caller needs to use one image workflow: the allowed sizes, how to write the subject (rules "
        "and example subjects with their images), the license and the install state of each model file. The workflow "
        "has no style of its own. Read subject.rules before the first generate_image with this workflow."),
    annotations=ToolAnnotations(read_only_hint=True, idempotent_hint=True, open_world_hint=False),
)
async def describe_workflow(
    ctx: Context,
    workflow: Annotated[str, Field(description="Name from list_workflows, for example 'it-illustration'.")],
) -> Annotated[CallToolResult, Envelope[DescribeResult]]:
    async def body() -> CallToolResult:
        w = catalog.get(workflow)
        files = await asyncio.to_thread(_status_rows, w)
        present = sum(1 for f in files if f.state == "present")
        state = "installed" if files and present == len(files) else ("partially_installed" if present else "not_installed")
        return _ok(DescribeResult(**catalog.detail(w), install_state=state, files=files), Envelope[DescribeResult])
    return await _guard(ctx, "describe_workflow", body)


@mcp.tool(
    title="Install an image workflow",
    description=(
        "Download and verify (SHA-256) the model files of one image workflow, so that generate_image can use it. "
        "Files that are already present are kept. Downloads are large (several GB) and can take 10 to 60 minutes. "
        "The models have their own license: the first call without accept_license=true returns the error "
        "LICENSE_CONFIRMATION_REQUIRED with the license and the size. Ask the user, then call again with "
        "accept_license=true. The call waits until the install is done and reports progress."),
    annotations=ToolAnnotations(read_only_hint=False, destructive_hint=False, idempotent_hint=True, open_world_hint=True),
)
async def install_workflow(
    ctx: Context,
    workflow: Annotated[str, Field(description="Name from list_workflows.")],
    accept_license: Annotated[bool, Field(description="True only after the user accepted the license shown in the first reply.")] = False,
) -> Annotated[CallToolResult, Envelope[InstallResult]]:
    async def body() -> CallToolResult:
        t0 = asyncio.get_running_loop().time()
        w = catalog.get(workflow)
        row = (await asyncio.to_thread(models.list_status)).get(w.name)
        if models.install_state(row) == "installed":
            res = await asyncio.to_thread(models.check, w, True)
            if res["ok"]:
                files = _file_rows(res["items"], "file", {"OK": "present"})
                return _ok(InstallResult(workflow=w.name, install_state="installed", already_installed=True, files=files,
                                         seconds=round(asyncio.get_running_loop().time() - t0, 1)), Envelope[InstallResult])
        if not accept_license:
            lic = w.license()
            files = [{"role": f.get("role"), "file": f.get("repo_path", "").split("/")[-1], "bytes": f.get("bytes")}
                     for f in w.models().get("files", [])]
            raise HorusError("LICENSE_CONFIRMATION_REQUIRED", f"installing {w.name} downloads {_size(lic['download_bytes'])} under the license '{lic['name']}'",
                             {"license": lic["name"], "license_url": lic["url"], "download_bytes": lic["download_bytes"], "files": files})
        report = _reporter(ctx)
        total = int(w.license()["download_bytes"]) or None
        run = models.Run()
        task = asyncio.create_task(asyncio.to_thread(models.install, w, run))
        try:
            while not task.done():
                await asyncio.wait({task}, timeout=5)
                if not task.done():
                    done = models.part_size(run.current_dest)
                    name = (run.current_dest or "").replace("\\", "/").split("/")[-1]
                    await report(done, total, f"downloading {name}: {done / 1e9:.2f} GB" if name else "checking files")
        except asyncio.CancelledError:
            run.cancel()
            raise
        res = task.result()
        files = _file_rows(res["items"], "download", {"DOWNLOADED": "downloaded", "PRESENT": "present"})
        return _ok(InstallResult(workflow=w.name, install_state="installed", already_installed=False, files=files,
                                 seconds=round(asyncio.get_running_loop().time() - t0, 1)), Envelope[InstallResult])
    return await _guard(ctx, "install_workflow", body)


@mcp.tool(
    title="Uninstall an image workflow",
    description=(
        "Delete the model files of one image workflow to free disk space. A file that another installed workflow also "
        "uses is kept unless remove_shared=true. Without confirm=true nothing is deleted: the call returns the error "
        "CONFIRMATION_REQUIRED with the list of files and sizes. Show it to the user, then call again with confirm=true."),
    annotations=ToolAnnotations(read_only_hint=False, destructive_hint=True, idempotent_hint=True, open_world_hint=False),
)
async def uninstall_workflow(
    ctx: Context,
    workflow: Annotated[str, Field(description="Name from list_workflows.")],
    confirm: Annotated[bool, Field(description="True only after the user agreed to the plan from the first reply.")] = False,
    remove_shared: Annotated[bool, Field(description="Also delete files that other workflows use.")] = False,
) -> Annotated[CallToolResult, Envelope[UninstallResult]]:
    async def body() -> CallToolResult:
        w = catalog.get(workflow)
        if confirm:
            await asyncio.to_thread(Comfy().free)  # an open model file cannot be deleted on Windows
        res = await asyncio.to_thread(models.remove, w, confirm, remove_shared)
        states = {"DELETED": "deleted", "WOULD_DELETE": "would_delete", "KEPT_SHARED": "kept_shared", "ABSENT": "absent"}
        files = _file_rows(res["items"], "file", states)
        if not confirm and any(f.state == "would_delete" for f in files):
            raise HorusError("CONFIRMATION_REQUIRED", "nothing was deleted: the user must confirm the plan",
                             {"files": [f.model_dump() for f in files],
                              "would_free_bytes": sum(f.bytes for f in files if f.state == "would_delete")})
        freed = sum(f.bytes for f in files if f.state == "deleted")
        return _ok(UninstallResult(workflow=w.name, files=files, freed_bytes=freed), Envelope[UninstallResult])
    return await _guard(ctx, "uninstall_workflow", body)


@mcp.tool(
    title="Generate an image",
    description=(
        "Make one image with the given workflow and save it as a PNG on this machine. The call waits until the image is "
        "done (often 3 to 8 minutes; the first call after a cold start takes longer) and reports progress. "
        "The subject is the whole prompt: what the picture shows and how it looks (style, palette, light, "
        "composition). horus sends it to the model exactly as written and adds nothing. The result has the absolute path of the PNG; open "
        "it and check it. The same subject and seed give the same image."),
    annotations=ToolAnnotations(read_only_hint=False, destructive_hint=False, idempotent_hint=False, open_world_hint=False),
)
async def generate_image(
    ctx: Context,
    workflow: Annotated[str, Field(description="Name from list_workflows, for example 'it-illustration'.")],
    subject: Annotated[str, Field(description="The whole prompt, sent to the model as written: content and style. Follow subject.rules of describe_workflow.")],
    out_path: Annotated[str | None, Field(description="Absolute path of the PNG to write. Default: a new file in the horus output folder.")] = None,
    seed: Annotated[int | None, Field(description="Integer for a repeatable image. Default: a random seed (returned in the result).")] = None,
    width: Annotated[int | None, Field(description="Pixels. Default and limits: describe_workflow output.")] = None,
    height: Annotated[int | None, Field(description="Pixels. Default and limits: describe_workflow output.")] = None,
    overwrite: Annotated[bool, Field(description="Replace out_path when it exists.")] = False,
) -> Annotated[CallToolResult, Envelope[ImageResult]]:
    async def body() -> CallToolResult:
        res = await _generate(workflow, subject, seed=seed, width=width, height=height, out_path=out_path,
                              overwrite=overwrite, report=_reporter(ctx))
        return _ok(ImageResult(**res), Envelope[ImageResult])
    return await _guard(ctx, "generate_image", body)


def main() -> None:
    fmt = "%(asctime)s horus %(levelname)s %(message)s"
    logging.basicConfig(stream=sys.stderr, level=logging.INFO, format=fmt)
    log_file = settings.get("HORUS_LOG_FILE")  # optional: a copy of the log, for a harness that hides stderr
    if log_file:
        handler = logging.FileHandler(log_file, encoding="utf-8")
        handler.setFormatter(logging.Formatter(fmt))
        log.addHandler(handler)
    mcp.run("stdio")


if __name__ == "__main__":
    main()
