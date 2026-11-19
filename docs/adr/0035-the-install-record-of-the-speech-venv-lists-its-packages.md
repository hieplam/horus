# 0035. Write the install record of the speech venv before the pip install, and list the packages after it

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the loop run of issue #188, by the standing rules of epic #181 and by ADR 0012 and ADR 0032

## Context

The setup plan of a speech workflow makes the speech venv in the speech folder
(ADR 0032). Python 3.12 makes the venv, then pip installs the pinned packages
`vieneu==3.8.4` and `faster-whisper==1.2.1`. The pip install downloads a lot,
so it can stop before its end: the network fails, or the user stops it.

- `horus uninstall` removes a folder only when it holds the install record
  `horus-install.json` ([ADR 0012](0012-uninstall-removes-only-what-horus-installed.md)).
  The uninstall of a speech workflow (issue #189) must remove the speech venv.
- The check of `generate_speech`, `list_workflows` and `horus doctor` finds the
  speech venv by the file of its Python. A pip install that stopped leaves that
  file, but not the packages.
- The venv of `horus install-comfy-mcp` gets its record before its pip install,
  and the record has no packages.

## Decision

- The install record of the speech venv has the kind `speech-venv`.
- horus writes the record right after `python -m venv`, before the pip install,
  with no packages.
- After the pip install ends, horus writes the record again, with the pinned
  packages in the field `packages`.
- The setup service counts the speech venv as ready when its Python is there,
  and its record lists the pinned packages or there is no record (a venv that
  horus did not make). A record with other packages gives the venv step of the
  plan again, and the check names the cause.
- The venv step refuses a folder with files and without the record: horus did
  not make it.

## Consequences

- `horus uninstall` can remove each speech venv that horus made, also one whose
  pip install stopped. It does not remove a venv that a person made.
- A change of the pinned packages makes each existing speech venv not ready,
  so the next `horus setup` runs the pip install again.
- The check of `generate_speech` reads only the file of the Python, not the
  record. A pip install that stopped can give `GENERATION_FAILED` there until
  the next install.
- To reverse this decision, write the record only after the pip install. Then
  the uninstall cannot remove a venv whose pip install stopped.
