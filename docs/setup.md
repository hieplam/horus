# Flow 1: set up a fresh Windows PC

This page takes a Windows PC with an NVIDIA GPU to your first AI image. An AI agent
(Claude Code or Codex) makes the image. The agent talks to ComfyUI through the
comfy MCP server. Every step is a script that is safe to run again.

Run all commands in PowerShell, from the root folder of this repo. Run this one time
in each PowerShell window, so that the scripts can run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
```

## What is each part

| Part | What it is | Why you need it |
|---|---|---|
| ComfyUI | A program that makes images from a graph of steps (a "workflow"). It has a browser page and an HTTP server at `http://127.0.0.1:8188`. | It does the real work on your GPU. |
| Model files | Large files (`.safetensors`) that hold the learned weights. FLUX 3 Image Klein has three: a diffusion model, a text encoder and a VAE. | ComfyUI cannot make an image without them. They are 11 GB in total. |
| comfy-cli | A command-line tool from the ComfyUI team. | The MCP server calls it. |
| comfy MCP server (`comfy-mcp`) | A small program that speaks the Model Context Protocol (MCP). MCP is the standard way for an agent to call tools. | It gives the agent safe tools: `server_info`, `validate_workflow`, `run_workflow` and more. |
| Agent | Claude Code or Codex. | You describe the image. The agent calls the tools. |

**Why the agent never edits ComfyUI directly.** The agent can break a working install
if it changes files in ComfyUI or in the workflow JSON by hand. The MCP tools check
a workflow before they run it, and they change one named setting at a time. So the
agent uses only the tools. The repo keeps the master copy of each workflow.

## Prerequisites

Check each item before step 1.

