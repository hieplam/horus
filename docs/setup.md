# Flow 1: set up a fresh Windows PC

This page takes a Windows PC with an NVIDIA GPU to your first AI image. An AI agent
(Claude Code or Codex) makes the image. The agent talks to ComfyUI through the
comfy MCP server. Every step is a command that is safe to run again.

The commands are subcommands of one program, `horus`. Step 0 builds it. Run all
commands in a terminal, from the root folder of this repo. Each command prints
its flags with `-h`, for example `horus workflow -h`.

**macOS and Linux.** `horus` builds and runs there too. Two commands are
Windows-only in this phase: `horus install-comfyui` and `horus start-comfyui`.
On macOS and Linux they exit with "not supported on this OS yet". There you
install and start ComfyUI by hand (see the ComfyUI README). A later phase adds
these two commands for macOS. The `comfy-mcp` entry of `.mcp.json` names a
Windows path, so on macOS and Linux you add that server to Claude Code by hand.
All other steps are the same.

## What is each part

| Part | What it is | Why you need it |
|---|---|---|
| ComfyUI | A program that makes images from a graph of steps (a "workflow"). It has a browser page and an HTTP server at `http://127.0.0.1:8188`. | It does the real work on your GPU. |
| Model files | Large files (`.safetensors`) that hold the learned weights. FLUX 3 Image Klein has three: a diffusion model, a text encoder and a VAE. | ComfyUI cannot make an image without them. They are 11 GB in total. |
| `horus` | One program, built from the Go code in this repo. It has the setup commands and the horus MCP server (`horus mcp`). | It installs and checks everything below, and it gives other agents the image tools. |
| comfy-cli | A command-line tool from the ComfyUI team. | The comfy MCP server calls it. |
| comfy MCP server (`comfy-mcp`) | A small program that speaks the Model Context Protocol (MCP). MCP is the standard way for an agent to call tools. | It gives the agent safe tools: `server_info`, `validate_workflow`, `run_workflow` and more. |
| Agent | Claude Code or Codex. | You describe the image. The agent calls the tools. |

**Why the agent never edits ComfyUI directly.** The agent can break a working install
if it changes files in ComfyUI or in the workflow JSON by hand. The MCP tools check
a workflow before they run it, and they change one named setting at a time. So the
agent uses only the tools. The repo keeps the master copy of each workflow.

## Prerequisites

Check each item before step 0.

