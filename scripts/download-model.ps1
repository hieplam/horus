<#
.SYNOPSIS
Download one file and keep it only if its size and SHA-256 hash are correct.

.DESCRIPTION
Safe download for big model files.

  - Writes to "<Destination>.part" first. The final name appears only after the
    SHA-256 hash matches.
  - If the destination file already exists and matches the size and hash, the
    script prints SKIP and exits 0.
  - If the destination file exists and does NOT match, the script refuses and
    exits 1. It never overwrites or deletes a file that it did not write.
  - It needs the file size plus 1 GiB of free space on the destination drive.
  - It retries a stopped download and resumes the .part file.
  - The Hugging Face token comes from the environment variable named by
    -TokenEnvVar, or from the .env file in the repo root. The token is never
    placed in the URL, on a command line, or in the output. The script sends it
    only to huggingface.co hosts.

Exit code 0 = file is present and verified. Exit code 1 = anything else.

.PARAMETER Url
HTTPS address of the file.

.PARAMETER Destination
Full path of the final file. The folder is created when needed.

.PARAMETER ExpectedBytes
Exact size of the file in bytes.

.PARAMETER Sha256
Expected SHA-256 hash, 64 hexadecimal characters.

.PARAMETER TokenEnvVar
Name of the variable that holds the access token. Default: HF_TOKEN.

.EXAMPLE
.\scripts\download-model.ps1 -Url https://huggingface.co/org/repo/resolve/main/model.safetensors `
  -Destination D:\models\model.safetensors -ExpectedBytes 123456 -Sha256 <64 hex characters>
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Url,
    [Parameter(Mandatory = $true)] [string] $Destination,
    [Parameter(Mandatory = $true)] [long] $ExpectedBytes,
    [Parameter(Mandatory = $true)] [ValidatePattern('^[A-Fa-f0-9]{64}$')] [string] $Sha256,
    [string] $TokenEnvVar = 'HF_TOKEN',
    [int] $MaxAttempts = 4
)

$ErrorActionPreference = 'Stop'
$spareBytes = 1GB

