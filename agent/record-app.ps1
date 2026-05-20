<#
.SYNOPSIS
    [DEPRECATED] Records a terminal application session with tuirec.

.DESCRIPTION
    DEPRECATED: Use `tuirec record --name <name>` directly instead. The tuirec
    binary now auto-downloads agg if not found, supports --name for output path
    conventions, and prints a recording summary. This script is retained for
    backward compatibility but will be removed in a future release.

    Generic recording wrapper for any terminal application. An AI agent (or human)
    supplies the --Keystrokes parameter describing what to demonstrate, and this
    script handles tool resolution and invoking tuirec record.

    Output goes to artifacts/ by default. If tuirec or agg are not on PATH,
    they are automatically downloaded from
    https://github.com/gui-cs/tuirec/releases and installed into ~/tools.

    See agent/RECORDING-AGENT.md for the keystroke syntax reference and
    guidance on composing keystroke scripts.

.PARAMETER Binary
    Path to the executable to record (required).

.PARAMETER Keystrokes
    tuirec keystroke script (required). Comma-separated sequence of keys,
    text literals, and wait directives. See RECORDING-AGENT.md for syntax.

.PARAMETER Name
    Short identifier for the recording (used in output filenames).
    Default: "demo"

.PARAMETER Title
    Human-readable title burned into the cast metadata.
    Default: "recording"

.PARAMETER Output
    GIF output path. Default: artifacts/<Name>.gif

.PARAMETER CastOutput
    Asciinema .cast output path. Default: artifacts/<Name>.cast

.PARAMETER Args
    Arguments to pass to the binary.

.PARAMETER Cols
    Recording columns. Default: 120

.PARAMETER Rows
    Recording rows. Default: 36

.PARAMETER ShowCommand
    Synthetic shell prompt/command pre-roll shown in the GIF before the app
    starts (e.g. '$ my-app config.yaml'). Omit for no pre-roll.

.PARAMETER StartupDelay
    Milliseconds to wait after the target process starts before copying its
    output and playing keystrokes. Default: 0 (no extra delay).

.PARAMETER InputDelay
    Default pause in milliseconds before the scripted keys begin (after
    startup-delay has elapsed). Default: 0.

.PARAMETER MaxDuration
    Maximum recording duration in seconds. Default: 60

.PARAMETER DrainMs
    Milliseconds to wait after last keystroke before stopping. Default: 1500

.PARAMETER Verbosity
    tuirec verbosity level: quiet, normal, high. 'high' logs key tokens and
    pacing to stderr for troubleshooting. Default: not set.

.PARAMETER KittyKeyboard
    Enable Kitty keyboard protocol. Encodes keystrokes as CSI u sequences and
    responds to app mode queries. Use for Terminal.Gui and other apps that
    support progressive enhancement â€” disambiguates Ctrl+M from Enter, etc.

.PARAMETER tuirecVersion
    tuirec release version to download if not found. Default: 0.1.5
#>
[CmdletBinding()]
param (
    [Parameter(Mandatory = $true)]
    [string] $Binary,

    [Parameter(Mandatory = $true)]
    [string] $Keystrokes,

    [string]   $Name = 'demo',
    [string]   $Title = 'recording',
    [string]   $Output,
    [string]   $CastOutput,
    [string[]] $Args,
    [int]      $Cols = 120,
    [int]      $Rows = 36,
    [int]      $MaxDuration = 60,
    [int]      $DrainMs = 1500,
    [string]   $ShowCommand,
    [int]      $StartupDelay = 0,
    [int]      $InputDelay = 0,
    [string]   $Verbosity,
    [switch]   $KittyKeyboard,
    [string]   $tuirecVersion = '0.1.5'
)

$ErrorActionPreference = 'Stop'

Write-Warning "record-app.ps1 is deprecated. Use 'tuirec record --name <name>' directly instead. tuirec now auto-downloads agg if needed."

$ToolsDir = Join-Path $HOME 'tools'

if (-not $Output) { $Output = "artifacts/${Name}.gif" }
if (-not $CastOutput) { $CastOutput = "artifacts/${Name}.cast" }

function Get-tuirecAssetName {
    if ($IsWindows -or ($PSVersionTable.PSVersion.Major -le 5)) {
        $os = 'windows'
        $ext = 'zip'
    } elseif ($IsMacOS) {
        $os = 'darwin'
        $ext = 'tar.gz'
    } else {
        $os = 'linux'
        $ext = 'tar.gz'
    }

    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
    $goArch = switch ($arch) {
        'x64'   { 'amd64' }
        'arm64' { 'arm64' }
        default { 'amd64' }
    }

    return "tuirec_${tuirecVersion}_${os}_${goArch}.${ext}"
}

