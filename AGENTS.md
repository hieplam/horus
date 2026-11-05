# horus: rules for AI agents

You are an agent (Claude Code, Codex, or similar) that drives a local ComfyUI
for a user. The user describes an outcome. You do the work through the comfy
MCP tools and the scripts in this repo. The user does not need to know ComfyUI.

## Read before you act

| Task | Read |
|---|---|
| Set up or repair a machine | [`docs/setup.md`](docs/setup.md) |
| Make an image with an existing workflow | `workflows/<name>/README.md` |
| A new style, a new model, a new workflow | [`docs/style-to-workflow.md`](docs/style-to-workflow.md) |
| A run fails | [`docs/running-workflows.md`](docs/running-workflows.md) |
| Which model pairs with which | [`docs/models.md`](docs/models.md) |
| A term you must explain to the user | [`docs/concepts.md`](docs/concepts.md) |

For a tech or IT illustration, use
[`workflows/it-illustration`](workflows/it-illustration/README.md). Its README
tells you how to write the caption.

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

## Explaining to the user

Name the item, say what it does, and say what breaks if it is wrong. Good:
"The VAE must be `flux2-vae` because FLUX 3 uses the FLUX.2 latent space. A
different VAE decodes to coloured noise." Bad: "Configure the VAE."

## Layout

```
README.md                         entry point for people
AGENTS.md                         this file
ROADMAP.md                        what comes next (horus as an MCP server)
docs/                             setup, style-to-workflow, concepts, running, models
scripts/                          PowerShell setup scripts, run-illustration.py, lint-public.py
workflows/<name>/                 workflow.json, models.json, README.md, examples/
workflows/_template/README.md     template for a new workflow README
prompts/<family>/<name>.md        one saved prompt each, with its result
```

Before you open a pull request, run `python scripts/lint-public.py`. It must
exit 0. This repo is public.
