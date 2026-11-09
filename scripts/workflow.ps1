<#
.SYNOPSIS
List the horus workflows, and install, check or remove the files that one workflow needs.

.DESCRIPTION
Actions:

  list                 Show every workflow under workflows\ with its description,
                       its model files and whether they are on disk (size check).
  check   <workflow>   Is this machine ready to run the workflow?
                         1. Every model file in models.json is on disk, with the
                            right size and SHA-256 (-Quick: size only).
                         2. models.json lists every model file that workflow.json
                            loads (a missing entry means install is incomplete).
                         3. When ComfyUI runs: every node class in workflow.json
                            exists on the server, and the server lists every
                            model file (so its folder mapping is right).
  install <workflow>   Download and verify the files in models.json, then run
                       check. A file that is already present and verified is
                       skipped. A wrong file is never overwritten.
  remove  <workflow>   Delete the model files of the workflow. Without -Yes it
                       only prints the plan. A file that another workflow also
                       lists is kept unless you add -Force. A file whose size
                       differs from models.json is never deleted.

-Only limits install, check and remove to some files. Each value matches a
role (for example vae, lora, text_encoder) or a file name.

Where the script looks for a model file of folder F (for example "vae"):
  1. -ModelsDir\F (or HORUS_MODELS_DIR\F). For F = text_encoders,
     -TextEncoderDir (or HORUS_TEXT_ENCODER_DIR) comes first.
  2. <ComfyUIDir>\ComfyUI\models\F
  3. Every folder that ComfyUI's extra_model_paths.yaml maps to F.
The first folder that holds the file is used. A new download goes to the first
folder in this list. When that folder is outside ComfyUI's own models folder,
install adds it to extra_model_paths.yaml (a backup is made first). Restart
ComfyUI after a change to that file.

models.json shape (per-file "repo", "revision" or "url" replace the top-level
source, for example for a LoRA from another repository):
  { "repo": "...", "revision": "...", "license": "...",
    "files": [ { "role": "vae", "repo_path": "vae/x.safetensors", "folder": "vae",
                 "bytes": 123, "sha256": "..." } ] }

Exit code 0 = the action succeeded. Exit code 1 = anything else.

.PARAMETER Action
list, check, install or remove.

.PARAMETER Workflow
Folder name under workflows\ (for example it-illustration).

.PARAMETER Only
Role or file name filter. Several values are allowed: -Only vae,lora

.PARAMETER Yes
remove: delete the files. Without it, remove only prints the plan.

.PARAMETER Force
remove: also delete a file that another workflow lists.

.PARAMETER Quick
check and list: compare the size only, do not compute SHA-256.

.PARAMETER Json
list: print JSON instead of a table.
check, install, remove: print the normal lines, and then one last line
"HORUS_JSON {...}" with the result: ok, action, workflow, message and one item
per check or file (status, kind, role, file, path, bytes, detail). The horus
MCP server reads this line. Output is UTF-8.

.PARAMETER ComfyUIDir
The portable folder. Default: HORUS_COMFYUI_DIR, then
"$env:USERPROFILE\ComfyUI_windows_portable".

.PARAMETER ServerUrl
ComfyUI address for the server checks. Default: HORUS_COMFYUI_URL, then
http://127.0.0.1:8188.

.PARAMETER ModelsJson
Use this file instead of workflows\<Workflow>\models.json.

.PARAMETER WorkflowsDir
Use this folder instead of the repo's workflows\ folder (for tests).

.EXAMPLE
.\scripts\workflow.ps1 list

.EXAMPLE
.\scripts\workflow.ps1 install it-illustration

