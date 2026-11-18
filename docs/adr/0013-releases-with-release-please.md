# 0013. Release with release-please on GitHub Releases, versions `0.x`, three plain files

- **Status:** Accepted
- **Date:** 2026-11-15
- **Deciders:** the owner (release-please and `0.x`), the planning session of Phase 4 (the assets)
- **Old ids in issue #4:** D10, D31, D36, D60

## Context

release-please is the release tool of Google. It reads the commits on `main`,
keeps one pull request open with the next version and the changelog, and makes
a GitHub Release when that pull request is merged. It needs commit subjects in
the Conventional Commits format.

## Decision

- Releases use release-please on GitHub Releases. A Homebrew tap and a Scoop
  bucket can come later.
- Versions stay `0.x` (`bump-minor-pre-major` and
  `bump-patch-for-minor-pre-major`). No version 1 until the owner says so.
- The title of a pull request follows Conventional Commits. The CI job
  `pr-title` checks it. A merge uses squash, so the title is the one commit on
  `main`.
- A release has 3 plain files: `horus-windows-amd64.exe`, `horus-darwin-arm64`
  and `checksums.txt`. No archive and no GoReleaser.
- One variable, `internal/version.Version`, holds the version. The release
  build sets it with the linker flag `-X`. `horus version` and the
  `serverInfo` of `horus mcp` print it.
- The CI job `release-build` runs the same build script on each pull request,
  without the upload.

## Consequences

- A `feat` commit raises the minor version, a `fix` commit raises the patch
  version, and the other types make no release.
- A wrong pull request title fails CI.
- A broken release build fails a pull request, not a release.
- To reverse this decision, delete `release-please-config.json` and
  `.github/workflows/release.yml`, and make each release by hand.
