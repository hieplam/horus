<#
.SYNOPSIS
Start ComfyUI (portable build) and wait until its server answers.

.DESCRIPTION
Starts the embedded Python of the portable build with ComfyUI's main.py. The
server listens on 127.0.0.1 only, so other computers cannot reach it.
If the server already answers, the script prints a message and exits 0.
ComfyUI writes its console output to two log files in -ComfyUIDir:
horus-comfyui.out.log and horus-comfyui.err.log.

-ComfyUIDir comes from, in this order: the parameter, the HORUS_COMFYUI_DIR
setting (environment or .env file), then "$env:USERPROFILE\ComfyUI_windows_portable".

.PARAMETER ComfyUIDir
The portable folder (the one that contains ComfyUI\ and python_embeded\).

.PARAMETER ServerUrl
Address that the server must answer on. The port is taken from this address.

.PARAMETER TimeoutSeconds
How long to wait for the server. The first start can take a few minutes.

.EXAMPLE
.\scripts\start-comfyui.ps1
#>
[CmdletBinding()]
param(
    [string] $ComfyUIDir = '',
    [string] $ServerUrl = 'http://127.0.0.1:8188',
    [int] $TimeoutSeconds = 300
)

$ErrorActionPreference = 'Stop'

function Get-HorusSetting([string] $Name, [string] $Default = '') {
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ($value) { return $value }
    $envFile = Join-Path (Split-Path -Parent $PSScriptRoot) '.env'
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
$python = Join-Path $ComfyUIDir 'python_embeded\python.exe'
$mainPy = Join-Path $ComfyUIDir 'ComfyUI\main.py'
if (-not (Test-Path -LiteralPath $python -PathType Leaf) -or -not (Test-Path -LiteralPath $mainPy -PathType Leaf)) {
    throw "ComfyUI was not found in $ComfyUIDir. Run .\scripts\install-comfyui.ps1 first, or pass -ComfyUIDir."
}

function Test-Server {
    try {
        $null = Invoke-RestMethod -Uri "$ServerUrl/system_stats" -TimeoutSec 3
        return $true
    } catch {
        return $false
    }
}

if (Test-Server) {
    Write-Output "ComfyUI already answers at $ServerUrl"
    exit 0
}

$port = ([uri] $ServerUrl).Port
$outLog = Join-Path $ComfyUIDir 'horus-comfyui.out.log'
$errLog = Join-Path $ComfyUIDir 'horus-comfyui.err.log'
$process = Start-Process -FilePath $python -WorkingDirectory $ComfyUIDir -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput $outLog -RedirectStandardError $errLog `
    -ArgumentList @('-s', 'ComfyUI\main.py', '--windows-standalone-build', '--listen', '127.0.0.1', '--port', "$port")
Write-Output "Started ComfyUI (process $($process.Id)). Waiting for $ServerUrl ..."

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 2
    if (Test-Server) {
        Write-Output "OK: ComfyUI answers at $ServerUrl"
        exit 0
    }
    if ($process.HasExited) {
        throw "ComfyUI stopped with exit code $($process.ExitCode). Read the log: $errLog"
    }
}
throw "ComfyUI did not answer within $TimeoutSeconds seconds. Read the logs: $outLog and $errLog"
