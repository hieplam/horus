# Flow 1: set up a fresh Windows PC or Mac

This page takes a Windows PC with an NVIDIA GPU, or a Mac with Apple Silicon,
to your first AI image. An AI agent (Claude Code or Codex) makes the image. The
agent talks to ComfyUI through the comfy MCP server. Every step is a command
that is safe to run again.

The commands are subcommands of one program, `horus`. Step 0 builds it. Run all
commands in a terminal, from the root folder of this repo. Each command prints
its flags with `-h`, for example `horus workflow -h`. The commands are the same
on Windows and macOS. Where a step differs, the page shows both systems side by
side.

**The two commands to use.** `horus doctor` checks the machine and prints one
`PASS`, `WARN` or `FAIL` line for each check. `horus setup <workflow>` prints
what the machine needs for a workflow and changes nothing. `horus setup
<workflow> -yes` installs ComfyUI first, then the model files. The steps below
also name the older commands (`horus check-setup`, `horus install-comfyui`,
`horus workflow install`). They still work and do the same job.

**Linux is not supported yet.** `horus` builds and runs on Linux. Three
commands exit with "not supported on this OS yet" there: `horus
install-comfyui`, `horus start-comfyui` and `horus stop-comfyui`. So the
install step of `horus setup <workflow> -yes` also stops there. On Linux you
install and start ComfyUI by hand (see the ComfyUI README). All other steps are
the same. Intel Macs are not supported either.

## What is each part

| Part | What it is | Why you need it |
|---|---|---|
| ComfyUI | A program that makes images from a graph of steps (a "workflow"). It has a browser page and an HTTP server at `http://127.0.0.1:8188`. | It does the real work on your GPU. |
| Model files | Large files (`.safetensors`) that hold the learned weights. FLUX 3 Image Klein has three: a diffusion model, a text encoder and a VAE. | ComfyUI cannot make an image without them. They are 11 GB in total. |
| `horus` | One program, built from the Go code in this repo. It has the setup commands, the horus MCP server (`horus mcp`) and the command that starts the comfy MCP server (`horus comfy-mcp`). | It installs and checks everything below, and it gives other agents the image tools. |
| comfy-cli | A command-line tool from the ComfyUI team, the program `comfy`, version 1.25.0. | The comfy MCP server calls it. On macOS, `horus` also installs, starts and stops ComfyUI with it. |
| comfy MCP server (`comfy-mcp`) | A small program that speaks the Model Context Protocol (MCP). MCP is the standard way for an agent to call tools. | It gives the agent safe tools: `server_info`, `validate_workflow`, `run_workflow` and more. |
| Agent | Claude Code or Codex. | You describe the image. The agent calls the tools. |

**Why the agent never edits ComfyUI directly.** The agent can break a working install
if it changes files in ComfyUI or in the workflow JSON by hand. The MCP tools check
a workflow before they run it, and they change one named setting at a time. So the
agent uses only the tools. The repo keeps the master copy of each workflow.

## Windows and macOS: what differs