function Get-HorusSetting([string] $Name, [string] $Default = '') {
    # Order: environment variable, then the .env file in the repo root, then the default.
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

function Get-Sha256Lower([string] $Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

try {
    if ($ExpectedBytes -le 0) { throw '-ExpectedBytes must be greater than 0.' }
    $uri = [uri] $Url
    if ($uri.Scheme -ne 'https') { throw "Only https addresses are allowed. Got: $($uri.Scheme)" }
    $dest = [IO.Path]::GetFullPath($Destination)
    $part = "$dest.part"
    $want = $Sha256.ToLowerInvariant()
    $name = Split-Path -Leaf $dest

    # 1. The file is already there.
    if (Test-Path -LiteralPath $dest -PathType Leaf) {
        $length = (Get-Item -LiteralPath $dest).Length
        if ($length -ne $ExpectedBytes) {
            throw "Refusing to overwrite $dest : it has $length bytes, expected $ExpectedBytes. Move or delete it yourself."
        }
        Write-Output "Checking SHA-256 of the existing file $name ..."
        $got = Get-Sha256Lower $dest
        if ($got -eq $want) {
            Write-Output "SKIP $name : already present and verified."
            exit 0
        }
        throw "Refusing to overwrite $dest : SHA-256 is $got, expected $want. Move or delete it yourself."
    }

    # 2. Free space: the rest of the file plus 1 GiB.
    $already = 0L
    if (Test-Path -LiteralPath $part -PathType Leaf) { $already = (Get-Item -LiteralPath $part).Length }
    $need = [math]::Max(0L, $ExpectedBytes - $already) + $spareBytes
    try {
        $drive = New-Object System.IO.DriveInfo([IO.Path]::GetPathRoot($dest))
        if ($drive.AvailableFreeSpace -lt $need) {
            throw ("Not enough free space on {0}. Need {1:N2} GiB (file plus 1 GiB spare), have {2:N2} GiB." -f
                $drive.Name, ($need / 1GB), ($drive.AvailableFreeSpace / 1GB))
        }
    } catch [System.ArgumentException] {
        Write-Output 'Warning: cannot read free space for this path (network path?). Continuing.'
    }

    New-Item -ItemType Directory -Path (Split-Path -Parent $dest) -Force | Out-Null

    # 3. Token, only for Hugging Face hosts.
    $token = Get-HorusSetting $TokenEnvVar
    $sendToken = $false
    if ($token) {
        if ($uri.Host -eq 'huggingface.co' -or $uri.Host.EndsWith('.huggingface.co')) { $sendToken = $true }
        else { Write-Output "Warning: the host $($uri.Host) is not huggingface.co. The token is not sent." }
    }
    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
    if (-not $curl) { throw 'curl.exe was not found. It ships with Windows 10 and 11.' }

    # 4. Download with retries. Each try resumes the .part file.
    Write-Output ("Downloading {0} ({1:N2} GiB) ..." -f $name, ($ExpectedBytes / 1GB))
    $complete = $false
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        $have = 0L
        if (Test-Path -LiteralPath $part -PathType Leaf) { $have = (Get-Item -LiteralPath $part).Length }
        if ($have -gt $ExpectedBytes) { Remove-Item -LiteralPath $part -Force; $have = 0L }
        if ($have -eq $ExpectedBytes) { $complete = $true; break }

        $curlArgs = @('--silent', '--show-error', '--location', '--fail', '--retry', '3', '--retry-all-errors',
            '-C', '-', '--output', $part, $uri.AbsoluteUri)
        $configPath = $null
        $exitCode = 1
        try {
            if ($sendToken) {
                # curl reads the header from a config file, so the token is never on a command line.
                # The file has no byte-order mark, because curl would read a BOM as part of the option name.
                $configPath = Join-Path $env:TEMP ('curl-auth-' + [guid]::NewGuid().ToString('N') + '.cfg')
                [IO.File]::WriteAllText($configPath, "header = `"Authorization: Bearer $token`"`n", (New-Object System.Text.UTF8Encoding($false)))
                $curlArgs = @('-K', $configPath) + $curlArgs
            }
            & $curl.Source @curlArgs
            $exitCode = $LASTEXITCODE
        } finally {
            if ($configPath) { Remove-Item -LiteralPath $configPath -Force -ErrorAction SilentlyContinue }
        }

        $have = 0L
        if (Test-Path -LiteralPath $part -PathType Leaf) { $have = (Get-Item -LiteralPath $part).Length }
        if ($exitCode -eq 0 -and $have -eq $ExpectedBytes) { $complete = $true; break }
        Write-Output "Attempt $attempt of $MaxAttempts did not finish (curl exit $exitCode, $have of $ExpectedBytes bytes)."
        if ($exitCode -eq 22) {
            Write-Output 'curl exit 22 means the server answered with an error. HTTP 401 = token missing or wrong. HTTP 403 = license not accepted for this account.'
        }
        if ($attempt -lt $MaxAttempts) { Start-Sleep -Seconds (5 * $attempt) }
    }
    if (-not $complete) {
        throw "Download did not finish after $MaxAttempts attempts. The partial file is kept for the next run: $part"
    }

    # 5. Verify, then rename.
    Write-Output "Checking SHA-256 of $name (this takes a while for big files) ..."
    $got = Get-Sha256Lower $part
    if ($got -ne $want) {
        Remove-Item -LiteralPath $part -Force
        throw "SHA-256 mismatch for $name : expected $want, got $got. The bad .part file was deleted."
    }
    Move-Item -LiteralPath $part -Destination $dest
    Write-Output "OK $name verified: $ExpectedBytes bytes, SHA-256 $got"
    exit 0
} catch {
    Write-Output "FAIL: $($_.Exception.Message)"
    exit 1
}
