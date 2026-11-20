# 0041. In the stack, the model tools read and change only the models folder of `HORUS_MODELS_DIR`, and `horus stack-mounts` mounts it into `horus-api`

- **Status:** Accepted
- **Date:** 2026-11-20
- **Deciders:** the agent of task #242, by the standing rules of epic #180

## Context

The spec: [`2026-11-19-job-queue.md`](../specs/2026-11-19-job-queue.md), the
sections "Settings" (`HORUS_MODELS_DIR`: the host folder of the model files
that ComfyUI reads, mounted into `horus-api`) and "MCP contract"
(`install_workflow` in the stack: "Only the model files, into
`HORUS_MODELS_DIR`"). [ADR 0029](0029-horus-runs-as-a-stack-of-three-containers.md):
the setup of ComfyUI stays on the host.

- Measured in the test stack `horus_test` on `main` (the pull request of
  task #242 has the output): `compose.yaml` and `horus stack-mounts` mount no
  models folder. `list_workflows` showed `not_installed` for a workflow whose
  model files were on the host. The first call of `install_workflow` answered
  `SETUP_CONFIRMATION_REQUIRED` with the step "install ComfyUI" in the home
  folder of the user of the container, and file steps into that folder.
- The setup service finds a model file in the models folder, in the
  `ComfyUI/models` folder of the install, and in each folder of
  `extra_model_paths.yaml`, and adds a new folder to that file. A container
  of the stack has no ComfyUI install and no `extra_model_paths.yaml`.
- Docker Compose refuses a bind mount whose source is empty, so
  `compose.yaml` cannot mount an optional folder of a setting.
  [ADR 0039](0039-generate-image-in-the-stack-uses-shared-folders-of-host-paths.md)
  puts the mounts of the settings into `compose.override.yaml`.
- An agent gives and gets the paths of its own machine
  (ADR 0039). A container sees a mounted folder at a path of Linux.
- The stack has no speech setup: `generate_speech` answers
  `BACKEND_UNAVAILABLE` there (ADR 0039).

## Decision

- `horus stack-mounts` reads `HORUS_MODELS_DIR` of the host. When it is set,
  `compose.override.yaml` mounts that folder into `horus-api` at `/models`,
  with the settings `HORUS_MODELS_DIR=/models` and `HORUS_MODELS_HOST_DIR`
  (the host folder). `horus-worker` gets no models folder.
- The setup service of an API process of the stack (`stackSetup` in
  `cmd/horus`) looks for a model file only in `HORUS_MODELS_DIR/<folder>`
  (`modelfiles.Store.OnlyModelsDir`), and sees ComfyUI as installed on the
  host. So a plan holds only model file steps. It never changes
  `extra_model_paths.yaml`.
- In the stack, `install_workflow` answers `INTERNAL` for a plan with a step
  that is not a model file, and runs nothing.
- In the stack without `HORUS_MODELS_DIR`, `install_workflow` and
  `uninstall_workflow` answer `BACKEND_NOT_INSTALLED`, action by human,
  with `details.setting` = `HORUS_MODELS_DIR` and a `next_step` that names
  the setting and `horus stack-mounts`.
- In the stack, `install_workflow` and `uninstall_workflow` of a speech
  workflow answer `BACKEND_UNAVAILABLE`, not retryable.
- `uninstall_workflow` in the stack keeps the rules of the direct path: the
  consent (`confirm`), the shared files (`remove_shared`) and the refusal of
  a file whose size differs from `models.json`. It deletes only files in the
  models folder.
- In the stack, the model tools show the host path of each model file and of
  each folder of a step.
- `LICENSE_CONFIRMATION_REQUIRED` also has `details.steps`: the steps of the
  plan, in the form of `SETUP_CONFIRMATION_REQUIRED`, on the direct path and
  in the stack.

## Consequences

- An agent in the stack sees the true install state of the model files of
  the host, and the consent of the user covers only model file downloads.
- The owner sets `HORUS_MODELS_DIR` to a folder that ComfyUI reads. horus in
  the stack cannot add the folder to `extra_model_paths.yaml`. A new file
  that ComfyUI does not list yet gives `BACKEND_RESTART_REQUIRED`.
- On Linux, the user of the containers (uid 10001) must be able to write into
  the models folder.
- A test stack mounts a new test folder as its models folder, and never the
  model files of the machine. The contract test refuses a models folder with
  a file that it did not write.
- To reverse this decision, remove the mount from `horus stack-mounts` and
  give the API process of the stack the setup service of the host. Then
  `install_workflow` in the stack plans a ComfyUI install again.
