# {{NAME}}

{{DESCRIPTION}}

Model family: **{{MODEL_FAMILY}}**

## Offer it to other agents

The horus MCP server offers this workflow only when the folder has
`manifest.json`. Copy `workflows/it-illustration/manifest.json` and change
every field: summary, theme, `good_for`, `not_good_for`, sizes, subject rules,
timing and `graph.inputs`. See [`docs/mcp.md`](../../docs/mcp.md#add-a-workflow-to-the-server).
Then run `tests/mcp_contract.py`.

## Purpose

State in one or two sentences what image this workflow makes and for whom.
Name the output type, the style, and the default size.

## Models it loads

List every model file the graph loads. Record the pinned file in
`models.json` and in the [model catalog](../../docs/models.md).

| Role | File | Folder under `models/` | Bytes | SHA-256 |
|---|---|---|---|---|
| Diffusion model or checkpoint | TBD | TBD | TBD | TBD |
| Text encoder | TBD | TBD | TBD | TBD |
| VAE | TBD | TBD | TBD | TBD |

## Couplings (what breaks)

A coupling is a setting that must match another setting. Write each one with
the failure that you see when it is wrong.

| Coupling | Must be | If wrong |
|---|---|---|
| Text encoder type in the loader node | TBD | TBD |
| VAE file | TBD | TBD |
| Sampler, steps, and CFG for this family | TBD | TBD |

## Slots

A slot is a setting that you can change with `set_workflow_slot`. Get the
list with `list_workflow_slots`. Node IDs differ for each workflow.

| Slot address | Meaning | Default | Safe range |
|---|---|---|---|
| TBD | Prompt text | TBD | TBD |
| TBD | Seed | TBD | any integer |
| TBD | Width and height | TBD | TBD |
| TBD | Steps | TBD | TBD |

## Run through the MCP tools

1. `server_info`
2. `list_workflow_slots` on a copy of `workflow.json`
3. `set_workflow_slot` on the copy (use `stdout=False` to write the copy)
4. `validate_workflow` on the copy
5. `run_workflow` with `wait=False`
6. `job` with `action="wait"`
7. `fetch_outputs`

See [running workflows](../../docs/running-workflows.md).

## Run through a script

Write the command that makes one image without an agent, or write "none".

```powershell
# TBD
```

## Evidence

A workflow is not done until it has run end to end. Fill this table after the
first successful run. Do not leave a value as TBD in a merged pull request.

| Item | Value |
|---|---|
| Date | TBD |
| ComfyUI version | TBD |
| `validate_workflow` result (`valid`) | TBD |
| `converted_node_count` and graph node count | TBD |
| `prompt_id` | TBD |
| Run time in seconds | TBD |
| Output file | TBD |
| Saved prompt | TBD |
