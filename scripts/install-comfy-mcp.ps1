<#
.SYNOPSIS
Install the comfy MCP server and comfy-cli in their own Python environment.

.DESCRIPTION
Creates a Python 3.12 virtual environment (venv) and installs two packages with
pinned versions:
  comfy-mcp  0.10.0   the MCP server that an AI agent talks to
  comfy-cli  1.25.0   the command-line tool that comfy-mcp calls

The default venv path is "$env:USERPROFILE\.venvs\comfy-mcp". The repo file
.mcp.json points at this path, so keep the default unless you also edit .mcp.json.
The script is safe to run again. If the pinned versions are already installed,
it changes nothing.

.PARAMETER VenvPath
Folder of the virtual environment.

.PARAMETER PythonVersion
Python version for the Python Launcher (py.exe). Default: 3.12.

.EXAMPLE
.\scripts\install-comfy-mcp.ps1
#>
[CmdletBinding()]
param(
    [string] $VenvPath = '',
    [string] $PythonVersion = '3.12'
)

$ErrorActionPreference = 'Stop'
$mcpVersion = '0.10.0'
$cliVersion = '1.25.0'

if (-not $VenvPath) { $VenvPath = Join-Path $env:USERPROFILE '.venvs\comfy-mcp' }
$python = Join-Path $VenvPath 'Scripts\python.exe'

if (-not (Test-Path -LiteralPath $python -PathType Leaf)) {
    if (Test-Path -LiteralPath $VenvPath) {
        throw "$VenvPath exists but is not a usable venv. Inspect it, then delete it or pass another -VenvPath."
    }
    $launcher = Get-Command py.exe -ErrorAction SilentlyContinue
    if (-not $launcher) { throw "The Python Launcher (py.exe) was not found. Install Python $PythonVersion from python.org." }
    $ErrorActionPreference = 'Continue'   # Windows PowerShell 5.1 treats native stderr as an error
    & $launcher.Source "-$PythonVersion" '-c' 'import sys' 2>$null
    $launcherExit = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    if ($launcherExit -ne 0) { throw "Python $PythonVersion was not found. Install it from python.org (check 'py launcher')." }
    New-Item -ItemType Directory -Path (Split-Path -Parent $VenvPath) -Force | Out-Null
    Write-Output "Creating the venv at $VenvPath ..."
    & $launcher.Source "-$PythonVersion" '-m' 'venv' $VenvPath
    if ($LASTEXITCODE -ne 0) { throw "Creating the venv failed (exit code $LASTEXITCODE)." }
}

$ErrorActionPreference = 'Continue'
$lines = @(& $python '-c' "import importlib.metadata as m; print(m.version('comfy-mcp')); print(m.version('comfy-cli'))" 2>$null)
$queryExit = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
$pinned = ($queryExit -eq 0 -and $lines.Count -eq 2 -and $lines[0].Trim() -eq $mcpVersion -and $lines[1].Trim() -eq $cliVersion)
if ($pinned) {
    Write-Output "SKIP: comfy-mcp $mcpVersion and comfy-cli $cliVersion are already installed."
} else {
    Write-Output "Installing comfy-mcp==$mcpVersion and comfy-cli==$cliVersion ..."
    & $python '-m' 'pip' 'install' '--disable-pip-version-check' "comfy-mcp==$mcpVersion" "comfy-cli==$cliVersion"
    if ($LASTEXITCODE -ne 0) { throw "pip install failed (exit code $LASTEXITCODE)." }
}

foreach ($exe in @('comfy-mcp.exe', 'comfy.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $VenvPath "Scripts\$exe") -PathType Leaf)) {
        throw "Expected program is missing: $(Join-Path $VenvPath "Scripts\$exe")"
    }
}
Write-Output "OK: comfy-mcp is installed in $VenvPath"
Write-Output 'Next: connect your agent (docs/setup.md, step 5).'
