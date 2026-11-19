# horus: rules for AI agents

You are an agent (Claude Code, Codex, or similar) in a clone of this repo. The
user describes an outcome. The user does not need to know ComfyUI. There are two
kinds of work:

- **Make an image.** Call the horus MCP server: `list_workflows`,
  `describe_workflow`, then `generate_image`. This is the only way to make an
  image, in a clone and outside it. Never run the graph of a workflow through
  the comfy MCP tools to make an image for the user.
- **Make speech.** Call the horus MCP server: `list_workflows`,
  `describe_workflow` of a workflow of the kind `speech`, then
  `generate_speech`. Compare the `transcript` of the result with your text.
- **Build or debug a workflow, or change horus.** Use the comfy MCP tools and
  the `horus` program of this repo.

## Read before you act

| Task | Read |
|---|---|
| Set up or repair a machine | [`docs/setup.md`](docs/setup.md) |
| `/mcp` shows `comfy-mcp` or `horus` as failed | Its program is not installed, or `horus` is not on the PATH. `.mcp.json` starts both servers through `horus` (`horus comfy-mcp` and `horus mcp`) on Windows and macOS. Run `horus doctor`, then the command that its FAIL line names (for `horus`: the install line of [step 0 of setup](docs/setup.md#step-0-install-the-horus-program), or `go install ./cmd/horus` in a clone, [how](docs/setup.md#build-horus)). Then ask the user to restart `claude`. |
| Install the horus program on a new machine | The install line of the [README](README.md#install-horus): `install.ps1` on Windows, `install.sh` on macOS. It checks the SHA-256 of the release asset. `horus version` prints the version. |
| Set up one workflow on a machine | `horus setup <workflow>` prints the plan and changes nothing. `horus setup <workflow> -yes` runs the plan: ComfyUI first, then the model files. A plan can start with a system package step (7-Zip with `winget`, Python 3.12 with `brew`): it runs only with `-with-packages`. The older names (`check-setup`, `install-comfyui`, `workflow install`) still work. Use `doctor` and `setup`. |
| Add horus to Claude Code or Codex | `horus register` (Claude Code) or `horus register -client codex`. |
| Remove what horus installed | `horus uninstall` prints the plan and changes nothing. `horus uninstall -yes` deletes the model files, and a ComfyUI folder or a venv only when horus installed it (the file `horus-install.json`). It keeps `.env` and `outputs/`. **Never run `-yes` on a machine with data that you must keep, and never with the real home folder in a test: use a sandbox.** |
| Stop the ComfyUI that horus started | `horus stop-comfyui`. It refuses a ComfyUI that a person started. |
| Make an image with an existing workflow | The horus MCP server: `list_workflows`, `describe_workflow`, `generate_image` ([`docs/mcp.md`](docs/mcp.md)). `workflows/<name>/README.md` shows how to write the subject. |
| Make speech with an existing speech workflow | The horus MCP server: `list_workflows`, `describe_workflow`, `generate_speech` ([`docs/mcp.md`](docs/mcp.md#generate_speech)). The result has the path of the audio file and its transcript. |
| A new style, a new model, a new workflow | [`docs/style-to-workflow.md`](docs/style-to-workflow.md) |
| `generate_image`, `generate_speech` or another horus tool fails | Read `error.code`, `error.action_by` and `error.next_step`. The codes: [`docs/mcp.md`](docs/mcp.md#error-codes) |
| A comfy MCP run fails while you build a workflow | [`docs/running-workflows.md`](docs/running-workflows.md) |
| Which model pairs with which | [`docs/models.md`](docs/models.md) |
| Why this model, and GPU traps | [`docs/why-flux3-klein.md`](docs/why-flux3-klein.md) |
| A term you must explain to the user | [`docs/concepts.md`](docs/concepts.md) |
| Change the job store, the broker or the worker (`internal/postgres`, `internal/brokertest`, `internal/worker`) | The spec [`docs/specs/2026-11-19-job-queue.md`](docs/specs/2026-11-19-job-queue.md), then [the worker](docs/mcp.md#the-worker) and [the Postgres tests](docs/mcp.md#the-postgres-tests) |
| Change the horus MCP server, its tools or its errors | [`docs/mcp.md`](docs/mcp.md), then run `go test ./...` and `tests/mcp_contract.py` |
| Change the speech worker (`internal/speechworker/worker.py`) | The docstring of `worker.py` gives the protocol. The Go tests use a fake worker, so also run `tests/speech/worker_real.py` with the real models: its docstring gives the command ([ADR 0022](docs/adr/0022-speech-runs-in-a-python-speech-worker.md)) |
| The flags of a `horus` command | `horus <command> -h` |
| Why horus does something in a specific way, or the history of a rule | The decision records: [`docs/adr/README.md`](docs/adr/README.md). Do not undo a decision without a new record. |
| Start feature work, or record a decision, a plan or a proof | [Feature work](#feature-work-one-place-for-each-record) below |
| Write or run the oracle of a goal row of a spec | [Goal suites](#goal-suites) below |
| The speech workflow `vi-speech`: its voices, text limits and tags | [`workflows/vi-speech/README.md`](workflows/vi-speech/README.md), then [`docs/specs/2026-11-20-text-to-speech.md`](docs/specs/2026-11-20-text-to-speech.md) |
| Offer a workflow to other agents | Add `workflows/<name>/manifest.json` ([fields](docs/mcp.md#add-a-workflow-to-the-server)) |

For a tech or IT illustration, use
[`workflows/it-illustration`](workflows/it-illustration/README.md). Its README
tells you how to write the caption.

**Two MCP servers.** The horus MCP server (`horus`, [`docs/mcp.md`](docs/mcp.md))
is the product. Every agent makes images and speech with it: it calls `list_workflows`,
`describe_workflow`, `generate_image` and `generate_speech`, and never sees ComfyUI or the speech worker. The comfy MCP
server (`comfy-mcp`) is the low-level tool for work on this repo only: build,
check and debug a workflow. A user's agent never gets it: `horus register` adds only `horus`. When you change a workflow, keep its `manifest.json` true, because the
horus server shows it to every caller.

## Rules

Rules 1 to 4 and 8 are for the comfy MCP tools.

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

1. **Simple.** Add no design pattern that hides no complexity.
2. **Deep modules.** A package hides its work behind a small interface.
3. **Pure functions first.** Code that does no input and no output gets no interface.
4. **Interfaces only at input and output.** The service declares the interface that it needs. A test gives it a fake.
5. **One direction.** A handler calls a service. A service calls an adapter through an interface. No call goes back.

`go test ./internal/archtest` enforces the direction, the OS boundary (OS-specific code lives only in `internal/platform` and `internal/backend`) and the token rule (only `internal/download` reads `HF_TOKEN`). The test has no allow list. Fix the code, not the test.

## Layout

```
README.md                         entry point for people: the install line, setup, uninstall
install.ps1, install.sh           the install scripts (the install line downloads them from main)
release-please-config.json        the release tool; .github/workflows/release.yml builds and uploads the 3 release assets
AGENTS.md                         this file
.claude/settings.json, .claude/hooks/    the Claude Code hooks: one refuses an edit of a frozen goal suite, one checks docs/adr after each Write or Edit
.mcp.json                         the two MCP servers of an agent in a clone: comfy-mcp and horus
ROADMAP.md                        what comes next
docs/                             setup, style-to-workflow, concepts, running, models, mcp
docs/adr/                         the decision records (ADRs): one file for one decision, README.md is the index
docs/specs/                       the spec of each piece of work: the ratified goal rows and the design
goals/<spec stem>/                the goal suite of one spec: one test function for each goal row (go test -tags goal)
goals/link_test.go                the link check: each suite has a spec, each goal row has a function
cmd/horus/                        wiring: main.go (the command table) and wiring.go (creates the adapters and gives them to the services)
internal/cli/                     handler: one file for each command group (flags in, text out)
internal/mcpserver/               handler: the 6 MCP tools, the envelope, the progress notifications
internal/worker/                  handler: the loop of horus worker: the slots, the gate of ComfyUI, the heartbeat, the worker lock, the sweep, the deletion of old jobs
internal/generate/                service: make one image
internal/speak/                   service: make one speech file (the checks of generate_speech, the speech worker of horus mcp)
internal/provision/               service: set up a machine (backend, model files, comfy-mcp and `horus comfy-mcp`, register, the plan of `horus setup`, the plan of `horus uninstall`)
internal/doctor/                  service: the checks of `horus doctor`
internal/devtools/                service: new-workflow and sync-workflows
internal/jobs/                    service: the job queue: the moves of a job, the recipe of the idempotency key, submit and wait, the delivery loop of the worker; it declares Broker and Store
internal/comfy/                   adapter: the ComfyUI REST client (comfy/comfytest is the fake ComfyUI for tests)
internal/download/                adapter: download with resume, size and SHA-256 check; the only code that reads HF_TOKEN
internal/modelfiles/              adapter: find, check and remove model files, extra_model_paths.yaml
internal/backend/                 adapter: Install, Start, Stop and Installed of a ComfyUI install (the Windows portable build; comfy-cli on macOS; backend/comfyclitest is the fake comfy for tests)
internal/postgres/                adapter: the job store and the Postgres broker (pgx); the schema horus in schema/*.sql, which Migrate applies; postgres/pgtest gives each test its own database
internal/speechworker/            adapter: start the Python speech worker (worker.py, go:embed), send a request, read the answer, stop it; speechworker/fakeworker is the fake worker for tests
internal/catalog/, graph/, herr/, jobdata/, settings/, platform/, version/    base: manifests, graph conversion, error codes, the data of a job and the readiness report, settings, processes and OS facts, the program version
internal/speechsetup/             base: where the speech setup is (the speech folder: venv/, models/, worker.py) and which parts a speech workflow needs
internal/archtest/                the test of the design rules
internal/brokertest/              the contract suite of a broker and the store contract of a job store (each one passes its suite), the in-memory broker and the in-memory job store of the tests
scripts/                          lint-public.py (a Python check before a pull request: private data, dead links, the ADR folder), frozen-suites.py (refuses a change to a frozen goal suite), build-release.sh (builds the 3 release assets)
tests/                            mcp_contract.py (the contract test of the MCP server, fake ComfyUI, no GPU), peak_load.py (the peak tool: many agents at once), workflow_parity.py, requirements.txt
tests/speech/                     the tests with the real speech models (not in CI): baseline.py, worker_real.py (it runs only with HORUS_SPEECH_REAL=1)
workflows/embed.go                the go:embed line: the program holds each workflow folder (add a new folder there)
workflows/<name>/                 an image workflow: manifest.json, workflow.json, models.json, README.md, examples/
workflows/vi-speech/              the speech workflow (kind speech): manifest.json, models.json, README.md. No graph and no examples
workflows/_template/README.md     template for a new workflow README
prompts/<family>/<name>.md        one saved prompt each, with its result
```

Before you open a pull request, run `python scripts/lint-public.py`. It must
exit 0. This repo is public. Also run `gofmt -l .` (it must print nothing) and
`go test ./...`. A change to `cmd/horus/`, `internal/` or a `manifest.json` also
needs `tests/mcp_contract.py` to end with all checks passed. A change to
`internal/postgres`, `internal/brokertest`, `internal/jobs` or `internal/worker` also needs
`go test ./internal/postgres/... ./internal/brokertest/... ./cmd/horus` with
`HORUS_TEST_DATABASE_URL` set ([how](docs/mcp.md#the-postgres-tests)). The test tools need
Python 3.12 with `pip install -r tests/requirements.txt` ([how](docs/mcp.md#test-the-server)).

## Feature work: one place for each record

| Record | Its one place |
|---|---|
| A decision that later work must follow | An Architecture Decision Record (ADR): one short file in `docs/adr/`, added in the pull request of the work that it decides ([rules](docs/adr/README.md#rules)) |
| The goal of a phase: the terms, the ratified goal rows, the design, the standing rules | A spec, `docs/specs/<date>-<topic>.md`, merged before the issues. It has no task list and no decision table: it links the ADRs |
| The oracle of each goal row | A goal suite, `goals/<spec stem>/`, added in the pull request of the spec ([rules](#goal-suites)) |
| The plan: the tasks, their order and the Verify block of each task | GitHub issues: the epic and one issue for each task |
| The proof of a task: Red, Green and Stub check with the literal output | The `## Verify` section of the pull request that closes the task |
| A decision of one task (a design gap, a standing rule) | A `## Decisions` section in the body of that pull request |

- **Write no plan file and no evidence log.** With the epic-loop skill, write
  each task body to a file in a scratch folder and give it to `epic.mjs create`
  as `"body"`. The manifest has no `plan` and no `evidence` entry. Run
  `epic.mjs check <task>` without `--evidence`.
- **A task that changes behaviour also changes the docs for that behaviour**, in
  the same pull request. Make a docs-only task only for a document with no code
  change, for example a page in `docs/`.
- **Work that fits in one pull request needs no spec and no epic.** Its pull
  request still has a `## Verify` section. When the owner ratifies goal rows
  for such work, rule 8 of the goal suites applies.

## Goal suites

A goal suite holds the oracle of each goal row of one spec. `go test` runs it.
[`goals/2026-11-18-goal-suites`](goals/2026-11-18-goal-suites/suite_test.go)
is an example.

1. **A spec with a `## Ratified goal` table has a suite**, the folder
   `goals/<spec stem>/`, added in the pull request of the spec. The spec stem
   is the file name of the spec without `.md`.
2. **One test function for each row.** The row `same-behaviour` is
   `TestSameBehaviour`. Each file starts with `//go:build goal`.
3. **A row that is not built yet fails**: `t.Fatal("not built: <what is missing>")`.
   A part that needs the GPU machine calls `t.Skip` and says what it needs.
4. **A suite tests horus from the outside.** It imports only the Go standard
   library. It starts programs (`horus`, `go test -run`, `gh`, the Python
   tests) and reads files.
5. **Run only the suite of your spec**:
   `go test -tags goal -v ./goals/<spec stem>`. Put the output in the
   `## Verify` section, with the count of `--- PASS` lines before and after.
   The count can only rise.
6. **The last pull request of the work** shows each row as PASS and adds the
   block `> **Record of a finished phase.**` to the spec, as its last change.
   The suite is then frozen.
7. **Never edit a frozen suite.** A change to a finished feature gets a new
   spec and a new suite.
8. **Work that fits in one pull request** gets a suite only when the owner
   ratifies goal rows for it. Then the pull request also adds a short spec
   with the goal table.

`go test ./goals` checks rules 1, 2 and 4 on each pull request, and CI compiles
each suite with `go vet -tags goal ./goals/...`. For rule 7, a hook refuses a
Write or an Edit of a file of a frozen suite, and the CI step
`scripts/frozen-suites.py --base` fails a pull request that changes one.

## Pages that agents read

These rules are for `AGENTS.md`, each README and each page in `docs/` that
tells an agent what to do.

1. **Give only the rule of today.** Write the page as if the rule was always
   there. Do not write what the rule was before, when it changed, or which
   record made it. An agent follows what a page says, so a line about the old
   way can send it to the old way. The history of a rule goes in its ADR.
2. **Put a check in a hook, not in a sentence.** If an agent must run a command
   after it changes a file, add a hook in `.claude/settings.json` that runs the
   command. Give the script a flag for that one part, as
   `scripts/lint-public.py --adr` does. CI stays the gate, because a hook does
   not run for a file that a shell command writes, and it does not run in each
   agent program.

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
