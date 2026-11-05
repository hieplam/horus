<#
.SYNOPSIS
One health check for the whole horus setup.

.DESCRIPTION
Prints one PASS or FAIL line for each check. Exit code 0 only if all checks pass.

Checks:
  1. ComfyUI is installed in -ComfyUIDir.
  2. The ComfyUI server answers at -ServerUrl.
  3. The comfy-mcp environment exists (comfy-mcp.exe and comfy.exe).
  4. The command in .mcp.json points at that environment.
  5. With -Workflow: the model files of that workflow are present and verified
     (runs install-workflow-models.ps1 -CheckOnly; hashing 11 GB takes minutes).

.PARAMETER ComfyUIDir
The portable folder. Default: HORUS_COMFYUI_DIR, then
"$env:USERPROFILE\ComfyUI_windows_portable".

.PARAMETER Workflow
Also check the model files of workflows\<Workflow>\models.json.

.PARAMETER TextEncoderDir
Passed to install-workflow-models.ps1 (text encoder on another drive).

.PARAMETER ModelsDir
Passed to install-workflow-models.ps1.

.PARAMETER ModelsJson
Passed to install-workflow-models.ps1 (use another model list).

.EXAMPLE
.\scripts\check-setup.ps1

.EXAMPLE
.\scripts\check-setup.ps1 -Workflow it-illustration
#>
[CmdletBinding()]
param(
    [string] $ComfyUIDir = '',
    [string] $ServerUrl = 'http://127.0.0.1:8188',
    [string] $VenvPath = '',
    [string] $Workflow = '',
    [string] $ModelsDir = '',
    [string] $TextEncoderDir = '',
    [string] $ModelsJson = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$script:failed = 0

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

function Write-Check([bool] $Ok, [string] $Name, [string] $Detail) {
    if ($Ok) { Write-Output "PASS  $Name - $Detail" }
    else { Write-Output "FAIL  $Name - $Detail"; $script:failed++ }
}

if (-not $ComfyUIDir) {
    $ComfyUIDir = Get-HorusSetting 'HORUS_COMFYUI_DIR' (Join-Path $env:USERPROFILE 'ComfyUI_windows_portable')
}
if (-not $VenvPath) { $VenvPath = Join-Path $env:USERPROFILE '.venvs\comfy-mcp' }

# 1. ComfyUI installed.
$mainPy = Join-Path $ComfyUIDir 'ComfyUI\main.py'
if (Test-Path -LiteralPath $mainPy -PathType Leaf) {
    Write-Check $true 'ComfyUI folder' $ComfyUIDir
} else {
    Write-Check $false 'ComfyUI folder' "ComfyUI\main.py not found in $ComfyUIDir. Fix: .\scripts\install-comfyui.ps1, or pass -ComfyUIDir."
}

# 2. Server answers.
try {
    $stats = Invoke-RestMethod -Uri "$ServerUrl/system_stats" -TimeoutSec 5
    $version = $stats.system.comfyui_version
    Write-Check $true 'ComfyUI server' "answers at $ServerUrl (ComfyUI $version)"
} catch {
    Write-Check $false 'ComfyUI server' "no answer at $ServerUrl. Fix: .\scripts\start-comfyui.ps1"
}

# 3. comfy-mcp environment.
$mcpExe = Join-Path $VenvPath 'Scripts\comfy-mcp.exe'
$cliExe = Join-Path $VenvPath 'Scripts\comfy.exe'
$venvOk = (Test-Path -LiteralPath $mcpExe -PathType Leaf) -and (Test-Path -LiteralPath $cliExe -PathType Leaf)
if ($venvOk) { Write-Check $true 'comfy-mcp environment' $VenvPath }
else { Write-Check $false 'comfy-mcp environment' "comfy-mcp.exe or comfy.exe missing in $VenvPath. Fix: .\scripts\install-comfy-mcp.ps1" }

# 4. .mcp.json matches the environment.
$mcpJson = Join-Path $repoRoot '.mcp.json'
try {
    $config = Get-Content -LiteralPath $mcpJson -Raw | ConvertFrom-Json
    $server = $config.mcpServers.'comfy-mcp'
    $command = ([string] $server.command).Replace('${USERPROFILE}', $env:USERPROFILE)
    $cliInJson = ([string] $server.env.COMFY_BIN).Replace('${USERPROFILE}', $env:USERPROFILE)
    $commandOk = ($command -ieq $mcpExe) -and (Test-Path -LiteralPath $command -PathType Leaf)
    $cliOk = ($cliInJson -ieq $cliExe) -and (Test-Path -LiteralPath $cliInJson -PathType Leaf)
    if ($commandOk -and $cliOk) {
        Write-Check $true '.mcp.json' "command and COMFY_BIN point at $VenvPath"
    } else {
        Write-Check $false '.mcp.json' "command is '$command' and COMFY_BIN is '$cliInJson'; expected '$mcpExe' and '$cliExe'. Fix: install the venv at the default path, or edit .mcp.json."
    }
} catch {
    Write-Check $false '.mcp.json' "cannot read comfy-mcp entry in $mcpJson : $($_.Exception.Message)"
}

# 5. Models of one workflow.
if ($Workflow) {
    $params = @{ Workflow = $Workflow; ComfyUIDir = $ComfyUIDir; CheckOnly = $true }
    if ($ModelsDir) { $params.ModelsDir = $ModelsDir }
    if ($TextEncoderDir) { $params.TextEncoderDir = $TextEncoderDir }
    if ($ModelsJson) { $params.ModelsJson = $ModelsJson }
    $lines = @(& (Join-Path $PSScriptRoot 'install-workflow-models.ps1') @params)
    $exit = $LASTEXITCODE
    foreach ($l in $lines) { Write-Output "      $l" }
    Write-Check ($exit -eq 0) "models for '$Workflow'" $(if ($exit -eq 0) { 'all files present and verified' } else { "see lines above. Fix: .\scripts\install-workflow-models.ps1 -Workflow $Workflow" })
}

if ($script:failed -gt 0) {
    Write-Output "$($script:failed) check(s) failed."
    exit 1
}
Write-Output 'All checks passed.'
exit 0