| Item | What you need | How to check |
|---|---|---|
| GPU | NVIDIA, 6 GB VRAM or more. The workflow was tested on a 6 GB card. | `nvidia-smi` |
| NVIDIA driver | Version 528.33 or newer. | `nvidia-smi` shows the version. |
| RAM | 16 GB. Close large programs before you make images. | Task Manager |
| Disk | About 9 GB for ComfyUI and 11 GB for models. Keep 1 GiB spare on each drive that gets a download. | File Explorer |
| Git | Any recent version. | `git --version` |
| Python 3.12 with the Python Launcher | Runs the MCP server. | `py -3.12 --version` |
| 7-Zip | Unpacks ComfyUI. | `winget install 7zip.7zip` |
| Microsoft Visual C++ x64 Redistributable | PyTorch needs it. Without it, ComfyUI fails with `WinError 1114` and `vcruntime140_1.dll`. | [Download from Microsoft](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist) |
| Hugging Face account | Free. You need it to accept the model license. | [huggingface.co](https://huggingface.co) |

`curl.exe` is part of Windows 10 and 11. You do not install it.

**Choose the drives first.** ComfyUI needs about 9 GB. The models need 11 GB. They can
live on different drives. You set the paths in step 1 and step 4. You can also write
them one time in `.env` (see step 3): `HORUS_COMFYUI_DIR`, `HORUS_MODELS_DIR` and
`HORUS_TEXT_ENCODER_DIR`. If you set nothing, ComfyUI goes to
`%USERPROFILE%\ComfyUI_windows_portable`.

## Step 1: install ComfyUI

```powershell
.\scripts\install-comfyui.ps1
.\scripts\start-comfyui.ps1
```

To use another drive, add `-InstallDir 'E:\AI\ComfyUI_windows_portable'` to the first
command and `-ComfyUIDir 'E:\AI\ComfyUI_windows_portable'` to the second.

**What it makes.** The first script downloads the official ComfyUI portable build
(2 GB), checks its SHA-256 hash, and unpacks it. The portable build has its own Python.
The second script starts the server and waits until `http://127.0.0.1:8188` answers.
The first start can take a few minutes. Open that address in a browser to see ComfyUI.

**If it fails.**

- `7-Zip was not found`: install 7-Zip, or pass `-SevenZip 'C:\path\to\7z.exe'`.
- `Not enough free space`: choose a drive with more space.
- `SHA-256 mismatch`: the script deleted the bad file. Run the command again.
- `already installed`: this is not an error. The script skips the install.
- ComfyUI stops at start with `WinError 1114`: install the Visual C++ Redistributable, then run `start-comfyui.ps1` again.
- Other start errors: read `horus-comfyui.err.log` in the ComfyUI folder.

## Step 2: install the two MCP servers

```powershell
.\scripts\install-comfy-mcp.ps1
.\scripts\install-horus-mcp.ps1
```

**What it makes.** Two Python virtual environments (venvs). Claude Code starts each
server program itself when a session starts. You do not start them.

| venv | Holds | Program |
|---|---|---|
| `%USERPROFILE%\.venvs\comfy-mcp` | `comfy-mcp` 0.10.0 and `comfy-cli` 1.25.0 | `comfy-mcp.exe` |
| `%USERPROFILE%\.venvs\horus-mcp` | this repo (editable) and `mcp` 2.3.0 | `horus-mcp.exe` |

The versions are fixed on purpose, so that every PC behaves the same. Keep the default
paths. The file `.mcp.json` in this repo points at both programs. If a program is
missing, Claude Code shows that server as failed in `/mcp`.

"Editable" means that the horus venv runs the code in this folder. So `git pull`
updates the horus server without a new install.

**If it fails.**

- `Python 3.12 was not found`: install Python 3.12 from python.org. Select "py launcher" in the installer.
- `pip install failed`: check your network, then run the command again.

## Step 3: put your Hugging Face token in `.env`

```powershell
Copy-Item .env.example .env
notepad .env
```

**What it makes.** A file `.env` that Git ignores. Follow the four steps written in
the file:

1. Sign in at huggingface.co.
2. Open `https://huggingface.co/black-forest-labs/flux-3-image-klein-comfy` and accept the FLUX Kommunity License.
3. Create a token of type "Read" at `https://huggingface.co/settings/tokens`.
4. Paste the token after `HF_TOKEN=`.

Also set the `HORUS_*` paths in this file if you chose other drives.

**If it fails later (in step 4).** The download scripts show the HTTP code.

| Code | Meaning | Fix |
|---|---|---|
| 401 | The token is missing, wrong or expired. | Create a new Read token and put it in `.env`. |
| 403 | The token is valid, but its account did not accept the license. | Accept the license on the model page with the same account that made the token. |

## Step 4: download the models, sync the workflow, run a smoke test

```powershell
.\scripts\workflow.ps1 install it-illustration
.\scripts\sync-workflows.ps1
.\scripts\check-setup.ps1 -Workflow it-illustration
```

If the text encoder must go to another drive, add `-TextEncoderDir 'E:\AI\models\text_encoders'`
to the first command. Add `-ModelsDir` to move all model folders.

**What it makes.**

- The first script reads `workflows\it-illustration\models.json`. It downloads each model file to a `.part` file, checks the size and SHA-256 hash, and renames it only when the hash matches. It skips a file that is already correct. It never overwrites a file that differs.
- When a model folder is outside ComfyUI, the script adds it to `extra_model_paths.yaml`. It makes a backup first. **Restart ComfyUI after this change:** run `start-comfyui.ps1` after you stop the old server.
- The second script copies the workflow into ComfyUI as `horus-it-illustration.json`. You see it in the Workflows pane of the ComfyUI page.
- The third script is the smoke test (a quick check that everything is in place). It prints one `PASS` or `FAIL` line for each check. It exits with code 0 only if all checks pass. It needs a few minutes, because it hashes 11 GB.

To check only, without a download, run `.\scripts\workflow.ps1 check it-illustration`.
The old command `install-workflow-models.ps1 -Workflow it-illustration` still works. It calls `workflow.ps1`.
See [manage workflows](#manage-workflows-list-install-check-remove) for the other actions.

**If it fails.**

- `Not enough free space`: the script needs the file size plus 1 GiB. Free space, or use `-TextEncoderDir` or `-ModelsDir` to choose another drive.
- `curl exit 22` or `HTTP 401` or `HTTP 403`: see the table in step 3.
- `Download did not finish`: run the same command again. It resumes the `.part` file.
- `SHA-256 mismatch`: the script deleted the bad download. Run the command again.
- `Refusing to overwrite`: a file with that name exists and is different. Move it or delete it yourself, then run again.
- The model is missing in the ComfyUI list: restart ComfyUI.

## Step 5: connect the agent

Start the agent in this repo folder:

```powershell
claude
```

Claude Code reads `.mcp.json` and asks whether to use the project servers `comfy-mcp`
and `horus`. Approve both. Then:

1. Type `/mcp`. It must show `comfy-mcp` and `horus` as connected.
2. Type: `run server_info`.

**What it makes.** A live link between the agent and your ComfyUI. `server_info` must
report that ComfyUI is running at `http://127.0.0.1:8188`.

**If it fails.**

- `comfy-mcp` or `horus` failed (`CONNECTION_CLOSED`): the program in `.mcp.json` does not exist. Run step 2 with the default paths, then restart `claude`. `scripts\check-setup.ps1` tells you whether the path matches.
- `Pending approval` in `claude mcp list`: approve the server in an interactive `claude` session. To be asked again, run `claude mcp reset-project-choices`.
- `UnicodeEncodeError: 'charmap' codec`: set `PYTHONUTF8=1`. The `.mcp.json` file already does this.
- `server_info` shows `running: false`: run `start-comfyui.ps1`.

For Codex, add the same server to its configuration. Use the paths of the venv from
step 2. Then restart Codex.

### Optional: the image tool for agents in other folders

```powershell
.\scripts\install-horus-mcp.ps1 -Register
```

**What it makes.** The same venv as step 2, and an entry `horus` in your Claude Code
user settings (user scope). Without `-Register`, only a session in this repo folder
sees `horus`, through `.mcp.json` (project scope). An agent in any folder then has the
tools `list_workflows`, `describe_workflow`, `install_workflow`, `uninstall_workflow`
and `generate_image`. Check it with `claude mcp get horus` from any folder: it shows
`Connected`. See [`mcp.md`](mcp.md).

**If it fails.** `claude was not found`: install Claude Code, or add the stdio server
`<venv>\Scripts\horus-mcp.exe` to your agent by hand. To remove the entry, run
`claude mcp remove horus -s user`.

## Step 6: make your first image

Read [`workflows/it-illustration/README.md`](../workflows/it-illustration/README.md). It
explains how to write the picture description for the agent. Then ask the agent, for
example: "Use it-illustration to draw a Kubernetes cluster with three nodes."

Without an agent, you can run the same workflow from the command line with
[`scripts/run-illustration.py`](../scripts/run-illustration.py). Its `--help` shows
the options.

**If it fails.** The README of the workflow lists the errors of the workflow itself.
The first image can take several minutes, because ComfyUI loads 11 GB of models.

## Check an existing machine at any time

```powershell
.\scripts\check-setup.ps1                              # server, both venvs and .mcp.json
.\scripts\check-setup.ps1 -Workflow it-illustration    # also the model hashes
```

## Manage workflows: list, install, check, remove

`scripts\workflow.ps1` manages the files that each workflow needs. A workflow
is a folder under `workflows\`. Its `models.json` lists every model file (diffusion
model, text encoder, VAE, LoRA and so on) with its size and SHA-256.

```powershell
.\scripts\workflow.ps1 list                                # every workflow, its description, installed or not
.\scripts\workflow.ps1 install it-illustration             # download and verify, then check
.\scripts\workflow.ps1 check it-illustration               # is this PC ready to run it?
.\scripts\workflow.ps1 remove it-illustration              # prints what it deletes, deletes nothing
.\scripts\workflow.ps1 remove it-illustration -Yes         # deletes the files
.\scripts\workflow.ps1 remove it-illustration -Only vae -Yes  # deletes one file (a role or a file name)
```

| Action | What it does | What it never does |
|---|---|---|
| `list` | Prints each workflow with the first paragraph of its README, the number and size of its model files, and if they are on disk (size check). `-Json` prints JSON. | Download or delete. |
| `install` | Downloads each missing file, checks the SHA-256, then runs `check`. A file that is present and correct is skipped. | Overwrite a file that differs. |
| `check` | Checks size and SHA-256 of each file (`-Quick`: size only). Checks that `models.json` lists every model file that `workflow.json` loads. When ComfyUI runs, checks that the server has every node class and lists every model file. | Change a file. |
| `remove` | Without `-Yes`, prints the plan. With `-Yes`, deletes the files and unfinished `.part` downloads. | Delete a file that another workflow lists (unless `-Force`), or a file whose size differs from `models.json`. |

The script finds a model file in the `-ModelsDir` folder, in ComfyUI's own
`models` folder, and in every folder that `extra_model_paths.yaml` maps. So a
text encoder on another drive is found when ComfyUI can find it.

`check` does not install a missing custom node pack. It names the missing node
class. Install the pack with the MCP tool `install_node`, then restart ComfyUI.

A file from another source (for example a LoRA from another Hugging Face
repository) gets its own `repo` and `revision`, or a full `url`, in its
`models.json` entry. The token goes only to `huggingface.co` hosts.