| Item | Windows | macOS |
|---|---|---|
| Hardware | An NVIDIA GPU with 6 GB VRAM or more, and 16 GB RAM. | Apple Silicon (an M-series chip) with 16 GB of memory. Tested on an Apple M4 with 16 GB. |
| How `horus` installs ComfyUI | It downloads the official Windows portable build (2 GB). The build has its own Python. It needs 7-Zip. | It runs comfy-cli: `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`. comfy-cli makes a Python venv and installs PyTorch for the GPU of the Mac. |
| Where `comfy` comes from | The venv of `horus install-comfy-mcp` (step 1). | The same venv, or the PATH. `horus` looks on the PATH first. |
| Default install folder (`HORUS_COMFYUI_DIR`) | `%USERPROFILE%\ComfyUI_windows_portable` | `~/horus`. ComfyUI goes to `~/horus/ComfyUI`. |
| Default models folder | `%USERPROFILE%\ComfyUI_windows_portable\ComfyUI\models` | `~/horus/ComfyUI/models` |
| The Go bin folder | `%USERPROFILE%\go\bin` | `~/go/bin` |
| The venv of the comfy MCP server | `%USERPROFILE%\.venvs\comfy-mcp` | `~/.venvs/comfy-mcp` |
| Time of the first image | 217 s to 470 s for each image (a 6 GB GTX 1660 Ti class card). | 1033 s for the first image after a cold start, 469 s for the next one (Apple M4, 16 GB). |
| Privacy folders | none | `horus doctor` prints a `WARN` line for a path in `~/Desktop`, `~/Documents` or `~/Downloads`. See [the macOS privacy folders](#the-macos-privacy-folders). |

The times are 1536x1024 images with 15 steps. The time limit of one image is
1800 s (`HORUS_GENERATE_TIMEOUT`).

### The macOS privacy folders

macOS protects three folders of your home folder: `~/Desktop`, `~/Documents`
and `~/Downloads`. This rule is TCC (Transparency, Consent and Control). A
program can read a file in these folders only after you grant it access. So
ComfyUI and comfy-mcp can fail to read a file there. The default folders of
`horus` are outside these folders.

`horus doctor` checks five paths: the repo root, the install folder, the models
folder, the text encoder folder and the output folder. For each path inside a
protected folder, it prints one line like this:

```
WARN  macOS privacy folder - the repo root /Users/you/Desktop/horus is inside /Users/you/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of /Users/you/Desktop, or grant access in System Settings > Privacy & Security.
```

A `WARN` line does not change the exit code. The last line stays
`All checks passed.` when no check failed. To get no `WARN` line, clone the
repo outside the three folders.

**Do not clone the repo into `~/horus`.** `~/horus` is the default install
folder of ComfyUI on macOS. Clone the repo into another folder, for example
`~/src/horus`.

## Prerequisites

Check each item before step 0.

**On Windows:**

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

**On macOS:**

| Item | What you need | How to check |
|---|---|---|
| Chip | Apple Silicon (an M-series chip). | Apple menu > About This Mac |
| Memory | 16 GB. ComfyUI used 9.1 GB in the Mac test run, and the Mac swapped. Close large programs before you make images. | Apple menu > About This Mac |
| Disk | About 2 GB for ComfyUI and 11 GB for models. Keep 1 GiB spare. | `df -h ~` |
| Git | Any recent version. The Xcode Command Line Tools have it (`xcode-select --install`). | `git --version` |
| Homebrew | The package manager that installs the next three items. | `brew --version` |
| Go 1.27.3 or newer, Python 3.12 and uv | `brew install python@3.12 go uv`. Go builds `horus`. `horus install-comfy-mcp` makes its venv with `python3.12`. comfy-cli depends on uv. The Mac test run installed all three with this command. | `go version` and `python3.12 --version` |
| Hugging Face account | Free. You need it to accept the model license. | [huggingface.co](https://huggingface.co) |

**Choose the folders first.** ComfyUI and the models can live on different
drives. You set the paths in step 2 and step 4. You can also write them one time
in `.env` (see step 3): `HORUS_COMFYUI_DIR`, `HORUS_MODELS_DIR` and
`HORUS_TEXT_ENCODER_DIR`. If you set nothing, `horus` uses the default folders
of the table above.

## Step 0: get the horus program

```
go install ./cmd/horus
horus -h
```

**What it makes.** `go install` builds `horus` and puts it in the Go bin folder:
`%USERPROFILE%\go\bin` on Windows, `~/go/bin` on macOS. The second command must
print the list of commands.

**The Go bin folder must be on the PATH.** The file `.mcp.json` starts both MCP
servers by the name `horus`, so Claude Code must find `horus` on the PATH. The
Go installer for Windows usually adds the folder. Homebrew on macOS does not.
If `horus -h` prints "not recognized" or "command not found", add the folder to
the PATH of your user account.

On Windows, run this in PowerShell, then open a new terminal:

```powershell
[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ";$env:USERPROFILE\go\bin", 'User')
```

On macOS, add `export PATH="$HOME/go/bin:$PATH"` to your shell profile
(`~/.zshrc`), then open a new terminal.

After a `git pull`, run `go install ./cmd/horus` again. The installed program
does not change until you build it again.

To build without an install, run `go build -o bin/horus.exe ./cmd/horus`
(`bin/horus` on macOS). Git ignores `bin/`. Call it as `bin\horus.exe`
(`bin/horus` on macOS). `.mcp.json` does not find this copy, because `bin` is
not on the PATH.

**If it fails.**

- `go: command not found` or "not recognized": install Go, then open a new terminal.

## Step 1: install the comfy MCP server

```
horus install-comfy-mcp
```

Do this step before step 2. On macOS, step 2 needs the program `comfy` that
this step installs.

**What it makes.** One Python virtual environment (venv) with `comfy-mcp` 0.10.0
and `comfy-cli` 1.25.0. On Windows, `horus` makes it with the Python Launcher
(`py -3.12`). On macOS, it makes it with `python3.12`. Claude Code starts each
server program itself when a session starts. You do not start them.

| Server | Program | Made by |
|---|---|---|
| comfy MCP server | `horus comfy-mcp`. It runs the `comfy-mcp` of the venv, or the `comfy-mcp` of the PATH when the venv has none. | `horus install-comfy-mcp` |
| horus MCP server | `horus mcp`, the program of step 0 | `go install ./cmd/horus` |

The file `.mcp.json` in this repo starts both servers through `horus`, on
Windows and on macOS. `horus comfy-mcp` sets `PYTHONUTF8=1` and
`PYTHONIOENCODING=utf-8` for comfy-mcp. When `COMFY_BIN` is not set and a
`comfy` program is next to comfy-mcp, it sets `COMFY_BIN` to that program. The versions are fixed on
purpose, so that every machine behaves the same. Keep the default path of the
venv. If a program is missing, Claude Code shows that server as failed in
`/mcp`.

**If it fails.**

- Windows, `The Python Launcher (py.exe) was not found`: install Python 3.12 from python.org. Select "py launcher" in the installer.
- macOS, `python3.12 was not found`: run `brew install python@3.12`, then open a new terminal.
- `pip install failed`: check your network, then run the command again.

## Step 2: install ComfyUI

```
horus install-comfyui
horus start-comfyui
```

To use another folder, add `-install-dir <folder>` to the first command and
`-comfyui-dir <folder>` to the second. For example `E:\AI\ComfyUI_windows_portable`
on Windows, or `/Volumes/Data/horus` on macOS.

**What it makes on Windows.** The first command downloads the official ComfyUI
portable build (2 GB), checks its SHA-256 hash, and unpacks it. The portable
build has its own Python.

**What it makes on macOS.** The first command runs comfy-cli. comfy-cli
installs ComfyUI 0.44.0 in `~/horus/ComfyUI`, with its own Python venv in
`~/horus/ComfyUI/.venv` and PyTorch for the GPU of the Mac (MPS, Metal
Performance Shaders). In the Mac test run, this took about 2 minutes and 2.0 GB
of disk.

**On both systems.** The second command starts the server and waits until
`http://127.0.0.1:8188` answers. The first start can take a few minutes. Open
that address in a browser to see ComfyUI. ComfyUI keeps running when the
terminal closes. `horus stop-comfyui` stops it, but only a ComfyUI that horus
started. `horus start-comfyui` writes the file `horus-comfyui.state.json` in
the install folder, and `horus stop-comfyui` reads it. When a person started
the ComfyUI that answers, `horus stop-comfyui` refuses and stops nothing.

**The short way.** `horus setup it-illustration` does two parts of the setup
in one command: the install of ComfyUI (the first command of this step) and
the download of the model files (the first command of step 4). It does not
start ComfyUI, and it does not run `horus sync-workflows` or `horus doctor`.
Run it first without `-yes`. It prints the plan: one line for each model file
(`PRESENT` or `MISSING`), then the numbered steps that it will run. It changes
nothing. When the plan is right, run `horus setup it-illustration -yes`. With
`-yes` it installs ComfyUI if it is missing, then downloads only the model
files that the plan lists. Press Ctrl+C to stop it. The download that runs
ends, the `.part` file stays, and the next run resumes it. A second run prints
`Plan: 0 steps. Nothing is missing.` Do step 3 (the token) before you run it
with `-yes`. Then run these three commands yourself:

```
horus start-comfyui
horus sync-workflows
horus doctor -workflow it-illustration
```

**If it fails.**

- `7-Zip was not found` (Windows): install 7-Zip, or pass `-seven-zip C:\path\to\7z.exe`.
- `comfy-cli (the program comfy) was not found` (macOS): run step 1 first, or `uv tool install comfy-cli==1.25.0`.
- `-seven-zip is a flag of the Windows portable build` (macOS): the flags `-archive-path`, `-seven-zip` and `-keep-archive` are for Windows only. Remove the flag.
- `comfy install failed` (macOS): read the lines of comfy above the error, then run the command again.
- `Not enough free space`: choose a drive with more space.
- `SHA-256 mismatch`: the command deleted the bad file. Run the command again.
- `already installed`: this is not an error. The command skips the install.
- Ctrl+C during `horus install-comfyui` or `horus setup -yes`: the command stops. Run it again.
- ComfyUI stops at start with `WinError 1114` (Windows): install the Visual C++ Redistributable, then run `horus start-comfyui` again.
- `Another ComfyUI is already using this database`: ComfyUI 0.44 locks its database in the folder `ComfyUI/user`. Two ComfyUI programs cannot run from one install folder. Stop the first one, or install a second copy in another folder and set `HORUS_COMFYUI_DIR` and `HORUS_COMFYUI_URL` for it.
- Other start errors: read the log. On Windows it is `horus-comfyui.err.log` in the install folder. On macOS it is `ComfyUI/user/comfyui_8188.log` in the install folder (the number is the port).

## Step 3: put your Hugging Face token in `.env`

On Windows:

```
copy .env.example .env
notepad .env
```

On macOS: `cp .env.example .env`, then open `.env` in a text editor, for
example `open -e .env`.

**What it makes.** A file `.env` that Git ignores. Follow the four steps written in
the file:

1. Sign in at huggingface.co.
2. Open `https://huggingface.co/black-forest-labs/flux-3-image-klein-comfy` and accept the FLUX Kommunity License.
3. Create a token of type "Read" at `https://huggingface.co/settings/tokens`.
4. Paste the token after `HF_TOKEN=`.

Also set the `HORUS_*` paths in this file if you chose other folders. On macOS
with the default folders, you set no path.

**Who reads the token.** Only the download code of `horus` reads `HF_TOKEN`. It
reads the environment variable first, then the `.env` file in the repo root. It
sends the token only to `huggingface.co` hosts. The horus MCP server reads it in
its own process while `install_workflow` runs.

**If it fails later (in step 4).** The download shows the HTTP code.

| Code | Meaning | Fix |
|---|---|---|
| 401 | The token is missing, wrong or expired. | Create a new Read token and put it in `.env`. |
| 403 | The token is valid, but its account did not accept the license. | Accept the license on the model page with the same account that made the token. |

## Step 4: download the models, sync the workflow, run a smoke test

```
horus workflow install it-illustration
horus sync-workflows
horus doctor -workflow it-illustration
```

If the text encoder must go to another drive, add `-text-encoder-dir <folder>`
to the first command, for example `E:\AI\models\text_encoders`. Add
`-models-dir` to move all model folders.

**What it makes.**

- The first command reads `workflows/it-illustration/models.json`. It downloads each model file to a `.part` file, checks the size and SHA-256 hash, and renames it only when the hash matches. It skips a file that is already correct. It never overwrites a file that differs. In the Mac test run, the 11 GB took about 5 minutes.
- When a model folder is outside ComfyUI, the command adds it to `extra_model_paths.yaml`. It makes a backup first. **Restart ComfyUI after this change:** run `horus stop-comfyui` (it stops a ComfyUI that horus started; stop any other one yourself), then run `horus start-comfyui`.
- The second command copies the workflow into ComfyUI as `horus-it-illustration.json`. You see it in the Workflows pane of the ComfyUI page.
- The third command (`horus doctor`, the new name of `horus check-setup`) is the smoke test (a quick check that everything is in place). It prints one `PASS` or `FAIL` line for each check (ComfyUI folder, ComfyUI server, comfy-mcp, comfy-cli, `.mcp.json`, horus). On macOS it can also print `WARN` lines for [the privacy folders](#the-macos-privacy-folders). It exits with code 0 only if no check fails. With `-workflow` it also checks the size and the SHA-256 of each model file. It reads all 11 GB: in the Mac test run (Apple M4) that took 5 s, and a slower disk takes longer. With `-quick` it checks the file sizes only.

To check only, without a download, run `horus workflow check it-illustration`.
See [manage workflows](#manage-workflows-list-install-check-remove) for the other actions.

**If it fails.**

- `Not enough free space`: the command needs the file size plus 1 GiB. Free space, or use `-text-encoder-dir` or `-models-dir` to choose another drive.
- `HTTP 401` or `HTTP 403`: see the table in step 3.
- `Download did not finish`: run the same command again. It resumes the `.part` file.
- Ctrl+C during the install: the command prints `FAIL: context canceled` after the download that runs. No next file starts. The `.part` file stays. Run the command again.
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

- `horus` failed: Claude Code did not find `horus` on the PATH. Do step 0 again, open a new terminal, then restart `claude`. `horus doctor` shows the line `horus on PATH`.
- `comfy-mcp` failed (`CONNECTION_CLOSED`): `horus` is not on the PATH, or the venv of step 1 is missing. Then `horus comfy-mcp` writes `comfy-mcp was not found in ... Fix: horus install-comfy-mcp` and exits 1. Run step 1, then restart `claude`. `horus doctor` shows the lines `comfy-mcp environment` and `.mcp.json`.
- `Pending approval` in `claude mcp list`: approve the server in an interactive `claude` session. To be asked again, run `claude mcp reset-project-choices`.
- `server_info` shows `running: false`: run `horus start-comfyui`. The horus tool `generate_image` also starts ComfyUI by itself, on Windows and on macOS.

For Codex, add the same two servers to its configuration: the command `horus`
with the argument `comfy-mcp`, and the command `horus` with the argument `mcp`.
Then restart Codex.

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
The first image takes longer, because ComfyUI loads 11 GB of models. See the
times in [the table of differences](#windows-and-macos-what-differs).

## Check an existing machine at any time

```
horus doctor                                      # ComfyUI, comfy-mcp, comfy-cli, horus on the PATH and .mcp.json
horus doctor -workflow it-illustration            # also the size and the SHA-256 of each model file
horus doctor -workflow it-illustration -quick     # also the size of each model file only
horus setup it-illustration                       # what this machine still needs for the workflow (changes nothing)
```

`horus check-setup` is the older name of `horus doctor` and prints the same lines.
Without `-server-url`, `horus doctor` checks the address in `HORUS_COMFYUI_URL`,
then `http://127.0.0.1:8188`.

## Manage workflows: list, install, check, remove

`horus workflow` manages the files that each workflow needs. A workflow
is a folder under `workflows/`. Its `models.json` lists every model file (diffusion
model, text encoder, VAE, LoRA and so on) with its size and SHA-256.

```
horus workflow list                                   # every workflow, its description, installed or not
horus workflow install it-illustration                # download and verify, then check
horus workflow check it-illustration                  # is this machine ready to run it?
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

A model folder can be outside ComfyUI. On a machine where the model files live on
another drive, `extra_model_paths.yaml` in the ComfyUI folder maps that folder,
and the model files are not in the ComfyUI folder. The command finds them
through that file.

The command finds a model file in the `-models-dir` folder, in ComfyUI's own
`models` folder, and in every folder that `extra_model_paths.yaml` maps. So a
text encoder on another drive is found when ComfyUI can find it.

`check` does not install a missing custom node pack. It names the missing node
class. Install the pack with the MCP tool `install_node`, then restart ComfyUI.

A file from another source (for example a LoRA from another Hugging Face
repository) gets its own `repo` and `revision`, or a full `url`, in its
`models.json` entry. The token goes only to `huggingface.co` hosts.