function Install-tuirecTools {
    $null = New-Item -ItemType Directory -Force -Path $ToolsDir

    $asset = Get-tuirecAssetName
    $url = "https://github.com/gui-cs/tuirec/releases/download/v${tuirecVersion}/${asset}"
    $tempFile = Join-Path ([System.IO.Path]::GetTempPath()) $asset

    Write-Host "Downloading tuirec v${tuirecVersion}: $url"
    Invoke-WebRequest -Uri $url -OutFile $tempFile -UseBasicParsing

    $tempExtract = Join-Path ([System.IO.Path]::GetTempPath()) "tuirec-extract-$([guid]::NewGuid())"
    $null = New-Item -ItemType Directory -Force -Path $tempExtract

    if ($asset.EndsWith('.zip')) {
        Expand-Archive -Path $tempFile -DestinationPath $tempExtract -Force
    } else {
        tar -xzf $tempFile -C $tempExtract
    }

    $exeExt = if ($IsWindows -or ($PSVersionTable.PSVersion.Major -le 5)) { '.exe' } else { '' }
    foreach ($tool in @('tuirec', 'agg')) {
        $src = Get-ChildItem -Path $tempExtract -Recurse -Filter "${tool}${exeExt}" | Select-Object -First 1
        if ($src) {
            Copy-Item -Path $src.FullName -Destination (Join-Path $ToolsDir "${tool}${exeExt}") -Force
            Write-Host "  Installed: ~/tools/${tool}${exeExt}"
        }
    }

    Remove-Item -Recurse -Force $tempFile -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $tempExtract -ErrorAction SilentlyContinue
}

function Resolve-tuirecTool {
    param ([string] $ToolName)

    $exeExt = if ($IsWindows -or ($PSVersionTable.PSVersion.Major -le 5)) { '.exe' } else { '' }

    # Check alongside this script first (bundled in release archive)
    $scriptDir = Split-Path -Parent $MyInvocation.ScriptName
    $parentDir = Split-Path -Parent $scriptDir
    foreach ($dir in @($scriptDir, $parentDir)) {
        $candidate = Join-Path $dir "${ToolName}${exeExt}"
        if (Test-Path $candidate) { return (Resolve-Path $candidate).Path }
    }

    $found = Get-Command $ToolName -ErrorAction SilentlyContinue
    if ($found) { return $found.Source }

    $toolPath = Join-Path $ToolsDir "${ToolName}${exeExt}"
    if (Test-Path $toolPath) { return (Resolve-Path $toolPath).Path }

    return $null
}

# Resolve tuirec and agg â€” download if missing
$tuirecBin = Resolve-tuirecTool 'tuirec'
$AggBin = Resolve-tuirecTool 'agg'

if (-not $tuirecBin -or -not $AggBin) {
    Write-Host 'tuirec or agg not found. Installing...'
    Install-tuirecTools
    $tuirecBin = Resolve-tuirecTool 'tuirec'
    $AggBin = Resolve-tuirecTool 'agg'
    if (-not $tuirecBin) { throw 'Failed to install tuirec' }
    if (-not $AggBin) { throw 'Failed to install agg' }
}

# Ensure output directories exist
$null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Output)
$null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $CastOutput)

Write-Host "Recording: $Title"
Write-Host "  Binary:     $Binary"
Write-Host "  Keystrokes: $Keystrokes"
Write-Host "  Output:     $Output"

$recordArgs = @(
    'record',
    '--binary', $Binary,
    '--keystrokes', $Keystrokes,
    '--output', $Output,
    '--cast-output', $CastOutput,
    '--agg-path', $AggBin,
    '--cols', $Cols,
    '--rows', $Rows,
    '--max-duration', $MaxDuration,
    '--drain', $DrainMs,
    '--title', $Title
)

if ($Args -and $Args.Count -gt 0) {
    foreach ($a in $Args) { $recordArgs += '--args'; $recordArgs += $a }
}
if ($ShowCommand)        { $recordArgs += '--show-command';   $recordArgs += $ShowCommand }
if ($StartupDelay -gt 0) { $recordArgs += '--startup-delay'; $recordArgs += $StartupDelay }
if ($InputDelay -gt 0)   { $recordArgs += '--input-delay';   $recordArgs += $InputDelay }
if ($Verbosity)          { $recordArgs += '--verbosity';     $recordArgs += $Verbosity }
if ($KittyKeyboard)      { $recordArgs += '--kitty-keyboard' }

& $tuirecBin @recordArgs

if ($LASTEXITCODE -ne 0) { throw "tuirec record failed with exit code $LASTEXITCODE" }

Write-Host ''
Write-Host "Recording complete:"
Write-Host "  GIF:  $Output"
Write-Host "  Cast: $CastOutput"

$GifPath = (Resolve-Path $Output).Path

try {
    Set-Clipboard -Value $GifPath
    Write-Host "  GIF path copied to clipboard."
} catch {
    # Set-Clipboard not available in all environments
}

try {
    Invoke-Item -Path $GifPath
} catch {
    # Non-interactive environments can't open files
}
