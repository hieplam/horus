# horus: rules for AI agents

You are an agent (Claude Code, Codex, or similar) that drives a local ComfyUI
for a user. The user describes an outcome. You do the work through the comfy
MCP tools and the `horus` program of this repo. The user does not need to know ComfyUI.

## Read before you act

| Task | Read |
|---|---|
| Set up or repair a machine | [`docs/setup.md`](docs/setup.md) |
| `/mcp` shows `comfy-mcp` or `horus` as failed | Its program is not installed, or `horus` is not on the PATH. `.mcp.json` starts both servers through `horus` (`horus comfy-mcp` and `horus mcp`) on Windows and macOS. Run `horus doctor`, then the command that its FAIL line names (`go install ./cmd/horus` for `horus`, [step 0 of setup](docs/setup.md#step-0-get-the-horus-program)). Then ask the user to restart `claude`. |
| Set up one workflow on a machine | `horus setup <workflow>` prints the plan and changes nothing. `horus setup <workflow> -yes` runs the plan: ComfyUI first, then the model files. The older names (`check-setup`, `install-comfyui`, `workflow install`) still work. Use `doctor` and `setup`. |
| Stop the ComfyUI that horus started | `horus stop-comfyui`. It refuses a ComfyUI that a person started. |
| Make an image with an existing workflow | `workflows/<name>/README.md` |
| A new style, a new model, a new workflow | [`docs/style-to-workflow.md`](docs/style-to-workflow.md) |
| A run fails | [`docs/running-workflows.md`](docs/running-workflows.md) |
| Which model pairs with which | [`docs/models.md`](docs/models.md) |
| Why this model, and GPU traps | [`docs/why-flux3-klein.md`](docs/why-flux3-klein.md) |
| A term you must explain to the user | [`docs/concepts.md`](docs/concepts.md) |
| Change the horus MCP server, its tools or its errors | [`docs/mcp.md`](docs/mcp.md), then run `go test ./...` and `tests/mcp_contract.py` |
| The flags of a `horus` command | `horus <command> -h` |
| Offer a workflow to other agents | Add `workflows/<name>/manifest.json` ([fields](docs/mcp.md#add-a-workflow-to-the-server)) |

For a tech or IT illustration, use
[`workflows/it-illustration`](workflows/it-illustration/README.md). Its README
tells you how to write the caption.

**Two MCP servers.** The comfy MCP server (`comfy-mcp`) is the low-level tool
for work on this repo: build, check and debug a workflow. The horus MCP server
(`horus`, [`docs/mcp.md`](docs/mcp.md)) is the product for other agents: they
call `list_workflows`, `describe_workflow` and `generate_image` and never see
ComfyUI. When you change a workflow, keep its `manifest.json` true, because the
horus server shows it to every caller.

## Rules

1. **Call `server_info` first**, before any other comfy MCP call.
2. **Call `validate_workflow` before every `run_workflow`.** Read `valid`. Also
   check that `converted_node_count` equals the node count of the graph. A stale
   UI export can check zero nodes and still report `valid: true`.
3. **Run async.** `run_workflow(wait=False)`, then `job(action="wait")`, then
   `fetch_outputs`. A cold model load can take longer than a blocking call allows.
4. **Never hand-edit `workflow.json`.** Use `list_workflow_slots` and
   `set_workflow_slot` on a copy in a scratch folder. Hand edits break the link
   table without an error.
5. **Never commit model files.** Record each model in `docs/models.md` and in the
   workflow's `models.json` with its byte size and SHA-256.
6. **A workflow is not done until it has run.** Validation is not proof. Run it,
   look at the image, then write the evidence in the workflow README.
7. **Save every prompt worth reusing** under `prompts/<family>/`. Prompts do not
   transfer between model families.
8. **Get node names from the live server** with the `nodes` tool. Do not trust
   your memory: node names change between ComfyUI versions.
9. **The user approves a new model** before you download it. Show the model, its
   license and its size first.
10. **Never put a token in a URL, a command line or a log.** Tokens live in `.env`.
11. **A workflow has no style.** horus sends the caller's subject to the model as
    written. Do not add a theme, a palette, a house style or style words to a
    `manifest.json`, a `workflow.json` default or the server. The caller owns the
    look. `tests/mcp_contract.py` fails when a workflow adds a style.

## Explaining to the user

Name the item, say what it does, and say what breaks if it is wrong. Good:
"The VAE must be `flux2-vae` because FLUX 3 uses the FLUX.2 latent space. A
different VAE decodes to coloured noise." Bad: "Configure the VAE."

## Design rules

The full rules and the layer table are in
[`docs/specs/2026-11-11-restructure.md`](docs/specs/2026-11-11-restructure.md).

1. **Simple.** Add no design pattern that hides no complexity.
2. **Deep modules.** A package hides its work behind a small interface.
3. **Pure functions first.** Code that does no input and no output gets no interface.
4. **Interfaces only at input and output.** The service declares the interface that it needs. A test gives it a fake.
5. **One direction.** A handler calls a service. A service calls an adapter through an interface. No call goes back.

`go test ./internal/archtest` enforces the direction, the OS boundary (OS-specific code lives only in `internal/platform` and `internal/backend`) and the token rule (only `internal/download` reads `HF_TOKEN`). The test has no allow list. Fix the code, not the test.

## Layout

```
README.md                         entry point for people
AGENTS.md                         this file
ROADMAP.md                        what comes next
docs/                             setup, style-to-workflow, concepts, running, models, mcp
docs/specs/, docs/plans/          the spec, the plan and the evidence log of each phase
cmd/horus/                        wiring: main.go (the command table) and wiring.go (creates the adapters and gives them to the services)
internal/cli/                     handler: one file for each command group (flags in, text out)
internal/mcpserver/               handler: the 5 MCP tools, the envelope, the progress notifications
internal/generate/                service: make one image
internal/provision/               service: set up a machine (backend, model files, comfy-mcp and `horus comfy-mcp`, register, the plan of `horus setup`)
internal/doctor/                  service: the checks of `horus doctor`
internal/devtools/                service: new-workflow and sync-workflows
internal/comfy/                   adapter: the ComfyUI REST client (comfy/comfytest is the fake ComfyUI for tests)
internal/download/                adapter: download with resume, size and SHA-256 check; the only code that reads HF_TOKEN
internal/modelfiles/              adapter: find, check and remove model files, extra_model_paths.yaml
internal/backend/                 adapter: Install, Start, Stop and Installed of a ComfyUI install (the Windows portable build; comfy-cli on macOS; backend/comfyclitest is the fake comfy for tests)
internal/catalog/, graph/, herr/, settings/, platform/    base: manifests, graph conversion, error codes, settings, processes and OS facts
internal/archtest/                the test of the design rules
scripts/                          lint-public.py (a Python check before a pull request)
tests/                            mcp_contract.py (the contract test of the MCP server, fake ComfyUI, no GPU), workflow_parity.py, requirements.txt
tools/                            goal_check.py: the goal check of each phase
workflows/<name>/                 manifest.json, workflow.json, models.json, README.md, examples/
workflows/_template/README.md     template for a new workflow README
prompts/<family>/<name>.md        one saved prompt each, with its result
```

Before you open a pull request, run `python scripts/lint-public.py`. It must
exit 0. This repo is public. Also run `gofmt -l .` (it must print nothing) and
`go test ./...`. A change to `cmd/horus/`, `internal/` or a `manifest.json` also
needs `tests/mcp_contract.py` to end with all checks passed. The test tools need
Python 3.12 with `pip install -r tests/requirements.txt` ([how](docs/mcp.md#test-the-server)).

## Pull request titles and merges

The title of a pull request follows the Conventional Commits format:
`<type>(<scope>): <subject>`, for example `feat(backend): add the Windows
portable backend (#62)`. The types are `feat`, `fix`, `docs`, `refactor`,
`test`, `chore`, `ci`, `build` and `perf`. The scope is optional. Put `!` before
the colon when the change breaks a caller. The CI job `pr-title` checks the
title.

Merge a pull request with squash (`gh pr merge <n> --squash`). The title then
becomes the one commit on `main`. The release tool (release-please) reads those
commits: a `feat` raises the minor version, a `fix` raises the patch version,
and the other types make no release. Versions stay `0.x`. The commits inside a
branch can have any subject.
