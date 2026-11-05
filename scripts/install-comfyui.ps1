<#
.SYNOPSIS
Install the official ComfyUI Windows portable build (NVIDIA, CUDA 12.6).

.DESCRIPTION
Steps:
  1. Skips with a message if ComfyUI is already in -InstallDir.
  2. Checks the free disk space, 7-Zip and curl.exe.
  3. Downloads the official archive from the ComfyUI GitHub release.
  4. Checks the archive size and SHA-256 hash.
  5. Unpacks the archive and moves it to -InstallDir.
  6. Deletes the archive (use -KeepArchive to keep it).

The portable build contains its own Python. It does not change your system Python.

-InstallDir comes from, in this order: the parameter, the HORUS_COMFYUI_DIR
setting (environment or .env file), then "$env:USERPROFILE\ComfyUI_windows_portable".
The folder can be on any drive. Pick a drive with about 9 GB free.

.PARAMETER InstallDir
Target folder. It will contain ComfyUI\, python_embeded\ and run_nvidia_gpu.bat.

.PARAMETER ArchivePath
Where to keep the downloaded .7z file during the install. Default: next to -InstallDir.

.PARAMETER SevenZip
Full path to 7z.exe. Default: search PATH, then "Program Files\7-Zip".

.PARAMETER KeepArchive
Do not delete the archive after a successful install.

.EXAMPLE
.\scripts\install-comfyui.ps1

.EXAMPLE
.\scripts\install-comfyui.ps1 -InstallDir 'E:\AI\ComfyUI_windows_portable'
#>
[CmdletBinding()]
param(
    [string] $InstallDir = '',
    [string] $ArchivePath = '',
    [string] $SevenZip = '',
    [switch] $KeepArchive
)

$ErrorActionPreference = 'Stop'

# Official release asset. The size and hash come from the GitHub release page.
$version = 'v0.44.0'
$assetName = 'ComfyUI_windows_portable_nvidia_cu126.7z'
$url = "https://github.com/Comfy-Org/ComfyUI/releases/download/$version/$assetName"
$expectedBytes = 1945636677L
$expectedSha256 = 'a26cf84f043b3d9fbf0d53296f57d7190482c339caf1b3b5cfda5149d454be9f'
$expandedBytes = 5681590674L
$spareBytes = 1GB

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

if (-not $InstallDir) {
    $InstallDir = Get-HorusSetting 'HORUS_COMFYUI_DIR' (Join-Path $env:USERPROFILE 'ComfyUI_windows_portable')
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir).TrimEnd('\')
$parentDir = Split-Path -Parent $InstallDir
if (-not $ArchivePath) { $ArchivePath = Join-Path $parentDir "$assetName" }
$ArchivePath = [IO.Path]::GetFullPath($ArchivePath)
$partPath = "$ArchivePath.part"

# 1. Already installed?
$mainPy = Join-Path $InstallDir 'ComfyUI\main.py'
if (Test-Path -LiteralPath $mainPy -PathType Leaf) {
    Write-Output "SKIP: ComfyUI is already installed in $InstallDir"
    exit 0
}
if ((Test-Path -LiteralPath $InstallDir) -and (Get-ChildItem -LiteralPath $InstallDir -Force | Select-Object -First 1)) {
    throw "$InstallDir exists, is not empty, and holds no ComfyUI. Choose another -InstallDir or empty this folder."
}

# 2. Tools and disk space.
if (-not $SevenZip) {
    $found = Get-Command 7z.exe -ErrorAction SilentlyContinue
    if ($found) { $SevenZip = $found.Source }
    else {
        foreach ($candidate in @((Join-Path $env:ProgramFiles '7-Zip\7z.exe'), (Join-Path ${env:ProgramFiles(x86)} '7-Zip\7z.exe'))) {
            if ($candidate -and (Test-Path -LiteralPath $candidate -PathType Leaf)) { $SevenZip = $candidate; break }
        }
    }
}
if (-not $SevenZip -or -not (Test-Path -LiteralPath $SevenZip -PathType Leaf)) {
    throw '7-Zip was not found. Install it (winget install 7zip.7zip) or pass -SevenZip with the full path to 7z.exe.'
}
$curl = Get-Command curl.exe -ErrorAction SilentlyContinue
if (-not $curl) { throw 'curl.exe was not found. It ships with Windows 10 and 11.' }

New-Item -ItemType Directory -Path $parentDir -Force | Out-Null
$need = $expectedBytes + $expandedBytes + $spareBytes
$drive = New-Object System.IO.DriveInfo([IO.Path]::GetPathRoot($InstallDir))
if ($drive.AvailableFreeSpace -lt $need) {
    throw ("Not enough free space on {0}. Need {1:N1} GiB (archive, unpacked files and 1 GiB spare), have {2:N1} GiB." -f
        $drive.Name, ($need / 1GB), ($drive.AvailableFreeSpace / 1GB))
}

# 3. Download (resumes a .part file after an interruption).
function Get-Sha256Lower([string] $Path) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }

