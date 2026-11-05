<#
.SYNOPSIS
Create workflows/<Name>/README.md from workflows/_template/README.md.

.EXAMPLE
.\scripts\new-workflow.ps1 -Name postcard-watercolor -Description "Soft watercolor landscape postcards" -ModelFamily "FLUX 3 Image Klein"

Exit code 0 on success. Exit code 1 if the name is not kebab-case, the template
is missing, or the workflow folder already exists.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Name,
    [Parameter(Mandatory = $true)] [string] $Description,
    [string] $ModelFamily = 'TBD'
)

$ErrorActionPreference = 'Stop'

if ($Name -cnotmatch '^[a-z0-9]+(-[a-z0-9]+)*$') {
    [Console]::Error.WriteLine("Name must be kebab-case (lowercase letters, digits, single hyphens): '$Name'")
    exit 1
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$template = Join-Path $repoRoot 'workflows\_template\README.md'
$target = Join-Path $repoRoot ('workflows\' + $Name)

if (-not (Test-Path -LiteralPath $template)) {
    [Console]::Error.WriteLine("Template not found: $template")
    exit 1
}
if (Test-Path -LiteralPath $target) {
    [Console]::Error.WriteLine("Workflow folder already exists. Not overwriting: $target")
    exit 1
}

$text = [IO.File]::ReadAllText($template, [Text.Encoding]::UTF8)
$text = $text.Replace('{{NAME}}', $Name).Replace('{{DESCRIPTION}}', $Description).Replace('{{MODEL_FAMILY}}', $ModelFamily)

New-Item -ItemType Directory -Path $target | Out-Null
[IO.File]::WriteAllText((Join-Path $target 'README.md'), $text, (New-Object Text.UTF8Encoding($false)))

Write-Output "Created $target\README.md"
Write-Output 'Next: add workflow.json through the ComfyUI UI export or a template. Do not hand-edit the graph.'
exit 0
