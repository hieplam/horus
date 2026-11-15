# Install the horus program on Windows (64-bit, Windows PowerShell 5.1 or later).
#
#   irm https://raw.githubusercontent.com/hieplam/horus/main/install.ps1 | iex
#
# The script downloads horus-windows-amd64.exe and checksums.txt from a GitHub
# release, checks the SHA-256, and moves the program to
# %LOCALAPPDATA%\Programs\horus\horus.exe. A second run replaces the program
# (an update).
#
# Settings (environment variables):
#   HORUS_VERSION            a release, for example 0.1.0. Default: the latest release.
#   HORUS_INSTALL_BASE_URL   replaces the address of the release files (the CI tests use it).
#   HORUS_INSTALL_DIR        replaces the folder of the program.
#   HORUS_INSTALL_NO_PATH=1  leaves the PATH of the user unchanged.
#
# The script sends no token and reads no secret. It throws an error on a
# failure. It never calls exit, because "iex" runs it in the session of the user.

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    $asset = 'horus-windows-amd64.exe'

    if ($env:OS -ne 'Windows_NT') {
        throw 'install.ps1: this script supports Windows only. On macOS, use install.sh.'
    }
    if (-not [Environment]::Is64BitOperatingSystem) {
        throw 'install.ps1: this script supports 64-bit Windows only.'
    }
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

    if ($env:HORUS_INSTALL_BASE_URL) {
        $base = $env:HORUS_INSTALL_BASE_URL.TrimEnd('/')
    } elseif ($env:HORUS_VERSION) {
        $base = 'https://github.com/hieplam/horus/releases/download/v' + $env:HORUS_VERSION.TrimStart('v')
    } else {
        $base = 'https://github.com/hieplam/horus/releases/latest/download'
    }

    if ($env:HORUS_INSTALL_DIR) {
        $dir = $env:HORUS_INSTALL_DIR
    } else {
        if (-not $env:LOCALAPPDATA) { throw 'install.ps1: LOCALAPPDATA is not set.' }
        $dir = Join-Path $env:LOCALAPPDATA 'Programs\horus'
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('horus-install-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $asset from $base"
        $exeTmp = Join-Path $tmp $asset
        $sumTmp = Join-Path $tmp 'checksums.txt'
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $exeTmp
            Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $sumTmp
        } catch {
            throw "install.ps1: cannot download from $base : $($_.Exception.Message)"
        }

        $expected = $null
        foreach ($line in Get-Content -LiteralPath $sumTmp) {
            $parts = $line.Trim() -split '\s+', 2
            if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -eq $asset) { $expected = $parts[0].ToLower(); break }
        }
        if (-not $expected) { throw "install.ps1: checksums.txt has no line for $asset." }
        $actual = (Get-FileHash -LiteralPath $exeTmp -Algorithm SHA256).Hash.ToLower()
        if ($actual -ne $expected) {
            Remove-Item -LiteralPath $exeTmp -Force
            [Console]::Error.WriteLine('install.ps1: the SHA-256 of the download is not the SHA-256 in checksums.txt. Nothing is installed.')
            [Console]::Error.WriteLine("  expected: $expected")
            [Console]::Error.WriteLine("  actual:   $actual")
            throw 'install.ps1: SHA-256 check failed.'
        }

        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $target = Join-Path $dir 'horus.exe'
        $old = Join-Path $dir 'horus.exe.old'
        if (Test-Path -LiteralPath $target) {
            # A running program cannot be overwritten, but it can be renamed.
            Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
            Move-Item -LiteralPath $target -Destination $old -Force
        }
        Move-Item -LiteralPath $exeTmp -Destination $target -Force
        Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
        Write-Host "Installed $target"
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    if ($env:HORUS_INSTALL_NO_PATH -ne '1') {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $entries = @()
        if ($userPath) { $entries = @($userPath -split ';' | Where-Object { $_ }) }
        $have = $entries | Where-Object { $_.TrimEnd('\') -ieq $dir.TrimEnd('\') }
        if (-not $have) {
            $newPath = ($entries + $dir) -join ';'
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Host "Added $dir to the PATH of the user. Open a new terminal."
        }
        $inSession = ($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $dir.TrimEnd('\') }
        if (-not $inSession) { $env:Path = $env:Path.TrimEnd(';') + ';' + $dir }
    }

    Write-Host ''
    Write-Host 'Next commands:'
    Write-Host '  horus setup it-illustration'
    Write-Host '  horus register'
}
