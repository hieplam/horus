# Architecture Decision Records

An Architecture Decision Record (ADR) is one short file for one decision. It
has three sections: Context, Decision and Consequences. This folder is the
only place for a decision that later work must follow. Do not undo a decision
without a new record.

## Rules

- Write an ADR when a decision changes what later work must do: a rule of the
  design, a default, a consent, a release rule, a rule of the owner.
- Do not write an ADR for a decision of one task. Put it in the `## Decisions`
  section of the pull request.
- Add the ADR in the pull request of the work that it decides.
- Copy [`0000-template.md`](0000-template.md). Take the next free number. If
  `main` got that number before your merge, take the next one.
- Never change the Decision section of an accepted ADR. To change a decision,
  add a new ADR and set the status of the old one to `Superseded by NNNN`.
- Add one row to the index below.

## The check

`python scripts/lint-public.py --adr` checks that each number is used one
time, that each record has its five parts, and that the index links each
record. You do not need to remember it:

- In Claude Code, a hook runs it after each Write or Edit of a file in this
  folder (`.claude/settings.json`). A finding comes back to the agent at once.
- CI runs it on each pull request, as a part of `scripts/lint-public.py`.

## Index

| # | Decision | Status |
|---|---|---|
| [0001](0001-go-one-binary-per-platform.md) | Write horus in Go, one program file for each platform | Accepted; the Python goal tool is superseded by 0021 |
| [0002](0002-port-first-then-restructure.md) | Port first with the same behaviour, then restructure | Accepted; the child process is superseded by 0006 |
| [0003](0003-stdio-transport-http-later.md) | Use the stdio transport. The HTTP transport comes later | Accepted; "no daemon" is superseded by 0025, "the only transport" by 0030 |
| [0004](0004-direct-rest-and-separate-processes.md) | Call the ComfyUI REST API directly. Run comfy-cli and comfy-mcp only as separate processes | Accepted; the image path is Proposed |
| [0005](0005-only-download-reads-the-token.md) | Only the download code reads `HF_TOKEN` | Accepted |
| [0006](0006-design-rules-and-three-layers.md) | Five design rules, three layers, and a test with no allow list | Accepted |
| [0007](0007-one-backend-interface-two-installs.md) | One Backend interface, the portable build on Windows and comfy-cli on macOS | Accepted |
| [0008](0008-never-stop-a-comfyui-that-a-person-started.md) | Never stop a ComfyUI that a person started | Accepted |
| [0009](0009-folders-install-user-and-repo-root.md) | The install folder, the user folder and the repo root | Accepted; the rule about a clone in `~/horus` is Proposed |
| [0010](0010-setup-prints-a-plan-doctor-checks.md) | `horus setup` prints a plan and acts only with `-yes`. `horus doctor` checks | Accepted |
| [0011](0011-setup-through-mcp-needs-two-consents.md) | `install_workflow` runs the whole setup plan, after a setup consent and a license consent | Accepted |
| [0012](0012-uninstall-removes-only-what-horus-installed.md) | `horus uninstall` removes only what horus installed | Accepted |
| [0013](0013-releases-with-release-please.md) | Release with release-please on GitHub Releases, versions `0.x`, three plain files | Accepted |
| [0014](0014-who-merges-and-who-publishes.md) | An agent merges only a pull request of the owner. The owner publishes each release | Accepted |
| [0015](0015-one-install-line.md) | One install line for each system, and the README shows it first | Accepted |
| [0016](0016-embed-the-built-in-workflows.md) | The program holds the built-in workflows, and a folder can replace them | Accepted |
| [0017](0017-start-comfy-mcp-through-horus.md) | `.mcp.json` starts the comfy MCP server through `horus comfy-mcp` | Accepted |
| [0018](0018-real-runs-on-the-owners-machines.md) | How a real run uses the owner's machines and the CI runners | Accepted |
| [0019](0019-register-for-claude-code-and-codex.md) | `horus register` serves Claude Code and Codex | Accepted |
| [0020](0020-one-place-for-each-record-of-feature-work.md) | One place for each record of feature work: ADR, spec, issue, pull request | Accepted |
| [0021](0021-goal-suites-are-go-test-packages.md) | A goal suite is a Go test package with the name of its spec, and it is frozen when the spec is finished | Accepted |
| [0022](0022-speech-runs-in-a-python-speech-worker.md) | Speech runs in a Python speech worker that `horus mcp` starts and keeps | Accepted |
| [0023](0023-a-workflow-has-a-kind-and-each-kind-has-its-tool.md) | A workflow has a kind, and each kind has its own tool on the one server | Accepted |
| [0024](0024-generate-speech-returns-a-transcript.md) | `generate_speech` returns a transcript, and the calling agent checks the audio | Accepted |
| [0025](0025-run-each-job-through-a-broker-and-one-worker.md) | Run each job through a broker and one worker | Accepted |
| [0026](0026-the-broker-is-an-interface-postgres-first.md) | The broker is an interface. Postgres is the first broker, and the job store stays in Postgres | Accepted |
| [0027](0027-the-idempotency-key-and-the-idempotent-consumer.md) | The idempotency key of a job, and the idempotent consumer | Accepted |
| [0028](0028-the-horus-ui-is-a-read-only-page-of-horus-api.md) | The Horus UI is a read-only page of `horus-api` on 127.0.0.1 | Accepted |
| [0029](0029-horus-runs-as-a-stack-of-three-containers.md) | horus runs as a Docker stack of 3 containers next to the ComfyUI of the host, and serves only when the 4 are up | Accepted |
| [0030](0030-horus-api-serves-mcp-over-stdio-and-http.md) | `horus-api` serves MCP over stdio and over HTTP | Accepted |
| [0031](0031-the-store-writes-each-event-and-the-service-sends-the-signal.md) | The job store writes each event row, and the service sends the signal after the move | Accepted |
| [0032](0032-the-speech-setup-is-one-folder-in-the-user-folder.md) | The speech setup is one folder in the user folder | Accepted |
| [0033](0033-each-job-store-passes-one-store-contract-with-the-clock-of-the-program.md) | Each job store passes one store contract, and takes its times from the clock of the program | Accepted |
| [0034](0034-the-payload-of-a-job-is-the-plan-of-generate-check.md) | The payload of a job is the plan of `generate.Check`, and the worker holds a delivery until ComfyUI answers | Accepted |
| [0035](0035-the-install-record-of-the-speech-venv-lists-its-packages.md) | Write the install record of the speech venv before the pip install, and list the packages after it | Accepted |
| [0036](0036-one-readiness-report-for-horus-ready-readyz-and-horus-mcp.md) | One readiness report for `horus ready`, `/readyz` and `horus mcp`, and a health check for each container | Accepted |
| [0037](0037-the-worker-checks-its-lock-and-takes-it-again.md) | The worker checks its worker lock at each heartbeat, and takes it again when its session ended | Accepted |
| [0038](0038-the-horus-ui-renders-on-the-server-and-checks-the-published-port.md) | The Horus UI renders on the server, reads the depth of the broker, and checks the Host with the published port | Accepted |