| Item | What you need | How to check |
|---|---|---|
| GPU | NVIDIA, 6 GB VRAM or more. The workflow was tested on a 6 GB card. | `nvidia-smi` |
| NVIDIA driver | Version 528.33 or newer. | `nvidia-smi` shows the version. |
| RAM | 16 GB. Close large programs before you make images. | Task Manager |
| Disk | About 9 GB for ComfyUI and 11 GB for models. Keep 1 GiB spare on each drive that gets a download. | File Explorer |
| Git | Any recent version. | `git --version` |
| Go 1.27.3 or newer | Builds the `horus` program. Install it from [go.dev/dl](https://go.dev/dl/). | `go version` |
| Python 3.12 with the Python Launcher | The comfy MCP server is a Python program. `horus` itself needs no Python. | `py -3.12 --version` |
| 7-Zip | Unpacks ComfyUI. | `winget install 7zip.7zip` |
| Microsoft Visual C++ x64 Redistributable | PyTorch needs it. Without it, ComfyUI fails with `WinError 1114` and `vcruntime140_1.dll`. | [Download from Microsoft](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist) |
| Hugging Face account | Free. You need it to accept the model license. | [huggingface.co](https://huggingface.co) |

**Choose the drives first.** ComfyUI needs about 9 GB. The models need 11 GB. They can
live on different drives. You set the paths in step 1 and step 4. You can also write
them one time in `.env` (see step 3): `HORUS_COMFYUI_DIR`, `HORUS_MODELS_DIR` and
`HORUS_TEXT_ENCODER_DIR`. If you set nothing, ComfyUI goes to
`%USERPROFILE%\ComfyUI_windows_portable`.

## Step 0: get the horus program

```
go install ./cmd/horus
horus -h
```

**What it makes.** `go install` builds `horus` and puts it in the Go bin folder:
`%USERPROFILE%\go\bin` on Windows, `$HOME/go/bin` on macOS and Linux. The second
command must print the list of commands.

**The Go bin folder must be on the PATH.** The file `.mcp.json` starts the horus
MCP server by the name `horus`, so Claude Code must find `horus` on the PATH. The
Go installer for Windows usually adds the folder. If `horus -h` prints "not
recognized" or "command not found", add the folder to the PATH of your user
account. On Windows, run this in PowerShell, then open a new terminal:

```powershell
[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ";$env:USERPROFILE\go\bin", 'User')
```

On macOS and Linux, add `export PATH="$HOME/go/bin:$PATH"` to your shell profile
(for example `~/.zshrc`), then open a new terminal.

After a `git pull`, run `go install ./cmd/horus` again. The installed program
does not change until you build it again.

To build without an install, run `go build -o bin/horus.exe ./cmd/horus`
(`bin/horus` on macOS and Linux). Git ignores `bin/`. Call it as `bin\horus.exe`.
`.mcp.json` does not find this copy, because `bin` is not on the PATH.

**If it fails.**

- `go: command not found` or "not recognized": install Go, then open a new terminal.

## Step 1: install ComfyUI

```
horus install-comfyui
horus start-comfyui
```

To use another drive, add `-install-dir E:\AI\ComfyUI_windows_portable` to the first
command and `-comfyui-dir E:\AI\ComfyUI_windows_portable` to the second.

**What it makes.** The first command downloads the official ComfyUI portable build
(2 GB), checks its SHA-256 hash, and unpacks it. The portable build has its own Python.
The second command starts the server and waits until `http://127.0.0.1:8188` answers.
The first start can take a few minutes. Open that address in a browser to see ComfyUI.

Both commands are Windows-only in this phase. On macOS and Linux, install and start
ComfyUI by hand.

**If it fails.**

- `7-Zip was not found`: install 7-Zip, or pass `-seven-zip C:\path\to\7z.exe`.
- `Not enough free space`: choose a drive with more space.
- `SHA-256 mismatch`: the command deleted the bad file. Run the command again.
- `already installed`: this is not an error. The command skips the install.
- ComfyUI stops at start with `WinError 1114`: install the Visual C++ Redistributable, then run `horus start-comfyui` again.
- Other start errors: read `horus-comfyui.err.log` in the ComfyUI folder.

## Step 2: install the comfy MCP server

```
horus install-comfy-mcp
```

**What it makes.** One Python virtual environment (venv). Claude Code starts each
server program itself when a session starts. You do not start them.

| Server | Program | Made by |
|---|---|---|
| comfy MCP server | `%USERPROFILE%\.venvs\comfy-mcp\Scripts\comfy-mcp.exe`, in a venv with `comfy-mcp` 0.10.0 and `comfy-cli` 1.25.0 | `horus install-comfy-mcp` |
| horus MCP server | `horus mcp`, the program of step 0 | `go install ./cmd/horus` |

The versions are fixed on purpose, so that every PC behaves the same. Keep the default
path of the venv. The file `.mcp.json` in this repo names both programs. If a program
is missing, Claude Code shows that server as failed in `/mcp`.

**If it fails.**

- `Python 3.12 was not found`: install Python 3.12 from python.org. Select "py launcher" in the installer.
- `pip install failed`: check your network, then run the command again.

## Step 3: put your Hugging Face token in `.env`

On Windows:

```
copy .env.example .env
notepad .env
```

On macOS and Linux: `cp .env.example .env`, then open `.env` in a text editor.

**What it makes.** A file `.env` that Git ignores. Follow the four steps written in
the file:

1. Sign in at huggingface.co.
2. Open `https://huggingface.co/black-forest-labs/flux-3-image-klein-comfy` and accept the FLUX Kommunity License.
3. Create a token of type "Read" at `https://huggingface.co/settings/tokens`.
4. Paste the token after `HF_TOKEN=`.

Also set the `HORUS_*` paths in this file if you chose other drives.

**If it fails later (in step 4).** The download shows the HTTP code.

| Code | Meaning | Fix |
|---|---|---|
| 401 | The token is missing, wrong or expired. | Create a new Read token and put it in `.env`. |
| 403 | The token is valid, but its account did not accept the license. | Accept the license on the model page with the same account that made the token. |

## Step 4: download the models, sync the workflow, run a smoke test

```
horus workflow install it-illustration
horus sync-workflows
horus check-setup -workflow it-illustration -full
```

If the text encoder must go to another drive, add `-text-encoder-dir E:\AI\models\text_encoders`
to the first command. Add `-models-dir` to move all model folders.

**What it makes.**

- The first command reads `workflows\it-illustration\models.json`. It downloads each model file to a `.part` file, checks the size and SHA-256 hash, and renames it only when the hash matches. It skips a file that is already correct. It never overwrites a file that differs.
- When a model folder is outside ComfyUI, the command adds it to `extra_model_paths.yaml`. It makes a backup first. **Restart ComfyUI after this change:** stop the old server, then run `horus start-comfyui`.
- The second command copies the workflow into ComfyUI as `horus-it-illustration.json`. You see it in the Workflows pane of the ComfyUI page.
- The third command is the smoke test (a quick check that everything is in place). It prints one `PASS` or `FAIL` line for each check (ComfyUI folder, ComfyUI server, comfy-mcp, comfy-cli, `.mcp.json`, horus). It exits with code 0 only if all checks pass. With `-full` it needs a few minutes, because it hashes 11 GB. Without `-full` it checks the file sizes only.

To check only, without a download, run `horus workflow check it-illustration`.
See [manage workflows](#manage-workflows-list-install-check-remove) for the other actions.

**If it fails.**

- `Not enough free space`: the command needs the file size plus 1 GiB. Free space, or use `-text-encoder-dir` or `-models-dir` to choose another drive.
- `HTTP 401` or `HTTP 403`: see the table in step 3.
- `Download did not finish`: run the same command again. It resumes the `.part` file.
- `SHA-256 mismatch`: the command deleted the bad download. Run the command again.
- `Refusing to overwrite`: a file with that name exists and is different. Move it or delete it yourself, then run again.
- `CONFLICT` from `horus sync-workflows`: the copy in ComfyUI differs from the repo copy. Compare both. Then run `horus sync-workflows -force` to use the repo copy. The old copy is kept as `.bak`.
- The model is missing in the ComfyUI list: restart ComfyUI.

## Step 5: connect the agent

Start the agent in this repo folder:

```
claude
```

Claude Code reads `.mcp.json` and asks whether to use the project servers `comfy-mcp`
and `horus`. Approve both. Then:

1. Type `/mcp`. It must show `comfy-mcp` and `horus` as connected.
2. Type: `run server_info`.

**What it makes.** A live link between the agent and your ComfyUI. `server_info` must
report that ComfyUI is running at `http://127.0.0.1:8188`.

**If it fails.**

- `horus` failed: Claude Code did not find `horus` on the PATH. Do step 0 again, open a new terminal, then restart `claude`. `horus check-setup` shows the line `horus on PATH`.
- `comfy-mcp` failed (`CONNECTION_CLOSED`): the program in `.mcp.json` does not exist. Run step 2 with the default path, then restart `claude`. `horus check-setup` tells you whether the path matches.
- `Pending approval` in `claude mcp list`: approve the server in an interactive `claude` session. To be asked again, run `claude mcp reset-project-choices`.
- `UnicodeEncodeError: 'charmap' codec` from `comfy-mcp`: set `PYTHONUTF8=1`. The `.mcp.json` file already does this.
- `server_info` shows `running: false`: run `horus start-comfyui`.

For Codex, add the same two servers to its configuration: the `comfy-mcp` program
of step 2, and the command `horus` with the argument `mcp`. Then restart Codex.

### Optional: the image tool for agents in other folders

```
horus register
```

**What it makes.** An entry `horus` in your Claude Code user settings (user scope).
It runs `claude mcp add --scope user horus -e HORUS_ROOT=<repo root> -- <path of horus> mcp`.
`<repo root>` is the absolute path of this repo. Without the entry, only a session in
this repo folder sees `horus`, through `.mcp.json` (project scope). An agent in any
folder then has the tools `list_workflows`, `describe_workflow`, `install_workflow`,
`uninstall_workflow` and `generate_image`. Check it with `claude mcp get horus` from
any folder: it shows `Connected` and `HORUS_ROOT`. See [`mcp.md`](mcp.md).

**Flags.** Run `horus register` inside this repo. It takes two flags:

- `-root <folder>`: the repo folder (it must have `workflows/`) to pass as `HORUS_ROOT`.
  Default: the repo found from the current folder.
- `-horus-path <path>`: the `horus` program that Claude Code starts. Default: the
  program that runs `horus register`.

When no repo is found and no `-root` is given, or when the folder has no `workflows/`,
`horus register` stops with an error and registers nothing.

**If it fails.** `The claude command was not found`: install Claude Code, or add the
stdio server `horus` with the argument `mcp` to your agent by hand. `SKIP: Claude Code
already knows a server named horus`: remove the old entry first with
`claude mcp remove horus --scope user`, then run the command again.

## Step 6: make your first image

Read [`workflows/it-illustration/README.md`](../workflows/it-illustration/README.md). It
explains how to write the picture description for the agent. Then ask the agent, for
example: "Use it-illustration to draw a Kubernetes cluster with three nodes."

Without an agent, you can run the same workflow from the command line with
`horus run-illustration -subject "..."`. `horus run-illustration -h` shows the
options.

**If it fails.** The README of the workflow lists the errors of the workflow itself.
The first image can take several minutes, because ComfyUI loads 11 GB of models.

## Check an existing machine at any time

```
horus check-setup                                      # ComfyUI, comfy-mcp, comfy-cli, horus on the PATH and .mcp.json
horus check-setup -workflow it-illustration            # also the size of each model file
horus check-setup -workflow it-illustration -full      # also the SHA-256 of each model file
```

## Manage workflows: list, install, check, remove

`horus workflow` manages the files that each workflow needs. A workflow
is a folder under `workflows\`. Its `models.json` lists every model file (diffusion
model, text encoder, VAE, LoRA and so on) with its size and SHA-256.

```
horus workflow list                                   # every workflow, its description, installed or not
horus workflow install it-illustration                # download and verify, then check
horus workflow check it-illustration                  # is this PC ready to run it?
horus workflow remove it-illustration                 # prints what it deletes, deletes nothing
horus workflow remove it-illustration -yes            # deletes the files
horus workflow remove it-illustration -only vae -yes  # deletes one file (a role or a file name)
```

| Action | What it does | What it never does |
|---|---|---|
| `list` | Prints each workflow with the first paragraph of its README, the number and size of its model files, and if they are on disk (size check). `-json` prints JSON. | Download or delete. |
| `install` | Downloads each missing file, checks the SHA-256, then runs `check`. A file that is present and correct is skipped. | Overwrite a file that differs. |
| `check` | Checks size and SHA-256 of each file (`-quick`: size only). Checks that `models.json` lists every model file that `workflow.json` loads. When ComfyUI runs, checks that the server has every node class and lists every model file. | Change a file. |
| `remove` | Without `-yes`, prints the plan. With `-yes`, deletes the files and unfinished `.part` downloads. | Delete a file that another workflow lists (unless `-force`), or a file whose size differs from `models.json`. |

The command finds a model file in the `-models-dir` folder, in ComfyUI's own
`models` folder, and in every folder that `extra_model_paths.yaml` maps. So a
text encoder on another drive is found when ComfyUI can find it.

`check` does not install a missing custom node pack. It names the missing node
class. Install the pack with the MCP tool `install_node`, then restart ComfyUI.

A file from another source (for example a LoRA from another Hugging Face
repository) gets its own `repo` and `revision`, or a full `url`, in its
`models.json` entry. The token goes only to `huggingface.co` hosts.
