# Flow 1: set up a fresh Windows PC or Mac

This page takes a Windows PC with an NVIDIA GPU, or a Mac with Apple Silicon,
to your first AI image. An AI agent (Claude Code or Codex) makes the image. The
agent calls one tool server, the horus MCP server. It never sees ComfyUI. Every
step is a command that is safe to run again.

The commands are subcommands of one program, `horus`. Step 0 installs it with one
line. Run the commands in a terminal. They need no clone of this repo, no Git,
no Go, and on Windows no Python. Each command prints its flags with `-h`, for
example `horus setup -h`. The commands are the same on Windows and macOS. Where
a step differs, the page shows both systems side by side.

**The commands to use.**

- `horus setup <workflow>` prints what the machine needs for a workflow (the plan) and changes nothing.
- `horus setup <workflow> -yes` runs the plan: ComfyUI first, then the model files. A speech workflow needs no ComfyUI: its plan makes the speech venv, then downloads the model files ([set up a speech workflow](#set-up-a-speech-workflow)).
- `horus setup <workflow> -yes -with-packages` also installs a system package that the plan needs (7-Zip with `winget` on Windows, Python 3.12 with `brew` on macOS, and for a speech workflow Python 3.12 and FFmpeg). Without `-with-packages`, horus never installs a system package.
- `horus register` adds the horus MCP server to Claude Code. `horus register -client codex` adds it to Codex.
- `horus doctor` checks the machine and prints one `PASS`, `WARN` or `FAIL` line for each check.
- `horus uninstall` prints what horus installed and changes nothing. `horus uninstall -yes` removes it ([details](#uninstall)).

A developer who changes horus works in a clone of this repo. That path has more
steps: see [for developers: work in a clone](#for-developers-work-in-a-clone).

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
| `horus` | One program. The release has it as one file for each system. It holds the commands and the horus MCP server. | It installs and checks everything in this table. |
| horus MCP server | `horus mcp`. MCP (Model Context Protocol) is the standard way for an agent to call tools. The tools: `list_workflows`, `describe_workflow`, `install_workflow`, `uninstall_workflow`, `generate_image`, `generate_speech`. | The agent calls it to make an image or speech. For an image, it sends the job to ComfyUI over HTTP and returns the path of the PNG. See [`mcp.md`](mcp.md). |
| comfy-cli | A command-line tool from the ComfyUI team, the program `comfy`, version 1.25.0. | macOS only: `horus` installs, starts and stops ComfyUI with it. |
| Agent | Claude Code or Codex. | You describe the image. The agent calls the horus tools. |

**Why the agent never touches ComfyUI.** The horus MCP server changes only the
inputs that the `manifest.json` of a workflow names: the caption, the seed, the
size and the file name. It checks the graph against the running ComfyUI before
it sends the job. So an agent cannot break a workflow or the install of ComfyUI.
The repo keeps the master copy of each workflow.

## Windows and macOS: what differs

| Item | Windows | macOS |
|---|---|---|
| Hardware | An NVIDIA GPU with 6 GB VRAM or more, and 16 GB RAM. | Apple Silicon (an M-series chip) with 16 GB of memory. Tested on an Apple M4 with 16 GB. |
| How `horus` installs ComfyUI | It downloads the official Windows portable build (2 GB). The build has its own Python. It needs 7-Zip. | It runs comfy-cli: `comfy --workspace <dir>/ComfyUI install --m-series --version 0.44.0`. comfy-cli makes a Python venv and installs PyTorch for the GPU of the Mac. |
| Where `comfy` comes from | Not needed. | The venv `~/.venvs/comfy-mcp` that `horus setup` makes, or the PATH. `horus` looks on the PATH first. |
| Default install folder (`HORUS_COMFYUI_DIR`) | `%USERPROFILE%\ComfyUI_windows_portable` | `~/horus`. ComfyUI goes to `~/horus/ComfyUI`. |
| Default models folder | `%USERPROFILE%\ComfyUI_windows_portable\ComfyUI\models` | `~/horus/ComfyUI/models` |
| Time of the first image | 217 s to 470 s for each image (a 6 GB GTX 1660 Ti class card). | 1033 s for the first image after a cold start, 469 s for the next one (Apple M4, 16 GB). |
| Privacy folders | none | `horus doctor` prints a `WARN` line for a path in `~/Desktop`, `~/Documents` or `~/Downloads`. See [the macOS privacy folders](#the-macos-privacy-folders). |

The times are 1536x1024 images with 15 steps. The time limit of one image is
1800 s (`HORUS_GENERATE_TIMEOUT`).

### The macOS privacy folders

macOS protects three folders of your home folder: `~/Desktop`, `~/Documents`
and `~/Downloads`. This rule is TCC (Transparency, Consent and Control). A
program can read a file in these folders only after you grant it access. So
ComfyUI can fail to read a file there. The default folders of `horus` are
outside these folders.

`horus doctor` checks five paths: the repo root, the install folder, the models
folder, the text encoder folder and the output folder. For each path inside a
protected folder, it prints one line like this:

```
WARN  macOS privacy folder - the repo root /Users/you/Desktop/horus is inside /Users/you/Desktop, a folder that macOS protects. ComfyUI and comfy-mcp can fail to read it. Fix: move it out of /Users/you/Desktop, or grant access in System Settings > Privacy & Security.
```

A `WARN` line does not change the exit code. The last line stays
`All checks passed.` when no check failed. To get no `WARN` line, keep the
folders, and a clone, outside the three folders.

**Do not clone the repo into `~/horus`.** `~/horus` is the default install
folder of ComfyUI on macOS. Clone the repo into another folder, for example
`~/src/horus`.

## Prerequisites

Check each item before step 0. A developer of this repo needs more: see
[for developers](#for-developers-work-in-a-clone).

**On Windows:**

| Item | What you need | How to check |
|---|---|---|
| GPU | NVIDIA, 6 GB VRAM or more. The workflow was tested on a 6 GB card. | `nvidia-smi` |
| NVIDIA driver | Version 528.33 or newer. | `nvidia-smi` shows the version. |
| RAM | 16 GB. Close large programs before you make images. | Task Manager |
| Disk | About 9 GB for ComfyUI and 11 GB for models. Keep 1 GiB spare on each drive that gets a download. | File Explorer |
| 7-Zip | Unpacks ComfyUI. `horus setup -yes -with-packages` installs it with `winget`. | `winget install 7zip.7zip` |
| Microsoft Visual C++ x64 Redistributable | PyTorch needs it. Without it, ComfyUI fails with `WinError 1114` and `vcruntime140_1.dll`. | [Download from Microsoft](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist) |
| Hugging Face account | Free. You need it to accept the model license. | [huggingface.co](https://huggingface.co) |
| An agent | Claude Code or Codex. | `claude --version` or `codex --version` |

**On macOS:**

| Item | What you need | How to check |
|---|---|---|
| Chip | Apple Silicon (an M-series chip). | Apple menu > About This Mac |
| Memory | 16 GB. ComfyUI used 9.1 GB in the Mac test run, and the Mac swapped. Close large programs before you make images. | Apple menu > About This Mac |
| Disk | About 2 GB for ComfyUI and 11 GB for models. Keep 1 GiB spare. | `df -h ~` |
| Homebrew | The package manager that installs Python 3.12. | `brew --version` |
| Python 3.12 | `horus setup` makes the venv of comfy-cli with `python3.12`. `horus setup -yes -with-packages` installs it with `brew install python@3.12`. | `python3.12 --version` |
| Hugging Face account | Free. You need it to accept the model license. | [huggingface.co](https://huggingface.co) |
| An agent | Claude Code or Codex. | `claude --version` or `codex --version` |

**Choose the folders first.** ComfyUI and the models can live on different
drives. You set the paths in step 2, with flags, or one time in `.env` (step 1):
`HORUS_COMFYUI_DIR`, `HORUS_MODELS_DIR` and `HORUS_TEXT_ENCODER_DIR`. If you set
nothing, `horus` uses the default folders of the table above.

## Step 0: install the horus program

On Windows 10 or 11 (64-bit), run this in PowerShell:

```
irm https://raw.githubusercontent.com/hieplam/horus/main/install.ps1 | iex
```

On macOS with Apple Silicon, run this in Terminal:

```
curl -fsSL https://raw.githubusercontent.com/hieplam/horus/main/install.sh | sh
```

Then open a new terminal and check the program:

```
horus version
```

**What it makes.** The script downloads the program and `checksums.txt` from the newest
[GitHub Release](https://github.com/hieplam/horus/releases), and calculates the SHA-256
of the download. When the SHA-256 is not the one in `checksums.txt`, the script deletes
the download, prints both values and installs nothing. Then it moves the program to its
place and puts that folder on the PATH of your user account.

| System | Program file | PATH |
|---|---|---|
| Windows | `%LOCALAPPDATA%\Programs\horus\horus.exe` | The folder joins the PATH of your user account (the registry value `Path` in `HKCU\Environment`). |
| macOS | `~/.local/bin/horus` | One line with the marker `# added by the horus installer` joins your shell profile, when the folder is not on the PATH. |

`horus version` must print `horus <version>`. Run the install line again to update.
The scripts read these environment variables:

| Variable | Effect |
|---|---|
| `HORUS_VERSION` | Installs this release, for example `0.1.0`. Default: the newest release. |
| `HORUS_INSTALL_DIR` | Installs the program in this folder. |
| `HORUS_INSTALL_NO_PATH=1` | Leaves the PATH and the shell profile unchanged. |

The manual steps (download the file, check its SHA-256, put it on the
PATH) are below: [install by hand](#install-by-hand).

**The user folder.** horus keeps its own files in `<home>/horus`: `%USERPROFILE%\horus` on
Windows, `~/horus` on macOS. `HORUS_HOME` changes it. This folder holds `.env`, `outputs/`
and the copy of the built-in workflows (`builtin-workflows/`). The program holds the
default workflows inside itself, so it needs no clone of this repo. A clone of this repo
still has priority: inside a clone, horus uses the `workflows/` and the `.env` of the clone.

**If it fails.**

- `horus` is "not recognized" or "command not found": open a new terminal. The old terminal has the old PATH.
- `the SHA-256 of the download is not the SHA-256 in checksums.txt`: the download is damaged. The script installed nothing. Run the line again.
- `this script supports Windows only` or `macOS on Apple silicon only`: you ran the script of the other system, or this machine is not supported (Intel Macs and Linux).

### Install by hand

Use these steps when you do not want to run a script from the
internet. There are three: download the program, check its SHA-256, and put it on
the PATH. Do the steps of your system.

On Windows, in PowerShell, download the program and the list of hashes:

```
Invoke-WebRequest https://github.com/hieplam/horus/releases/latest/download/horus-windows-amd64.exe -OutFile horus.exe
Invoke-WebRequest https://github.com/hieplam/horus/releases/latest/download/checksums.txt -OutFile checksums.txt
```

Print the SHA-256 of your download, then the line of `checksums.txt` for the same
file. The two hashes must be equal, letter for letter. If they differ, delete
`horus.exe` and do not run it.

```
(Get-FileHash horus.exe -Algorithm SHA256).Hash
Select-String horus-windows-amd64.exe checksums.txt
```

Move the program to its folder, then add the folder to the PATH of your user account:

```
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\horus"
Move-Item horus.exe "$env:LOCALAPPDATA\Programs\horus\horus.exe" -Force
[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ";$env:LOCALAPPDATA\Programs\horus", 'User')
```

On macOS, in Terminal, download the program and the list of hashes:

```
curl -fsSLO https://github.com/hieplam/horus/releases/latest/download/horus-darwin-arm64
curl -fsSLO https://github.com/hieplam/horus/releases/latest/download/checksums.txt
```

Print the SHA-256 of your download, then the line of `checksums.txt` for the same
file. The two hashes must be equal. If they differ, delete `horus-darwin-arm64` and
do not run it.

```
shasum -a 256 horus-darwin-arm64
grep horus-darwin-arm64 checksums.txt
```

Make the file a program, move it to its folder, and add the folder to the PATH:

```
chmod +x horus-darwin-arm64
mkdir -p "$HOME/.local/bin"
mv horus-darwin-arm64 "$HOME/.local/bin/horus"
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
```

## Step 1: put your Hugging Face token in `.env`

Make the file `.env` in the user folder: `%USERPROFILE%\horus\.env` on Windows,
`~/horus/.env` on macOS. Write one line in it:

```
HF_TOKEN=<your token>
```

To get the token:

1. Sign in at huggingface.co.
2. Open `https://huggingface.co/black-forest-labs/flux-3-image-klein-comfy` and accept the FLUX Kommunity License.
3. Create a token of type "Read" at `https://huggingface.co/settings/tokens`.
4. Paste the token after `HF_TOKEN=`.

Also set the `HORUS_*` paths in this file if you chose other folders. On macOS
with the default folders, you set no path. In a clone, horus reads the `.env`
of the clone instead: copy `.env.example` to `.env` there. Git ignores `.env`.

**Who reads the token.** Only the download code of `horus` reads `HF_TOKEN`. It
reads the environment variable first, then the `.env` file (of the clone, or of
the user folder when there is no clone). It sends the token only to
`huggingface.co` hosts. The horus MCP server reads it in its own process while
`install_workflow` runs.

**If it fails later (in step 2).** The download shows the HTTP code.

| Code | Meaning | Fix |
|---|---|---|
| 401 | The token is missing, wrong or expired. | Create a new Read token and put it in `.env`. |
| 403 | The token is valid, but its account did not accept the license. | Accept the license on the model page with the same account that made the token. |

## Step 2: set up the workflow

```
horus setup it-illustration
```

This prints the plan and changes nothing. The plan has one line for each model
file (`PRESENT` or `MISSING`), then the numbered steps that `-yes` runs. When
the plan is right, run it:

```
horus setup it-illustration -yes
```

To use other folders, add `-comfyui-dir <folder>`, `-models-dir <folder>` or
`-text-encoder-dir <folder>`. For example `E:\AI\ComfyUI_windows_portable` on
Windows, or `/Volumes/Data/horus` on macOS.

**A system package.** On a new machine, the plan can start with a package step:
7-Zip with `winget` on Windows, Python 3.12 with `brew` on macOS. A package step
is a change of the system, so horus asks you first. Without approval,
`horus setup it-illustration -yes` prints the package and its command, changes
nothing and exits 1. To approve it, run:

```
horus setup it-illustration -yes -with-packages
```

Or install the package yourself. When `winget` or `brew` is not on the PATH,
the plan says what to install and where to get it, and `-yes` changes nothing.
horus never removes a system package.

**What it makes on Windows.** horus downloads the official ComfyUI portable
build (2 GB), checks its SHA-256 hash, and unpacks it. The portable build has
its own Python.

**What it makes on macOS.** horus makes a Python venv with comfy-cli in
`~/.venvs/comfy-mcp`. Then comfy-cli installs ComfyUI 0.44.0 in
`~/horus/ComfyUI`, with its own Python venv in `~/horus/ComfyUI/.venv` and
PyTorch for the GPU of the Mac (MPS, Metal Performance Shaders). In the Mac test
run, this took about 2 minutes and 2.0 GB of disk.

**The model files, on both systems.**

- horus reads the `models.json` of the workflow. It downloads each model file to a `.part` file, checks the size and SHA-256 hash, and renames it only when the hash matches. It skips a file that is already correct. It never overwrites a file that differs. In the Mac test run, the 11 GB took about 5 minutes.
- When a model folder is outside ComfyUI, horus adds it to `extra_model_paths.yaml`. It makes a backup first. A ComfyUI that runs reads this file only at start: run `horus stop-comfyui`, then `horus start-comfyui`.

A second run prints `Plan: 0 steps. Nothing is missing.` Press Ctrl+C to stop
a run. The download that runs ends, the `.part` file stays, and the next run
resumes it.

**ComfyUI starts by itself.** `horus setup` does not start ComfyUI. The first
`generate_image` starts it. To start it now and open its page at
`http://127.0.0.1:8188`, run `horus start-comfyui`. ComfyUI keeps running when
the terminal closes. `horus stop-comfyui` stops a ComfyUI that horus started,
and no other one.

**If it fails.**

- `7-Zip was not found` (Windows): approve the package step with `-with-packages`, or install 7-Zip yourself.
- `comfy install failed` (macOS): read the lines of comfy above the error, then run the command again.
- `Not enough free space`: the download needs the file size plus 1 GiB. Free space, or use `-text-encoder-dir` or `-models-dir` to choose another drive.
- `HTTP 401` or `HTTP 403`: see the table in step 1.
- `Download did not finish`: run the same command again. It resumes the `.part` file.
- `SHA-256 mismatch`: horus deleted the bad file. Run the command again.
- `Refusing to overwrite`: a file with that name exists and is different. Move it or delete it yourself, then run again.
- ComfyUI stops at start with `WinError 1114` (Windows): install the Visual C++ Redistributable, then run `horus start-comfyui` again.
- `Another ComfyUI is already using this database`: ComfyUI 0.44 locks its database in the folder `ComfyUI/user`. Two ComfyUI programs cannot run from one install folder. Stop the first one, or install a second copy in another folder and set `HORUS_COMFYUI_DIR` and `HORUS_COMFYUI_URL` for it.
- Other start errors: read the log. On Windows it is `horus-comfyui.err.log` in the install folder. On macOS it is `ComfyUI/user/comfyui_8188.log` in the install folder (the number is the port).

### Set up a speech workflow

A speech workflow, for example `vi-speech`, makes speech on the CPU. It needs
no ComfyUI and no GPU. Its model files are 1.1 GB.

```
horus setup vi-speech
```

This prints the plan and changes nothing. The plan has these steps, in order:

1. A package step for Python 3.12, only when the speech venv must be made and no Python 3.12 is found: `winget install --id Python.Python.3.12` on Windows, `brew install python@3.12` on macOS.
2. A package step for FFmpeg, only when `ffmpeg` or `ffprobe` is not on the PATH: `winget install --id Gyan.FFmpeg` on Windows, `brew install ffmpeg` on macOS.
3. The speech venv: Python 3.12 makes a Python venv in `<user folder>/speech/venv`, then pip installs `vieneu==3.8.4` and `faster-whisper==1.2.1`.
4. The model files: horus downloads each one to `<user folder>/speech/models`, and checks its size and SHA-256.

The user folder is `HORUS_HOME`, else `~/horus`. `HORUS_SPEECH_DIR` sets
another speech folder. When the plan is right, run it:

```
horus setup vi-speech -yes
```

A package step runs only with `-with-packages`, as for an image workflow.

**What it makes.** The speech folder holds the speech venv, the model files and
`worker.py`, the program that `horus mcp` starts to make speech. horus writes
the install record `horus-install.json` into the venv before the pip install,
and again with the packages after it. So `horus uninstall` can remove the venv,
also after a pip install that did not end. horus never removes a system
package.

**After a package step.** A program that started before the install does not
find FFmpeg on its PATH. Open a new terminal, and restart the agent program.

**Check it.** `horus doctor -workflow vi-speech` checks the speech venv,
`ffmpeg`, `ffprobe` and each model file. It does not check ComfyUI, because
a speech workflow does not need it.

**If it fails.**

- `pip install ... failed`: read the lines of pip above the error, then run the command again. The plan has the venv step again until the pip install ends.
- `... is not a speech venv that horus made`: a folder that horus did not make is at the place of the venv. Inspect it, delete it, then run the command again.
- `Python 3.12 was not found`: approve the package step with `-with-packages`, or install Python 3.12 yourself.
- `ffprobe is not next to ffmpeg`: install FFmpeg yourself, so that `ffmpeg` and `ffprobe` are on the PATH.

## Step 3: give the horus tools to your agent

For Claude Code:

```
horus register
```

For Codex:

```
horus register -client codex
```

**What it makes.** An entry `horus` in the user settings of the agent (user
scope). For Claude Code it runs `claude mcp add --scope user horus -- <path of
horus> mcp`. For Codex it runs `codex mcp add horus -- <path of horus> mcp`. An
agent in any folder then has the tools `list_workflows`, `describe_workflow`,
`install_workflow`, `uninstall_workflow`, `generate_image` and
`generate_speech`.

Check it from any folder with `claude mcp get horus`: it shows `Connected`.
Then restart the agent. In Claude Code, `/mcp` shows `horus` as connected.

**Flags.** `horus register` takes three flags:

- `-client claude|codex`: the client to register the server for. Default: `claude`. Another name exits 2.
- `-root <folder>`: a clone (it must have `workflows/`) to pass as `HORUS_ROOT`. Default: the clone found from the current folder. Without a clone, horus passes no `HORUS_ROOT` and uses its built-in workflows.
- `-horus-path <path>`: the `horus` program that the agent starts. Default: the program that runs `horus register`.

When `-root` names a folder without `workflows/`, it stops with an error and
registers nothing.

**If it fails.**

- `The claude command was not found`: install Claude Code, or add the stdio server `horus` with the argument `mcp` to your agent by hand.
- `SKIP: Claude Code already knows a server named horus`: remove the old entry first with `claude mcp remove horus --scope user`, then run the command again.
- The agent shows `horus` as failed: the agent did not find the program. Run step 0 again, open a new terminal, then restart the agent.

## Step 4: make your first image

Start the agent in any folder and ask, for example: "Use it-illustration to
draw a Kubernetes cluster with three nodes, flat vector, pastel colours, no
text."

The agent calls `list_workflows`, then `describe_workflow`, and reads how to
write the subject. Then it calls `generate_image`, which waits until the PNG is
on disk and returns its path. The workflow has no style: write the look in your
request. [`workflows/it-illustration/README.md`](../workflows/it-illustration/README.md)
shows example subjects.

When a model file is missing, the agent calls `install_workflow`. That tool
asks you to accept the license first. On a machine without ComfyUI, it also
shows the setup steps and asks you first.

Without an agent, you can run the same workflow from the command line with
`horus run-illustration -subject "..."`. `horus run-illustration -h` shows the
options.

**If it fails.** Each error of the horus tools has a `code`, an `action_by` and
a `next_step`: see [the error codes](mcp.md#error-codes). The first image takes
longer, because ComfyUI starts and loads 11 GB of models. See the times in
[the table of differences](#windows-and-macos-what-differs).

## Check an existing machine at any time

```
horus doctor                                      # ComfyUI, horus on the PATH; in a clone also comfy-mcp, comfy-cli and .mcp.json
horus doctor -workflow it-illustration            # also the size and the SHA-256 of each model file
horus doctor -workflow it-illustration -quick     # also the size of each model file only
horus doctor -workflow vi-speech                  # the speech venv, ffmpeg, ffprobe and each model file; no ComfyUI check
horus setup it-illustration                       # what this machine still needs for the workflow (changes nothing)
```

`horus doctor` exits with code 0 only if no check fails. With `-workflow` it
reads all 11 GB of the model files: in the Mac test run (Apple M4) that took
5 s, and a slower disk takes longer. Without `-server-url`, it checks the
address in `HORUS_COMFYUI_URL`, then `http://127.0.0.1:8188`.

To see the workflow in the Workflows pane of the ComfyUI page, run
`horus sync-workflows`. It copies each workflow into ComfyUI as
`horus-<name>.json`. When the copy in ComfyUI differs, it prints `CONFLICT`.
Compare both, then run `horus sync-workflows -force`. The old copy is kept as
`.bak`.

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

`horus workflow install` downloads only the model files. To install ComfyUI
and the model files together, use `horus setup` (step 2).

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
class. A developer installs the pack with the comfy MCP tool `install_node`,
then restarts ComfyUI.

A file from another source (for example a LoRA from another Hugging Face
repository) gets its own `repo` and `revision`, or a full `url`, in its
`models.json` entry. The token goes only to `huggingface.co` hosts.

## Uninstall

`horus uninstall` prints what horus installed on this machine and changes nothing.
Each line starts with `REMOVE` or `KEEP`, then the path, what it is and the reason.

```
horus uninstall
```

`horus uninstall -yes` removes each `REMOVE` item. Read the plan first, because nobody can
undo the removal. The exit code is 0 when each `REMOVE` item is gone, and 1 when one is left.

```
horus uninstall -yes
```

| Part | What `-yes` removes | What it keeps |
|---|---|---|
| Model files | The model files of each workflow, by the rules of `horus workflow remove` (with `-force`). This includes a file that a person downloaded by hand. The files of a speech workflow are in the speech folder. | A file whose size differs from `models.json`. |
| ComfyUI | The install folder, with everything in it, when it has the install record `horus-install.json`. horus first stops a ComfyUI that horus started. | A folder without the record (a ComfyUI that you or an older horus installed). A ComfyUI that answers and that horus did not start. |
| comfy-cli and comfy MCP venv | The venv `~/.venvs/comfy-mcp` (of `horus setup` on macOS, or of `horus install-comfy-mcp`), when it has the install record. | A venv without the record. The plan says how to delete it by hand. |
| Speech setup | In the speech folder: the speech venv, when it has the install record of the kind `speech-venv`, and `worker.py`. Then each empty folder of the speech folder, and the speech folder itself when it is empty. Close each agent program that uses horus first: a speech worker that runs holds files of the speech venv. | A speech venv without the record (a venv that you made). |
| MCP registration | The server `horus` of Claude Code and of Codex, when the client is found and knows the server. | |
| User folder | `builtin-workflows/` and the state files of horus. | `.env` (your token) and `outputs/` (your images): they are your data. The plan names both paths. |
| Program | The PATH entry of the install script, and the program file (the last item). | A system package (7-Zip, Python, FFmpeg): horus never removes one. A PATH entry that you added by hand. |

`horus uninstall -models` limits the command to the model files. `horus uninstall -mcp`
limits it to the registration of Claude Code and Codex.

## Run horus as a stack

horus can also run as a Docker stack of 3 containers next to the ComfyUI of
this machine: `horus-db` (Postgres), `horus-worker` and `horus-api`. horus then
serves only when its 4 dependencies are up: the 3 containers and ComfyUI. The
rules: [the stack and its readiness](mcp.md#the-stack-and-its-readiness).

You need a clone of this repo (the stack builds the image of horus from it)
and Docker (Docker Desktop on Windows and macOS). ComfyUI stays on this
machine: set it up with the steps above, and start it with
`horus start-comfyui`. The stack never starts ComfyUI.

1. Put a database password in `.env` at the repo root. Choose a long random
   value. Git ignores `.env`.

   ```
   HORUS_DB_PASSWORD=<a long random value>
   ```

   `docker compose` reads `.env` too. `.env.example` lists the other settings
   of the stack: `HORUS_STACK` and `HORUS_API_PORT`. Keep no
   `HORUS_COMFYUI_URL` line in `.env`: the containers reach ComfyUI at
   `http://host.docker.internal:8188`.

   An agent that connects over HTTP (step 5) also needs the bearer token
   `HORUS_API_TOKEN` in `.env`. These commands write a random token into
   `.env` and print nothing. Run one of them one time, in the repo root:

   ```
   # Windows (PowerShell)
   $b = New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b); Add-Content .env ("HORUS_API_TOKEN=" + -join ($b | ForEach-Object { $_.ToString('x2') }))
   # macOS (Terminal)
   echo "HORUS_API_TOKEN=$(openssl rand -hex 32)" >> .env
   ```

   Without the token, `/mcp` refuses each request, and stdio still works.

2. Write the shared folders of the stack. An agent can ask for a PNG only in
   a shared folder. Without `HORUS_SHARED_DIRS`, the one shared folder is the
   output folder of horus (`.scratch/outputs` in the clone). To choose other
   folders, put pairs `<host folder>=<container folder>` in `.env`, with `;`
   between two pairs. Each container folder is one folder under `/shared/`.

   ```
   HORUS_SHARED_DIRS=C:\Users\you\Pictures\horus=/shared/pictures
   ```

   Also put the models folder in `.env`: `HORUS_MODELS_DIR` is the host
   folder of the model files that ComfyUI reads, with one subfolder for each
   kind of file (`diffusion_models`, `text_encoders`, `vae`). ComfyUI must
   read this folder: its own `ComfyUI/models` folder, or a folder that
   `extra_model_paths.yaml` of ComfyUI maps. The stack mounts it into
   `horus-api`. There, `list_workflows` reads the install state, and
   `install_workflow` and `uninstall_workflow` change only the model files
   of this folder, after the consent of the user. Without the setting,
   `install_workflow` in the stack answers `BACKEND_NOT_INSTALLED`.

   ```
   HORUS_MODELS_DIR=C:\Users\you\ComfyUI-models
   ```

   Then run, from the repo root:

   ```
   horus stack-mounts
   ```

   It writes `compose.override.yaml` next to `compose.yaml`: the shared
   folders and the models folder. Git ignores the file. After a change of
   `HORUS_SHARED_DIRS` or `HORUS_MODELS_DIR`, run it again, then
   `docker compose up -d`. On Linux, the user of the containers (uid 10001)
   must be able to write into the models folder.

3. Start the stack from the repo root. The first start builds the image
   `horus:local`.

   ```
   docker compose up -d --wait
   ```

   It starts `horus-db`, then `horus-worker`, then `horus-api`, and returns
   when the 3 containers are healthy. `horus-api` is healthy only when ComfyUI
   answers.

4. Check the readiness. Each of the 4 lines must say `ok`.

   ```
   docker exec horus-api horus ready
   ```

   A `FAIL` line names the dependency that is down and why. Start it, and run
   the command again.

5. Give the stack to your agent, over stdio or over HTTP. Remove an older
   entry first, because `horus register` skips a client that already knows a
   server named `horus`.

   ```
   claude mcp remove horus
   horus register -docker
   ```

   With `-docker`, the agent starts `docker exec -i horus-api horus mcp`
   (stdio). Or connect over HTTP:

   ```
   horus register -http
   ```

   With `-http`, the agent connects to `http://127.0.0.1:8190/mcp` (the port
   is `HORUS_API_PORT`), and sends the header
   `Authorization: Bearer <HORUS_API_TOKEN>`. The config of the agent names
   the variable `HORUS_API_TOKEN`, not the token. So give the agent the
   variable: set it in the environment, with the value of `.env`, then start
   a new agent session.

   ```
   # Windows (PowerShell): a variable of your user, from the line in .env
   $t = (Get-Content .env | Where-Object { $_ -like 'HORUS_API_TOKEN=*' }) -replace '^HORUS_API_TOKEN=', ''; [Environment]::SetEnvironmentVariable('HORUS_API_TOKEN', "$t", 'User')
   # macOS (Terminal): a line in ~/.zshrc, from the line in .env
   echo "export $(grep '^HORUS_API_TOKEN=' .env)" >> ~/.zshrc
   ```

   `-client codex` works with `-docker` and with `-http`. The agent gives and
   gets host paths: a PNG is in a shared folder of this machine. The stack
   makes images only: for speech, use `horus register` without `-docker` or
   `-http`.

6. Open the Horus UI at `http://127.0.0.1:8190/` (the port is
   `HORUS_API_PORT`). The read-only page shows the readiness, the queue, the
   running jobs, the recent results, the worker and the workflows
   ([the Horus UI](mcp.md#the-horus-ui)). Use the host name `127.0.0.1` or
   `localhost`: the page refuses another host name. The page shows the clock
   times of the time zone of your browser, and the path of each PNG on this
   machine. The stack needs no time zone setting.

`docker compose stop` stops the stack, and `docker compose up -d --wait` starts
it again. The jobs stay in the volume `horus-db-data`. When a dependency is
down, a new session does not start, and a call gets `NOT_READY`. When the
dependency is up again, horus serves again with no action on horus.

## For developers: work in a clone

Use this path only when you change the code of horus, or when you build a new
workflow ([Flow 2](style-to-workflow.md)). In a clone, the agent has two MCP
servers:

| Server | Program | What it is for |
|---|---|---|
| horus MCP server | `horus mcp` | The same tools that a user's agent gets. Test it here. |
| comfy MCP server | `horus comfy-mcp`. It runs the program `comfy-mcp` (version 0.10.0) from the venv of `horus install-comfy-mcp`, or from the PATH. | Low-level ComfyUI tools: `server_info`, `validate_workflow`, `set_workflow_slot`, `run_workflow` and more. Use them to build, check and debug a workflow. See [running workflows](running-workflows.md). |

The file `.mcp.json` of the clone starts both servers through `horus`, on
Windows and on macOS.

**Prerequisites.** Git, Go 1.27.3 or newer from [go.dev/dl](https://go.dev/dl/)
(on macOS, `brew install go`), and Python 3.12. On Windows, Python needs the
Python Launcher (`py -3.12 --version`). On macOS, `brew install python@3.12`.

### Build horus

Clone this repo, open a terminal in its root folder, then build the program:

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

### Install the comfy MCP server

```
horus install-comfy-mcp
```

**What it makes.** One Python virtual environment (venv) with `comfy-mcp` 0.10.0
and `comfy-cli` 1.25.0, in `%USERPROFILE%\.venvs\comfy-mcp` on Windows and
`~/.venvs/comfy-mcp` on macOS. On Windows, `horus` makes it with the Python
Launcher (`py -3.12`). On macOS, it makes it with `python3.12`. On macOS,
`horus setup` already made this venv. The versions are fixed on purpose, so
that every machine behaves the same.

`horus comfy-mcp` sets `PYTHONUTF8=1` and `PYTHONIOENCODING=utf-8` for
comfy-mcp. When `COMFY_BIN` is not set and a `comfy` program is next to
comfy-mcp, it sets `COMFY_BIN` to that program.

**If it fails.**

- Windows, `The Python Launcher (py.exe) was not found`: install Python 3.12 from python.org. Select "py launcher" in the installer.
- macOS, `python3.12 was not found`: run `brew install python@3.12`, then open a new terminal.
- `pip install failed`: check your network, then run the command again.

### Set up ComfyUI and the models

Do steps 1 and 2 of this page in the clone: `copy .env.example .env` (Windows)
or `cp .env.example .env` (macOS), then `horus setup it-illustration -yes`.
In a clone, horus uses the `workflows/` and the `.env` of the clone. Then run:

```
horus start-comfyui
horus sync-workflows
horus doctor -workflow it-illustration
```

`horus doctor` must show no `FAIL` line. In a clone it also checks comfy-mcp,
comfy-cli and both entries of `.mcp.json`.

### Connect the agent in the clone

```
claude
```

Claude Code reads `.mcp.json` and asks whether to use the project servers
`comfy-mcp` and `horus`. Approve both. Then:

1. Type `/mcp`. It must show `comfy-mcp` and `horus` as connected.
2. Type: `run server_info`. It must report that ComfyUI is running at `http://127.0.0.1:8188`.

**If it fails.**

- `horus` failed: Claude Code did not find `horus` on the PATH. Build it again, open a new terminal, then restart `claude`. `horus doctor` shows the line `horus on PATH`.
- `comfy-mcp` failed (`CONNECTION_CLOSED`): `horus` is not on the PATH, or the venv is missing. Then `horus comfy-mcp` writes `comfy-mcp was not found in ... Fix: horus install-comfy-mcp` and exits 1. Run `horus install-comfy-mcp`, then restart `claude`. `horus doctor` shows the lines `comfy-mcp environment` and `.mcp.json`.
- `Pending approval` in `claude mcp list`: approve the server in an interactive `claude` session. To be asked again, run `claude mcp reset-project-choices`.
- `server_info` shows `running: false`: run `horus start-comfyui`.

`horus register` in a clone passes `HORUS_ROOT=<clone>`, so an agent in another
folder uses the workflows of the clone.

**Older command names.** `horus check-setup` is the older name of `horus doctor`.
`horus install-comfyui` and `horus workflow install` are the two halves of
`horus setup -yes`. They still work and do the same job. Use `doctor` and `setup`.