.EXAMPLE
.\scripts\workflow.ps1 remove it-illustration -Only vae        # prints the plan
.\scripts\workflow.ps1 remove it-illustration -Only vae -Yes   # deletes
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)] [ValidateSet('list', 'check', 'install', 'remove')] [string] $Action,
    [Parameter(Position = 1)] [string] $Workflow = '',
    [string[]] $Only = @(),
    [switch] $Yes,
    [switch] $Force,
    [switch] $Quick,
    [switch] $Json,
    [string] $ComfyUIDir = '',
    [string] $ModelsDir = '',
    [string] $TextEncoderDir = '',
    [string] $ServerUrl = '',
    [string] $ModelsJson = '',
    [string] $WorkflowsDir = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$modelExtensions = '\.(safetensors|sft|gguf|ckpt|pt|pth|bin|onnx)$'

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

# Result items for -Json. Each printed status line also adds one item.
$script:items = New-Object System.Collections.ArrayList
function Add-Item([string] $Status, [string] $Kind, [string] $Role = '', [string] $File = '', [string] $Path = '', [long] $Bytes = -1, [string] $Detail = '') {
    [void] $script:items.Add([ordered]@{ status = $Status; kind = $Kind; role = $Role; file = $File; path = $Path; bytes = $Bytes; detail = $Detail })
}
function Write-JsonResult([bool] $Ok, [string] $Message) {
    if (-not $Json -or $Action -eq 'list') { return }
    $result = [ordered]@{ ok = $Ok; action = $Action; workflow = $Workflow; message = $Message; items = $script:items.ToArray() }
    [Console]::Out.WriteLine('HORUS_JSON ' + (ConvertTo-Json -InputObject $result -Depth 5 -Compress))
}
if ($Json) { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false) }

function Get-Description([string] $Readme) {
    # The first paragraph after the "# name" heading.
    if (-not (Test-Path -LiteralPath $Readme -PathType Leaf)) { return '(no README.md)' }
    $lines = [IO.File]::ReadAllLines($Readme, [Text.Encoding]::UTF8)
    $paragraph = @()
    $afterTitle = $false
    foreach ($l in $lines) {
        if (-not $afterTitle) { if ($l -match '^#\s') { $afterTitle = $true }; continue }
        if ($l.Trim() -eq '') { if ($paragraph.Count) { break } else { continue } }
        if ($l -match '^(#|\||!\[|```)') { if ($paragraph.Count) { break } else { continue } }
        $paragraph += $l.Trim()
    }
    if (-not $paragraph.Count) { return '(no description)' }
    return ($paragraph -join ' ')
}

function Read-Manifest([string] $Path) {
    # Returns the manifest header and one object per file, with its download address.
    $m = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    $files = @()
    foreach ($f in @($m.files)) {
        if (-not $f) { continue }
        $repo = if ($f.repo) { $f.repo } else { $m.repo }
        $rev = if ($f.revision) { $f.revision } else { $m.revision }
        $url = if ($f.url) { [string] $f.url } else { "https://huggingface.co/$repo/resolve/$rev/$($f.repo_path)" }
        $name = if ($f.file) { [string] $f.file } elseif ($f.repo_path) { Split-Path -Leaf $f.repo_path } else { Split-Path -Leaf ([uri] $url).AbsolutePath }
        if (-not $f.folder) { throw "A file in $Path has no 'folder'." }
        if ([string] $f.sha256 -notmatch '^[A-Fa-f0-9]{64}$') { throw "File $name in $Path has no valid sha256." }
        if ([long] $f.bytes -le 0) { throw "File $name in $Path has no valid bytes." }
        $files += [pscustomobject]@{
            Role = [string] $f.role; Name = $name; Folder = [string] $f.folder
            Bytes = [long] $f.bytes; Sha256 = ([string] $f.sha256).ToLowerInvariant(); Url = $url
        }
    }
    return [pscustomobject]@{ License = [string] $m.license; Files = $files }
}

