# Architecture Decision Records

An Architecture Decision Record (ADR) is one short file for one decision. It
has three sections: Context, Decision and Consequences (the format of Michael
Nygard). This folder is the only place for a decision that later work must
follow. [ADR 0020](0020-one-place-for-each-record-of-feature-work.md) says
where the other records of feature work go.

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
- `python scripts/lint-public.py` checks the numbers, the parts of each record
  and the index.

## Index

| # | Decision | Status |
|---|---|---|
| [0001](0001-go-one-binary-per-platform.md) | Write horus in Go, one program file for each platform | Accepted |
| [0002](0002-port-first-then-restructure.md) | Port first with the same behaviour, then restructure | Accepted; the child process is superseded by 0006 |
| [0003](0003-stdio-transport-http-later.md) | Use the stdio transport. The HTTP transport comes later | Accepted |
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

## The old decision ids of issue #4

Until 2026-11-18 the decisions were rows D0 to D68 in the decision log of
[issue #4](https://github.com/hieplam/horus/issues/4). Old code comments,
issues and pull requests use those ids. This table gives the ADR that holds
each one.

| Old id | ADR | | Old id | ADR | | Old id | ADR |
|---|---|---|---|---|---|---|---|
| D0 | 0001 | | D23 | 0001 | | D46 | 0017 |
| D1 | 0001 | | D24 | 0010 | | D47 | 0007 |
| D2 | 0003 | | D25 | 0018 | | D48 | 0007 |
| D3 | 0003 | | D26 | 0006 | | D49 | 0007 |
| D4 | 0004 | | D27 | 0006 | | D50 | 0010 |
| D5 | 0007 | | D28 | 0006, 0005 | | D51 | 0010 |
| D6 | 0007 | | D29 | 0002 | | D52 | 0014 |
| D7 | 0005 | | D30 | 0015 | | D53 | 0009 |
| D8 | 0002 | | D31 | 0013 | | D54 | 0007 |
| D9 | 0016 | | D32 | 0016 | | D55 | 0003 |
| D10 | 0013 | | D33 | 0012 | | D56 | 0011 |
| D11 | 0002 | | D34 | 0015 | | D57 | 0011 |
| D12 | 0002 | | D35 | 0015 | | D58 | 0018 |
| D13 | 0001 | | D36 | 0013 | | D59 | 0014 |
| D14 | 0002 | | D37 | 0007, 0008 | | D60 | 0013 |
| D15 | 0001 | | D38 | 0010 | | D61 | 0015 |
| D16 | 0002 | | D39 | 0006 | | D62 | 0009, 0016 |
| D17 | 0007 | | D40 | 0018 | | D63 | 0011 |
| D18 | 0009 | | D41 | 0018 | | D64 | 0011 |
| D19 | 0002 | | D42 | 0007 | | D65 | 0012 |
| D20 | 0007 | | D43 | 0009 | | D66 | 0019 |
| D21 | 0002 | | D44 | 0008 | | D67 | 0018 |
| D22 | 0001 | | D45 | 0010 | | D68 | 0018 |
