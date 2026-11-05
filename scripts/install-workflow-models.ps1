<#
.SYNOPSIS
Download and verify the model files that one workflow needs.

.DESCRIPTION
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
$repoRoot = Split-Path -Parent $PSScriptRoot

function Get-HorusSetting([string] $Name, [string] $Default = '') {
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ($value) { return $value }
    $envFile = Join-Path $repoRoot '.env'
    if (Test-Path -LiteralPath $envFile -PathType Leaf) {
        $line = Get-Content -LiteralPath $envFile |
            Where-Object { $_ -match ('^\s*' + [regex]::Escape($Name) + '\s*=') } | Select-Object -First 1
        if ($line) {
            $value = ($line -replace ('^\s*' + [regex]::Escape($Name) + '\s*=\s*'), '').Trim().Trim('"').Trim("'")
            if ($value) { return $value }
        }
    }
    return $Default
}

function Get-FullPath([string] $Path) { return [IO.Path]::GetFullPath($Path).TrimEnd('\') }

try {
    if ($Workflow -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "Invalid workflow name: $Workflow" }
    if (-not $ModelsJson) { $ModelsJson = Join-Path $repoRoot "workflows\$Workflow\models.json" }
    if (-not (Test-Path -LiteralPath $ModelsJson -PathType Leaf)) { throw "Model list not found: $ModelsJson" }
    $manifest = Get-Content -LiteralPath $ModelsJson -Raw | ConvertFrom-Json
    if (-not $manifest.files -or @($manifest.files).Count -eq 0) { throw "No files listed in $ModelsJson" }

    if (-not $ComfyUIDir) {
        $ComfyUIDir = Get-HorusSetting 'HORUS_COMFYUI_DIR' (Join-Path $env:USERPROFILE 'ComfyUI_windows_portable')
    }
    $comfyDir = Join-Path $ComfyUIDir 'ComfyUI'
    $defaultModelsRoot = Join-Path $comfyDir 'models'
    if (-not $ModelsDir) { $ModelsDir = Get-HorusSetting 'HORUS_MODELS_DIR' $defaultModelsRoot }
    if (-not $TextEncoderDir) { $TextEncoderDir = Get-HorusSetting 'HORUS_TEXT_ENCODER_DIR' '' }
    if (-not $CheckOnly -and -not (Test-Path -LiteralPath (Join-Path $comfyDir 'main.py') -PathType Leaf)) {
        throw "ComfyUI was not found in $ComfyUIDir. Run .\scripts\install-comfyui.ps1 first, or pass -ComfyUIDir."
    }

    # Build the target list.
    $targets = @()
    foreach ($f in @($manifest.files)) {
        $dir = Join-Path $ModelsDir $f.folder
        if ($f.folder -eq 'text_encoders' -and $TextEncoderDir) { $dir = $TextEncoderDir }
        $targets += [pscustomobject]@{
            Role = $f.role; Folder = $f.folder; Dir = $dir
            Dest = Join-Path $dir (Split-Path -Leaf $f.repo_path)
            Bytes = [long] $f.bytes; Sha256 = ([string] $f.sha256).ToLowerInvariant()
            Url = "https://huggingface.co/$($manifest.repo)/resolve/$($manifest.revision)/$($f.repo_path)"
        }
    }

    $failures = 0
    if ($CheckOnly) {
        Write-Output "Checking $($targets.Count) model file(s) for workflow '$Workflow' (no download, no change)."
        foreach ($t in $targets) {
            if (-not (Test-Path -LiteralPath $t.Dest -PathType Leaf)) {
                Write-Output "MISSING   $($t.Role): $($t.Dest)"; $failures++; continue
            }
            $length = (Get-Item -LiteralPath $t.Dest).Length
            if ($length -ne $t.Bytes) {
                Write-Output "BAD SIZE  $($t.Role): $($t.Dest) has $length bytes, expected $($t.Bytes)"; $failures++; continue
            }
            $got = (Get-FileHash -LiteralPath $t.Dest -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($got -ne $t.Sha256) {
                Write-Output "BAD HASH  $($t.Role): $($t.Dest) SHA-256 is $got, expected $($t.Sha256)"; $failures++; continue
            }
            Write-Output "OK        $($t.Role): $($t.Dest) ($length bytes, SHA-256 verified)"
        }
    } else {
        Write-Output "License: $($manifest.license)"
        Write-Output "Installing $($targets.Count) model file(s) for workflow '$Workflow'."
        foreach ($t in $targets) {
            Write-Output "-- $($t.Role) -> $($t.Dest)"
            & (Join-Path $PSScriptRoot 'download-model.ps1') -Url $t.Url -Destination $t.Dest `
                -ExpectedBytes $t.Bytes -Sha256 $t.Sha256 -TokenEnvVar 'HF_TOKEN'
            if ($LASTEXITCODE -ne 0) { $failures++ }
        }

        # Map folders that are outside ComfyUI's own models folder.
        $yamlPath = Join-Path $comfyDir 'extra_model_paths.yaml'
        $yaml = ''
        if (Test-Path -LiteralPath $yamlPath -PathType Leaf) { $yaml = Get-Content -LiteralPath $yamlPath -Raw }
        $changed = $false
        $backedUp = $false
        $seen = @{}
        foreach ($t in $targets) {
            if ($seen.ContainsKey($t.Dir)) { continue }
            $seen[$t.Dir] = $true
            if ((Get-FullPath $t.Dir) -ieq (Get-FullPath (Join-Path $defaultModelsRoot $t.Folder))) { continue }
            $full = Get-FullPath $t.Dir
            $base = (Split-Path -Parent $full) -replace '\\', '/'
            $leaf = Split-Path -Leaf $full
            $hasBase = $yaml -match [regex]::Escape("base_path: $base")
            $hasLine = $yaml -match ('(?m)^\s+' + [regex]::Escape($t.Folder) + ':\s*' + [regex]::Escape($leaf) + '\s*$')
            if ($hasBase -and $hasLine) { Write-Output "extra_model_paths.yaml already maps $($t.Folder) -> $base/$leaf"; continue }
            if ((Test-Path -LiteralPath $yamlPath) -and -not $backedUp) {
                Copy-Item -LiteralPath $yamlPath -Destination ("$yamlPath.bak-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
                $backedUp = $true
            }
            $yaml = ($yaml.TrimEnd() + "`n`nhorus_$($t.Folder):`n    base_path: $base`n    $($t.Folder): $leaf`n").TrimStart()
            $changed = $true
            Write-Output "extra_model_paths.yaml: added $($t.Folder) -> $base/$leaf"
        }
        if ($changed) {
            [IO.File]::WriteAllText($yamlPath, $yaml, (New-Object System.Text.UTF8Encoding($false)))
            Write-Output 'Restart ComfyUI so that it reads the new paths (.\scripts\start-comfyui.ps1).'
        }
    }

    if ($failures -gt 0) { throw "$failures of $($targets.Count) file(s) are not ready." }
    Write-Output "OK: all $($targets.Count) file(s) are present and verified."
    exit 0
} catch {
    Write-Output "FAIL: $($_.Exception.Message)"
    exit 1
}
