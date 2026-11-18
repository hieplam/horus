# 0015. One install line for each system, and the README shows it first

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner (the goal), the planning session of Phase 4 (the folders)
- **Old ids in issue #4:** D30, D34, D35, D61

## Context

Before the first release, a person installed Go, ran `go install` and edited
the PATH. The end goal of the owner: on a new Windows machine or a new Mac,
one line installs the program, and `horus setup <workflow>` or an agent does
the other steps. Windows has no `sh`.

## Decision

- `install.sh` (macOS, through `curl` and `sh`) and `install.ps1` (Windows,
  through `irm` and `iex`) are at the repo root. `install.ps1` is permitted,
  because horus itself still needs no PowerShell to run.
- Each script downloads the program of the platform from the newest GitHub
  Release and checks its SHA-256 against `checksums.txt` before it makes a
  folder. A wrong SHA-256 installs nothing.
- `install.ps1` puts the program in `%LOCALAPPDATA%\Programs\horus` and adds
  that folder to the PATH of the user (`HKCU\Environment`).
- `install.sh` puts the program in `~/.local/bin` and adds one line with the
  marker `# added by the horus installer` to `~/.zprofile`.
- `README.md` shows the install line and the manual setup steps in its first
  or second section.

## Consequences

- A person runs 1 command to get `horus` on the PATH.
- The install line downloads the script from `main`, so a bad script on
  `main` breaks each new install. The CI jobs `install windows-latest` and
  `install macos-latest` test both scripts on each pull request.
- `horus uninstall` knows the marker and the PATH entry, so a change of them
  needs a change of the uninstall
  ([0012](0012-uninstall-removes-only-what-horus-installed.md)).
- To reverse this decision, delete the two scripts. A person then builds the
  program with Go.
