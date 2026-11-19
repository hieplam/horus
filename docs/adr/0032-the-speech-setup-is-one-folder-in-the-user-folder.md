# 0032. The speech setup is one folder in the user folder

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the loop run of issue #187, by the standing rules of epic #181 and by ADR 0009 and ADR 0022

## Context

A speech workflow needs three things on the machine that an image workflow
does not need ([ADR 0022](0022-speech-runs-in-a-python-speech-worker.md)):

- the speech venv, whose Python runs the speech worker;
- the model files of the speech model and of the speech recognition model;
- the file of the speech worker, which the `horus` program writes before each start.

The spec ([`2026-11-20-text-to-speech.md`](../specs/2026-11-20-text-to-speech.md))
says that horus makes the venv and writes the worker file to the user folder.
It does not give the folders. The tool `generate_speech` (issue #187) must find
them, and the install (issue #188) and the uninstall (issue #189) must use the
same folders.

- [ADR 0009](0009-folders-install-user-and-repo-root.md): the user folder is
  `HORUS_HOME`, else `<home>/horus`, on both systems. It is outside the TCC
  folders of macOS.
- The model files of an image workflow go to the models folder of ComfyUI. A
  speech workflow needs no ComfyUI, so that folder can be absent.
- The spec row `speech-install` needs 0 files of the speech setup in the
  sandbox after the uninstall.

## Decision

- The speech setup is one folder, the speech folder: `HORUS_SPEECH_DIR`, else
  `<user folder>/speech`.
- The speech venv is `<speech folder>/venv`. horus writes its install record
  (`horus-install.json`) there.
- The model files of each speech workflow are in `<speech folder>/models`,
  each file in the `folder` of its `models.json` entry, with the last part of
  its `repo_path` as the file name. The speech worker gets `--models <speech folder>/models`.
- The file of the speech worker is `<speech folder>/worker.py`.
- A speech workflow also needs `ffmpeg` and `ffprobe` on the PATH.
- The package `internal/speechsetup` is the one place of these rules: `Here`
  gives the folders, `Parts` lists what a workflow needs, and `Missing` gives
  what is not there. The speak service, the setup plan and `horus doctor` use it.

## Consequences

- A user has one more folder to find, inside the user folder.
- A test sets `HORUS_SPEECH_DIR` or `HORUS_HOME` to a sandbox. If it does not,
  it reads the speech setup of the real home folder.
- The uninstall removes the speech folder parts: the venv because it has the
  install record, the model files of `models.json`, and `worker.py`.
- To reverse this decision, change `speechsetup.Here` and `speechsetup.At`.
  Existing installs then need a move of the venv and the model files.
