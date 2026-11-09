<#
.SYNOPSIS
Install the horus MCP server in its own Python environment, and optionally register it for Claude Code.

.DESCRIPTION
Creates a Python 3.12 virtual environment (venv) and installs this repo into it
in editable mode, with the pinned MCP SDK (mcp 2.3.0, from pyproject.toml).
"Editable" means that the venv runs the code in this folder, so a git pull
updates the server without a new install.

The server program is <VenvPath>\Scripts\horus-mcp.exe. It speaks MCP over
stdio. The repo file .mcp.json points at the default path, so an agent that
runs in this repo finds the server.

-Register also adds the server to your Claude Code user settings
(claude mcp add --scope user), so agents in every other folder on this PC can
use it. The script is safe to run again.

.PARAMETER VenvPath
Folder of the virtual environment. Default: "$env:USERPROFILE\.venvs\horus-mcp".

.PARAMETER PythonVersion
Python version for the Python Launcher (py.exe). Default: 3.12.

.PARAMETER Register
Also register the server for Claude Code in user scope.

.EXAMPLE
.\scripts\install-horus-mcp.ps1 -Register
#>
[CmdletBinding()]
param(
    [string] $VenvPath = '',
    [string] $PythonVersion = '3.12',
    [switch] $Register
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $VenvPath) { $VenvPath = Join-Path $env:USERPROFILE '.venvs\horus-mcp' }
$python = Join-Path $VenvPath 'Scripts\python.exe'
$exe = Join-Path $VenvPath 'Scripts\horus-mcp.exe'

if (-not (Test-Path -LiteralPath $python -PathType Leaf)) {
    if (Test-Path -LiteralPath $VenvPath) {
        throw "$VenvPath exists but is not a usable venv. Inspect it, then delete it or pass another -VenvPath."
    }
    $launcher = Get-Command py.exe -ErrorAction SilentlyContinue
    if (-not $launcher) { throw "The Python Launcher (py.exe) was not found. Install Python $PythonVersion from python.org." }
    New-Item -ItemType Directory -Path (Split-Path -Parent $VenvPath) -Force | Out-Null
    Write-Output "Creating the venv at $VenvPath ..."
    & $launcher.Source "-$PythonVersion" '-m' 'venv' $VenvPath
    if ($LASTEXITCODE -ne 0) { throw "Creating the venv failed (exit code $LASTEXITCODE)." }
}

Write-Output "Installing horus (editable) and its pinned dependencies ..."
& $python '-m' 'pip' 'install' '--disable-pip-version-check' '--quiet' '--editable' $repoRoot
if ($LASTEXITCODE -ne 0) { throw "pip install failed (exit code $LASTEXITCODE)." }
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw "Expected program is missing: $exe" }

$ErrorActionPreference = 'Continue'
# Single quotes inside: Windows PowerShell 5.1 drops inner double quotes on a native command line.
$version = & $python '-c' "import importlib.metadata as m; print(m.version('mcp'))" 2>$null
$ErrorActionPreference = 'Stop'
if ($LASTEXITCODE -ne 0 -or -not $version) { throw "The venv has no mcp package. Run the script again." }
Write-Output "OK: horus-mcp is installed in $VenvPath (mcp $version)."

if ($Register) {
    $claude = Get-Command claude -ErrorAction SilentlyContinue
    if (-not $claude) { throw 'The claude command was not found. Install Claude Code, or add the server by hand (docs/mcp.md).' }
    # Ask from a folder outside the repo, so the project entry in .mcp.json does not count.
    $ErrorActionPreference = 'Continue'
    Push-Location $env:TEMP
    try { $null = & $claude.Source mcp get horus 2>$null; $known = ($LASTEXITCODE -eq 0) } finally { Pop-Location }
    $ErrorActionPreference = 'Stop'
    if ($known) {
        Write-Output 'SKIP: Claude Code already knows a server named horus (claude mcp get horus).'
    } else {
        & $claude.Source mcp add --scope user horus -- $exe
        if ($LASTEXITCODE -ne 0) { throw "claude mcp add failed (exit code $LASTEXITCODE)." }
        Write-Output 'OK: registered for Claude Code in user scope. Start a new Claude Code session to see the tools.'
    }
}
Write-Output 'Test: run the contract test with the venv Python: tests\mcp_contract.py (docs/mcp.md).'
