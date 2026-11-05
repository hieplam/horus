<#
.SYNOPSIS
Copy the repo workflows into ComfyUI's Workflows pane.

.DESCRIPTION
For each workflows\<name>\workflow.json, the script writes
<ComfyUIDir>\ComfyUI\user\default\workflows\horus-<name>.json.
Open the Workflows pane in the ComfyUI browser page (press w) and refresh it to
see the files. The repo copy is the master. If a pane copy differs (for example
you saved a change in the ComfyUI page), the script reports a CONFLICT and
leaves it alone. Use -Force to replace it. The script then keeps the old pane
copy as horus-<name>.json.bak.

.PARAMETER ComfyUIDir
The portable folder. Default: HORUS_COMFYUI_DIR, then
"$env:USERPROFILE\ComfyUI_windows_portable".

.PARAMETER Force
Replace a pane copy that differs from the repo copy.

.EXAMPLE
.\scripts\sync-workflows.ps1
#>
[CmdletBinding()]
param(
    [string] $ComfyUIDir = '',
    [switch] $Force
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

if (-not $ComfyUIDir) {
    $ComfyUIDir = Get-HorusSetting 'HORUS_COMFYUI_DIR' (Join-Path $env:USERPROFILE 'ComfyUI_windows_portable')
}
if (-not (Test-Path -LiteralPath (Join-Path $ComfyUIDir 'ComfyUI\main.py') -PathType Leaf)) {
    throw "ComfyUI was not found in $ComfyUIDir. Run .\scripts\install-comfyui.ps1 first, or pass -ComfyUIDir."
}
$target = Join-Path $ComfyUIDir 'ComfyUI\user\default\workflows'
$sourceRoot = Join-Path $repoRoot 'workflows'
if (-not (Test-Path -LiteralPath $sourceRoot -PathType Container)) { throw "No workflows folder in the repo: $sourceRoot" }
New-Item -ItemType Directory -Path $target -Force | Out-Null

$conflicts = 0
$count = 0
foreach ($dir in (Get-ChildItem -LiteralPath $sourceRoot -Directory)) {
    $source = Join-Path $dir.FullName 'workflow.json'
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { continue }
    $count++
    $destination = Join-Path $target "horus-$($dir.Name).json"
    if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
        Copy-Item -LiteralPath $source -Destination $destination
        Write-Output "copied   horus-$($dir.Name).json"
        continue
    }
    $same = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -eq (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash
    if ($same) {
        Write-Output "ok       horus-$($dir.Name).json"
    } elseif ($Force) {
        Copy-Item -LiteralPath $destination -Destination "$destination.bak" -Force
        Copy-Item -LiteralPath $source -Destination $destination -Force
        Write-Output "replaced horus-$($dir.Name).json (old copy kept as .bak)"
    } else {
        $conflicts++
        Write-Output "CONFLICT horus-$($dir.Name).json differs from workflows\$($dir.Name)\workflow.json. Compare both, then run again with -Force to use the repo copy."
    }
}
if ($count -eq 0) { Write-Output 'No workflows\*\workflow.json files found. Nothing to sync.' }
if ($conflicts -gt 0) { exit 1 }
exit 0