function Select-Files($Files) {
    if (-not $Only.Count) { return @($Files) }
    $picked = @($Files | Where-Object { $f = $_; @($Only | Where-Object { $_ -ieq $f.Role -or $_ -ieq $f.Name }).Count -gt 0 })
    if (-not $picked.Count) { throw "-Only $($Only -join ',') matches no file. Roles: $((@($Files | ForEach-Object Role) | Select-Object -Unique) -join ', ')" }
    return $picked
}

function Get-YamlFolders([string] $YamlPath) {
    # Minimal reader for extra_model_paths.yaml: returns @{ folder = @(full paths) }.
    $map = @{}
    if (-not (Test-Path -LiteralPath $YamlPath -PathType Leaf)) { return $map }
    $base = ''
    $blockKey = ''
    foreach ($raw in [IO.File]::ReadAllLines($YamlPath)) {
        $line = ($raw -replace '\s+#.*$', '').TrimEnd()
        if ($line.Trim() -eq '' -or $line.TrimStart().StartsWith('#')) { continue }
        if ($line -match '^\S') { $base = ''; $blockKey = ''; continue }   # new top-level section
        if ($line -match '^\s+base_path:\s*(.+)$') { $base = $Matches[1].Trim().Trim('"').Trim("'"); $blockKey = ''; continue }
        if ($line -match '^\s+is_default:') { $blockKey = ''; continue }
        if ($line -match '^\s+([A-Za-z0-9_]+):\s*\|\s*$') { $blockKey = $Matches[1]; continue }
        if ($line -match '^\s+([A-Za-z0-9_]+):\s*(.+)$') { $blockKey = ''; $vals = @($Matches[2]); $key = $Matches[1] }
        elseif ($blockKey) { $key = $blockKey; $vals = @($line.Trim()) }
        else { continue }
        foreach ($v in $vals) {
            $v = $v.Trim().Trim('"').Trim("'")
            if (-not $v) { continue }
            $p = if ([IO.Path]::IsPathRooted($v) -or -not $base) { $v } else { Join-Path $base $v }
            if (-not $map.ContainsKey($key)) { $map[$key] = @() }
            $map[$key] += (Get-FullPath $p)
        }
    }
    return $map
}

function Get-ModelDirs([string] $Folder) {
    $dirs = @()
    if ($Folder -eq 'text_encoders' -and $script:teDir) { $dirs += $script:teDir }
    $dirs += (Join-Path $script:modelsRoot $Folder)
    $dirs += (Join-Path $script:defaultModelsRoot $Folder)
    if ($script:yamlFolders.ContainsKey($Folder)) { $dirs += $script:yamlFolders[$Folder] }
    $seen = @{}
    return @($dirs | ForEach-Object { Get-FullPath $_ } | Where-Object { if ($seen.ContainsKey($_.ToLower())) { $false } else { $seen[$_.ToLower()] = $true; $true } })
}

function Find-ModelFile($File) {
    foreach ($d in (Get-ModelDirs $File.Folder)) {
        $p = Join-Path $d $File.Name
        if (Test-Path -LiteralPath $p -PathType Leaf) { return $p }
    }
    return $null
}

function Get-Workflows {
    $out = @()
    foreach ($dir in (Get-ChildItem -LiteralPath $script:workflowsRoot -Directory | Sort-Object Name)) {
        if ($dir.Name.StartsWith('_')) { continue }
        $mj = Join-Path $dir.FullName 'models.json'
        $out += [pscustomobject]@{
            Name = $dir.Name; Dir = $dir.FullName
            ModelsJson = $(if (Test-Path -LiteralPath $mj -PathType Leaf) { $mj } else { $null })
            Description = Get-Description (Join-Path $dir.FullName 'README.md')
        }
    }
    return $out
}

function Get-OtherUsers($File, [string] $Except) {
    # Names of the other workflows whose models.json lists the same folder and file name.
    $users = @()
    foreach ($w in (Get-Workflows)) {
        if ($w.Name -eq $Except -or -not $w.ModelsJson) { continue }
        foreach ($g in (Read-Manifest $w.ModelsJson).Files) {
            if ($g.Folder -ieq $File.Folder -and $g.Name -ieq $File.Name) { $users += $w.Name; break }
        }
    }
    return $users
}