$archiveReady = $false
if (Test-Path -LiteralPath $ArchivePath -PathType Leaf) {
    if ((Get-Item -LiteralPath $ArchivePath).Length -eq $expectedBytes -and (Get-Sha256Lower $ArchivePath) -eq $expectedSha256) {
        Write-Output "Using the verified archive that is already at $ArchivePath"
        $archiveReady = $true
    } else {
        throw "A file already exists at $ArchivePath and does not match the official archive. Move it or pass another -ArchivePath."
    }
}
if (-not $archiveReady) {
    Write-Output ("Downloading ComfyUI {0} portable ({1:N2} GiB) from GitHub ..." -f $version, ($expectedBytes / 1GB))
    $ok = $false
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        & $curl.Source '--silent' '--show-error' '--location' '--fail' '--retry' '3' '--retry-all-errors' '-C' '-' '--output' $partPath $url
        $have = 0L
        if (Test-Path -LiteralPath $partPath) { $have = (Get-Item -LiteralPath $partPath).Length }
        if ($LASTEXITCODE -eq 0 -and $have -eq $expectedBytes) { $ok = $true; break }
        Write-Output "Attempt $attempt of 3 did not finish (curl exit $LASTEXITCODE, $have bytes)."
        if ($have -gt $expectedBytes) { Remove-Item -LiteralPath $partPath -Force }
        Start-Sleep -Seconds (5 * $attempt)
    }
    if (-not $ok) { throw "Download did not finish. The partial file is kept so the next run can resume it: $partPath" }

    # 4. Verify.
    Write-Output 'Checking SHA-256 ...'
    $actual = Get-Sha256Lower $partPath
    if ($actual -ne $expectedSha256) {
        Remove-Item -LiteralPath $partPath -Force
        throw "SHA-256 mismatch. Expected $expectedSha256, got $actual. The bad download was deleted."
    }
    Move-Item -LiteralPath $partPath -Destination $ArchivePath
}

# 5. Unpack into a staging folder, then move the top-level folder to -InstallDir.
$staging = Join-Path $parentDir ('horus-extract-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $staging | Out-Null
try {
    Write-Output 'Unpacking (a few minutes) ...'
    & $SevenZip 'x' $ArchivePath "-o$staging" '-y' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "7-Zip failed with exit code $LASTEXITCODE. The archive is kept at $ArchivePath" }
    $top = Join-Path $staging 'ComfyUI_windows_portable'
    if (-not (Test-Path -LiteralPath (Join-Path $top 'ComfyUI\main.py') -PathType Leaf)) {
        throw 'The archive does not have the expected ComfyUI_windows_portable\ComfyUI\main.py layout.'
    }
    if (Test-Path -LiteralPath $InstallDir) { Remove-Item -LiteralPath $InstallDir -Force }   # empty folder only; checked above
    Move-Item -LiteralPath $top -Destination $InstallDir
} finally {
    Remove-Item -LiteralPath $staging -Recurse -Force -ErrorAction SilentlyContinue
}

if (-not $KeepArchive) {
    Remove-Item -LiteralPath $ArchivePath -Force
    Write-Output 'Deleted the archive. Use -KeepArchive to keep it next time.'
}
Write-Output "OK: ComfyUI $version is installed in $InstallDir"
Write-Output "Next: .\scripts\start-comfyui.ps1 -ComfyUIDir '$InstallDir'"
