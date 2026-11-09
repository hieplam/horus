<#
.SYNOPSIS
Download and verify the model files that one workflow needs.

.DESCRIPTION
This script calls workflow.ps1 (install, or check with -CheckOnly). Use
workflow.ps1 directly for the newer actions: list, remove, and -Only.

Reads workflows\<Workflow>\models.json. For each file in the list, the script
calls download-model.ps1. That script skips a file that is already correct and
refuses to overwrite a file that is wrong.

models.json has this shape:
  { "repo": "...", "revision": "...", "license": "...",
    "files": [ { "role": "...", "repo_path": "...", "folder": "diffusion_models",
                 "bytes": 123, "sha256": "..." } ] }
Download address: https://huggingface.co/<repo>/resolve/<revision>/<repo_path>

Where each file goes:
  - <ModelsDir>\<folder>\<file name>, where ModelsDir is, in this order: the
    -ModelsDir parameter, the HORUS_MODELS_DIR setting, then
    <ComfyUIDir>\ComfyUI\models.
  - For files in the "text_encoders" folder, -TextEncoderDir (or the
    HORUS_TEXT_ENCODER_DIR setting) replaces the folder above.
Big models can live on another drive. When a folder is not inside ComfyUI's own
models folder, the script writes it to ComfyUI's extra_model_paths.yaml (a
backup is made first). Restart ComfyUI after a change to that file.

-CheckOnly changes nothing and downloads nothing. It checks that each file is
present and that its size and SHA-256 hash are right. Hashing 11 GB takes a few
minutes.

Exit code 0 = every file is present and verified. Exit code 1 = anything else.

.PARAMETER Workflow
Folder name under workflows\ (for example it-illustration).

.PARAMETER ComfyUIDir
The portable folder. Default: HORUS_COMFYUI_DIR, then
"$env:USERPROFILE\ComfyUI_windows_portable".

.PARAMETER ModelsDir
Base folder that holds the model subfolders. Default: see above.

.PARAMETER TextEncoderDir
Full path of the text_encoders folder, when it is not under ModelsDir.

.PARAMETER ModelsJson
Use this file instead of workflows\<Workflow>\models.json.

.PARAMETER CheckOnly
Verify only.

.EXAMPLE
.\scripts\install-workflow-models.ps1 -Workflow it-illustration

.EXAMPLE
.\scripts\install-workflow-models.ps1 -Workflow it-illustration -TextEncoderDir 'E:\AI\text_encoders' -CheckOnly
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Workflow,
    [string] $ComfyUIDir = '',
    [string] $ModelsDir = '',
    [string] $TextEncoderDir = '',
    [string] $ModelsJson = '',
    [switch] $CheckOnly
)

$ErrorActionPreference = 'Stop'

# This script is kept for its old command line. workflow.ps1 does the work.
$params = @{ Action = $(if ($CheckOnly) { 'check' } else { 'install' }); Workflow = $Workflow }
if ($ComfyUIDir) { $params.ComfyUIDir = $ComfyUIDir }
if ($ModelsDir) { $params.ModelsDir = $ModelsDir }
if ($TextEncoderDir) { $params.TextEncoderDir = $TextEncoderDir }
if ($ModelsJson) { $params.ModelsJson = $ModelsJson }
& (Join-Path $PSScriptRoot 'workflow.ps1') @params
exit $LASTEXITCODE