function Get-AllStrings($Node) {
    # Every string value in a parsed JSON tree.
    if ($null -eq $Node) { return }
    if ($Node -is [string]) { $Node; return }
    if ($Node -is [System.Collections.IEnumerable]) { foreach ($x in $Node) { Get-AllStrings $x }; return }
    if ($Node -is [pscustomobject]) { foreach ($p in $Node.PSObject.Properties) { Get-AllStrings $p.Value } }
}

function Get-NodeTypes($Graph) {
    if ($Graph.nodes) { return @($Graph.nodes | ForEach-Object { [string] $_.type } | Where-Object { $_ } | Select-Object -Unique) }
    return @($Graph.PSObject.Properties | ForEach-Object { [string] $_.Value.class_type } | Where-Object { $_ } | Select-Object -Unique)
}

function Invoke-Check([string] $Name, $Manifest, [string] $WorkflowJson) {
    # Prints one line per check. Puts the number of failures in $script:checkFailures.
    $script:checkFailures = 0
    $files = @(Select-Files $Manifest.Files)
    Write-Output "Checking workflow '$Name': $($files.Count) model file(s)$(if ($Quick) { ', size only' })."
    foreach ($f in $files) {
        $p = Find-ModelFile $f
        if (-not $p) {
            $dirs = (Get-ModelDirs $f.Folder) -join '; '
            Write-Output "MISSING   $($f.Role) $($f.Name) (not in: $dirs)"; $script:checkFailures++
            Add-Item 'MISSING' 'file' $f.Role $f.Name '' $f.Bytes "not in: $dirs"; continue
        }
        $len = (Get-Item -LiteralPath $p).Length
        if ($len -ne $f.Bytes) {
            Write-Output "BAD SIZE  $($f.Role) $p has $len bytes, expected $($f.Bytes)"; $script:checkFailures++
            Add-Item 'BAD_SIZE' 'file' $f.Role $f.Name $p $len "expected $($f.Bytes) bytes"; continue
        }
        if (-not $Quick) {
            $got = (Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($got -ne $f.Sha256) {
                Write-Output "BAD HASH  $($f.Role) $p SHA-256 is $got, expected $($f.Sha256)"; $script:checkFailures++
                Add-Item 'BAD_HASH' 'file' $f.Role $f.Name $p $len "SHA-256 is $got, expected $($f.Sha256)"; continue
            }
        }
        Write-Output "OK        $($f.Role) $p ($len bytes$(if (-not $Quick) { ', SHA-256 verified' }))"
        Add-Item 'OK' 'file' $f.Role $f.Name $p $len $(if ($Quick) { 'size verified' } else { 'size and SHA-256 verified' })
    }

    if (-not $WorkflowJson -or -not (Test-Path -LiteralPath $WorkflowJson -PathType Leaf)) {
        Write-Output "SKIP      graph checks: no workflow.json"
        Add-Item 'SKIP' 'graph' -Detail 'no workflow.json'
        return
    }
    $graph = Get-Content -LiteralPath $WorkflowJson -Raw | ConvertFrom-Json
    $listed = @{}
    foreach ($f in $Manifest.Files) { $listed[$f.Name.ToLower()] = $true }
    $wanted = @(Get-AllStrings $graph | Where-Object { $_ -match $modelExtensions } | ForEach-Object { ($_ -replace '\\', '/').Split('/')[-1] } | Select-Object -Unique)
    $unlisted = @($wanted | Where-Object { -not $listed.ContainsKey($_.ToLower()) })
    foreach ($w in $unlisted) {
        Write-Output "UNLISTED  workflow.json loads $w, but models.json does not list it. Add it to models.json."; $script:checkFailures++
        Add-Item 'UNLISTED' 'graph' '' $w '' -1 'workflow.json loads this file, but models.json does not list it'
    }
    if (-not $unlisted.Count) {
        Write-Output "OK        models.json lists all $($wanted.Count) model file(s) that workflow.json loads"
        Add-Item 'OK' 'graph' -Detail "models.json lists all $($wanted.Count) model files of workflow.json"
    }

    try {
        $info = Invoke-RestMethod -Uri "$script:serverUrl/object_info" -TimeoutSec 30
    } catch {
        Write-Output "SKIP      server checks: no answer at $script:serverUrl (start ComfyUI with .\scripts\start-comfyui.ps1)"
        Add-Item 'SKIP' 'server' -Detail "no answer at $script:serverUrl"
        return
    }
    $types = Get-NodeTypes $graph
    $missingTypes = @($types | Where-Object { -not $info.PSObject.Properties[$_] })
    foreach ($t in $missingTypes) {
        Write-Output "NO NODE   the server has no node class '$t'. Install its node pack (MCP install_node), then restart ComfyUI."; $script:checkFailures++
        Add-Item 'NO_NODE' 'server' '' $t '' -1 "the server has no node class '$t'"
    }
    if (-not $missingTypes.Count) {
        Write-Output "OK        the server has all $($types.Count) node classes of workflow.json"
        Add-Item 'OK' 'server' -Detail "the server has all $($types.Count) node classes"
    }
    $visible = @{}
    foreach ($s in (Get-AllStrings $info)) { if ($s -match $modelExtensions) { $visible[(($s -replace '\\', '/').Split('/')[-1]).ToLower()] = $true } }
    foreach ($f in $files) {
        if ($visible.ContainsKey($f.Name.ToLower())) {
            Write-Output "OK        the server lists $($f.Name)"
            Add-Item 'OK' 'server' $f.Role $f.Name -Detail 'the server lists the file'
        } else {
            Write-Output "NOT SEEN  the server does not list $($f.Name). Check extra_model_paths.yaml, then restart ComfyUI."; $script:checkFailures++
            Add-Item 'NOT_SEEN' 'server' $f.Role $f.Name -Detail 'the server does not list the file'
        }
    }
    return
}

function Add-YamlMappings($Targets) {
    # Map download folders that are outside ComfyUI's own models folder.
    $yamlPath = Join-Path $script:comfyDir 'extra_model_paths.yaml'
    $yaml = ''
    if (Test-Path -LiteralPath $yamlPath -PathType Leaf) { $yaml = Get-Content -LiteralPath $yamlPath -Raw }
    $changed = $false
    $backedUp = $false
    $seen = @{}
    foreach ($t in $Targets) {
        $full = Get-FullPath $t.Dir
        if ($seen.ContainsKey($full.ToLower())) { continue }
        $seen[$full.ToLower()] = $true
        if ($full -ieq (Get-FullPath (Join-Path $script:defaultModelsRoot $t.Folder))) { continue }
        if ($script:yamlFolders.ContainsKey($t.Folder) -and @($script:yamlFolders[$t.Folder] | Where-Object { $_ -ieq $full }).Count) {
            Write-Output "extra_model_paths.yaml already maps $($t.Folder) -> $full"; continue
        }
        if ((Test-Path -LiteralPath $yamlPath) -and -not $backedUp) {
            Copy-Item -LiteralPath $yamlPath -Destination ("$yamlPath.bak-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
            $backedUp = $true
        }
        $base = (Split-Path -Parent $full) -replace '\\', '/'
        $leaf = Split-Path -Leaf $full
        $yaml = ($yaml.TrimEnd() + "`n`nhorus_$($t.Folder):`n    base_path: $base`n    $($t.Folder): $leaf`n").TrimStart()
        $changed = $true
        Write-Output "extra_model_paths.yaml: added $($t.Folder) -> $base/$leaf"
    }
    if ($changed) {
        [IO.File]::WriteAllText($yamlPath, $yaml, (New-Object System.Text.UTF8Encoding($false)))
        $script:yamlFolders = Get-YamlFolders $yamlPath
        Write-Output 'Restart ComfyUI so that it reads the new paths (.\scripts\start-comfyui.ps1).'
    }
}

try {
    # Settings and folders.
    if (-not $WorkflowsDir) { $WorkflowsDir = Join-Path $repoRoot 'workflows' }
    $script:workflowsRoot = $WorkflowsDir
    if (-not (Test-Path -LiteralPath $script:workflowsRoot -PathType Container)) { throw "No workflows folder: $script:workflowsRoot" }
    if (-not $ComfyUIDir) { $ComfyUIDir = Get-HorusSetting 'HORUS_COMFYUI_DIR' (Join-Path $env:USERPROFILE 'ComfyUI_windows_portable') }
    $script:comfyDir = Join-Path $ComfyUIDir 'ComfyUI'
    $script:defaultModelsRoot = Join-Path $script:comfyDir 'models'
    if (-not $ModelsDir) { $ModelsDir = Get-HorusSetting 'HORUS_MODELS_DIR' $script:defaultModelsRoot }
    $script:modelsRoot = $ModelsDir
    if (-not $TextEncoderDir) { $TextEncoderDir = Get-HorusSetting 'HORUS_TEXT_ENCODER_DIR' '' }
    $script:teDir = $TextEncoderDir
    if (-not $ServerUrl) { $ServerUrl = Get-HorusSetting 'HORUS_COMFYUI_URL' 'http://127.0.0.1:8188' }
    $script:serverUrl = $ServerUrl.TrimEnd('/')
    $script:yamlFolders = Get-YamlFolders (Join-Path $script:comfyDir 'extra_model_paths.yaml')

    if ($Action -eq 'list') {
        $rows = @()
        foreach ($w in (Get-Workflows)) {
            $files = @(); $present = 0
            if ($w.ModelsJson) {
                $files = @((Read-Manifest $w.ModelsJson).Files)
                foreach ($f in $files) {
                    $p = Find-ModelFile $f
                    if ($p -and (Get-Item -LiteralPath $p).Length -eq $f.Bytes) { $present++ }
                }
            }
            $bytes = [long] (($files | Measure-Object -Property Bytes -Sum).Sum)
            $rows += [pscustomobject]@{
                name = $w.Name; description = $w.Description
                files = $files.Count; present = $present; bytes = $bytes; gigabytes = [math]::Round($bytes / 1e9, 2)
                installed = $(if (-not $files.Count) { 'no models' } elseif ($present -eq $files.Count) { 'yes' } else { "no ($present of $($files.Count) files)" })
                roles = (@($files | ForEach-Object Role) -join ', ')
            }
        }
        if ($Json) { ConvertTo-Json -InputObject @($rows) -Depth 4; exit 0 }
        if (-not $rows.Count) { Write-Output "No workflows in $script:workflowsRoot"; exit 0 }
        foreach ($r in $rows) {
            Write-Output $r.name
            Write-Output "  $($r.description)"
            Write-Output "  Models: $($r.files) file(s), $($r.gigabytes) GB ($($r.roles)). Installed: $($r.installed) (size check)."
            Write-Output ''
        }
        Write-Output "Install: .\scripts\workflow.ps1 install <name>    Check: .\scripts\workflow.ps1 check <name>"
        exit 0
    }

    # The other actions need one workflow.
    if (-not $Workflow) { throw "Name a workflow. Run '.\scripts\workflow.ps1 list' to see them." }
    if ($Workflow -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw "Invalid workflow name: $Workflow" }
    $wfDir = Join-Path $script:workflowsRoot $Workflow
    if (-not $ModelsJson) {
        if (-not (Test-Path -LiteralPath $wfDir -PathType Container)) {
            throw "No workflow '$Workflow'. Known: $((Get-Workflows | ForEach-Object Name) -join ', ')"
        }
        $ModelsJson = Join-Path $wfDir 'models.json'
    }
    if (-not (Test-Path -LiteralPath $ModelsJson -PathType Leaf)) { throw "Model list not found: $ModelsJson" }
    $manifest = Read-Manifest $ModelsJson
    $workflowJson = Join-Path $wfDir 'workflow.json'

    if ($Action -eq 'check') {
        Invoke-Check $Workflow $manifest $workflowJson
        $fail = $script:checkFailures
        if ($fail -gt 0) { throw "$fail check(s) failed. Fix: .\scripts\workflow.ps1 install $Workflow" }
        Write-Output "OK: workflow '$Workflow' is ready."
        Write-JsonResult $true "workflow '$Workflow' is ready"
        exit 0
    }

    if ($Action -eq 'install') {
        if (-not (Test-Path -LiteralPath (Join-Path $script:comfyDir 'main.py') -PathType Leaf)) {
            throw "ComfyUI was not found in $ComfyUIDir. Run .\scripts\install-comfyui.ps1 first, or pass -ComfyUIDir."
        }
        $files = @(Select-Files $manifest.Files)
        Write-Output "License: $($manifest.License)"
        Write-Output "Installing $($files.Count) model file(s) for workflow '$Workflow'."
        $failures = 0
        $newTargets = @()
        foreach ($f in $files) {
            $dest = Find-ModelFile $f
            if (-not $dest) {
                $dir = @(Get-ModelDirs $f.Folder)[0]
                $dest = Join-Path $dir $f.Name
                $newTargets += [pscustomobject]@{ Folder = $f.Folder; Dir = $dir }
            }
            Write-Output "-- $($f.Role) -> $dest"
            $dlLines = New-Object System.Collections.ArrayList
            & (Join-Path $PSScriptRoot 'download-model.ps1') -Url $f.Url -Destination $dest `
                -ExpectedBytes $f.Bytes -Sha256 $f.Sha256 -TokenEnvVar 'HF_TOKEN' |
                ForEach-Object { [void] $dlLines.Add([string] $_); Write-Output $_ }
            if ($LASTEXITCODE -ne 0) {
                $failures++
                $why = @($dlLines | Where-Object { $_ -match '^(FAIL:|curl exit 22|Attempt )' }) -join ' | '
                Add-Item 'FAILED' 'download' $f.Role $f.Name $dest $f.Bytes $why
            } elseif (@($dlLines | Where-Object { $_ -match '^SKIP ' }).Count) {
                Add-Item 'PRESENT' 'download' $f.Role $f.Name $dest $f.Bytes 'already present and verified'
            } else {
                Add-Item 'DOWNLOADED' 'download' $f.Role $f.Name $dest $f.Bytes 'downloaded and SHA-256 verified'
            }
        }
        if ($newTargets.Count) { Add-YamlMappings $newTargets }
        if ($failures -gt 0) { throw "$failures of $($files.Count) file(s) are not ready." }
        Write-Output ''
        $Quick = $true   # download-model.ps1 has verified every hash.
        Invoke-Check $Workflow $manifest $workflowJson
        $fail = $script:checkFailures
        if ($fail -gt 0) { throw "The files are installed, but $fail check(s) failed. See the lines above." }
        Write-Output "OK: workflow '$Workflow' is installed and ready."
        Write-JsonResult $true "workflow '$Workflow' is installed and ready"
        exit 0
    }

    if ($Action -eq 'remove') {
        $files = @(Select-Files $manifest.Files)
        $plan = @()
        $refused = 0
        foreach ($f in $files) {
            $p = Find-ModelFile $f
            $parts = @((Get-ModelDirs $f.Folder) | ForEach-Object { Join-Path $_ "$($f.Name).part" } | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf })
            foreach ($pp in $parts) { $plan += [pscustomobject]@{ Path = $pp; Bytes = (Get-Item -LiteralPath $pp).Length; Label = "unfinished download of $($f.Name)"; Role = $f.Role; Name = "$($f.Name).part"; Users = '' } }
            if (-not $p) { Write-Output "ABSENT    $($f.Role) $($f.Name)"; Add-Item 'ABSENT' 'file' $f.Role $f.Name '' $f.Bytes 'not on disk'; continue }
            $len = (Get-Item -LiteralPath $p).Length
            if ($len -ne $f.Bytes) {
                Write-Output "REFUSE    $($f.Role) $p has $len bytes, models.json says $($f.Bytes). It is not the listed file. Delete it yourself."; $refused++
                Add-Item 'REFUSED' 'file' $f.Role $f.Name $p $len "the file has $len bytes, models.json says $($f.Bytes): it is not the listed file"; continue
            }
            $users = @(Get-OtherUsers $f $Workflow)
            if ($users.Count -and -not $Force) {
                Write-Output "KEEP      $($f.Role) $p (also used by: $($users -join ', '); -Force deletes it)"
                Add-Item 'KEPT_SHARED' 'file' $f.Role $f.Name $p $len "also used by: $($users -join ', ')"; continue
            }
            $plan += [pscustomobject]@{ Path = $p; Bytes = $len; Label = "$($f.Role)$(if ($users.Count) { " (also used by: $($users -join ', '))" })"; Role = $f.Role; Name = $f.Name; Users = ($users -join ', ') }
        }
        $total = [long] (($plan | Measure-Object -Property Bytes -Sum).Sum)
        if (-not $plan.Count) {
            if ($refused) { throw "Nothing deleted. $refused file(s) refused." }
            Write-Output "Nothing to delete for workflow '$Workflow'."
            Write-JsonResult $true "nothing to delete"
            exit 0
        }
        foreach ($x in $plan) { Write-Output ("{0}  {1} {2} ({3:N2} GB)" -f $(if ($Yes) { 'DELETE   ' } else { 'WOULD DEL' }), $x.Label, $x.Path, ($x.Bytes / 1e9)) }
        if (-not $Yes) {
            foreach ($x in $plan) { Add-Item 'WOULD_DELETE' 'file' $x.Role $x.Name $x.Path $x.Bytes $(if ($x.Users) { "also used by: $($x.Users)" } else { '' }) }
            Write-Output ("Dry run: nothing deleted. {0:N2} GB in {1} file(s). Add -Yes to delete." -f ($total / 1e9), $plan.Count)
            Write-JsonResult $true ("dry run: nothing deleted, {0} file(s) would be deleted" -f $plan.Count)
            exit 0
        }
        $errors = 0
        foreach ($x in $plan) {
            try {
                Remove-Item -LiteralPath $x.Path -Force
                Add-Item 'DELETED' 'file' $x.Role $x.Name $x.Path $x.Bytes ''
            } catch {
                Write-Output "FAIL      cannot delete $($x.Path): $($_.Exception.Message) (Is ComfyUI using it? Stop ComfyUI and try again.)"; $errors++
                Add-Item 'DELETE_FAILED' 'file' $x.Role $x.Name $x.Path $x.Bytes $_.Exception.Message
            }
        }
        if ($errors) { throw "$errors file(s) were not deleted." }
        if ($refused) { throw "Deleted $($plan.Count) file(s), but $refused file(s) were refused. See the REFUSE lines." }
        Write-Output ("OK: deleted {0} file(s), {1:N2} GB. Reinstall: .\scripts\workflow.ps1 install {2}" -f $plan.Count, ($total / 1e9), $Workflow)
        Write-JsonResult $true ("deleted {0} file(s)" -f $plan.Count)
        exit 0
    }
} catch {
    Write-Output "FAIL: $($_.Exception.Message)"
    Write-JsonResult $false $_.Exception.Message
    exit 1
}
