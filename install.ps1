# install.ps1 — Build and install json2pptx on Windows (no bash/make required).
#
# Usage:
#   .\install.ps1                        # Build + install to %LOCALAPPDATA%\json2pptx
#   .\install.ps1 -Prefix "C:\tools"     # Custom install prefix
#   .\install.ps1 -SkipBuild             # Use pre-built bin\json2pptx.exe
#   .\install.ps1 -SkipSkill             # Skip Claude Code skill
#   .\install.ps1 -SkipMcp              # Skip MCP server config
#   .\install.ps1 -SkipTemplates        # Skip template file installation

param(
    [string]$Prefix = "$env:LOCALAPPDATA\json2pptx",
    [switch]$SkipBuild,
    [switch]$SkipSkill,
    [switch]$SkipMcp,
    [switch]$SkipTemplates,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
    Write-Host @"
Usage: .\install.ps1 [OPTIONS]

Options:
  -Prefix DIR      Install prefix (default: %LOCALAPPDATA%\json2pptx)
  -SkipBuild       Use pre-built bin\json2pptx.exe (skip Go compilation)
  -SkipSkill       Don't install Claude Code skill
  -SkipMcp         Don't install MCP server config
  -SkipTemplates   Don't install template files
  -Help            Show this help

Installs:
  <Prefix>\bin\json2pptx.exe                CLI binary (also serves as MCP server)
  ~\.json2pptx\templates\                   PPTX template files
  ~\.claude\skills\*\                       Claude Code skill files (3 skills)
  ~\.claude\mcp.json                        MCP server configuration
"@
    exit 0
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "==> json2pptx Windows installer"
Write-Host "    prefix: $Prefix"
Write-Host ""

# --- Prerequisites ---

if (-not $SkipBuild) {
    # Check Go
    $GoCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $GoCmd) {
        Write-Host "ERROR: Go is required but not installed." -ForegroundColor Red
        Write-Host "       Download from https://go.dev/dl/"
        exit 1
    }

    $GoVersionRaw = (go version) -replace '.*go(\d+\.\d+).*', '$1'
    $GoMajor, $GoMinor = $GoVersionRaw -split '\.'
    if ([int]$GoMajor -lt 1 -or ([int]$GoMajor -eq 1 -and [int]$GoMinor -lt 23)) {
        Write-Host "ERROR: Go >= 1.23 required (found $GoVersionRaw)" -ForegroundColor Red
        exit 1
    }
    Write-Host "    go: $(go version)"
}

# --- Version info ---

$Version = "dev"
$Commit = "unknown"
try {
    $Version = git describe --tags --always --dirty 2>$null
    if (-not $Version) { $Version = "dev" }
} catch { }
try {
    $Commit = git rev-parse --short HEAD 2>$null
    if (-not $Commit) { $Commit = "unknown" }
} catch { }
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$LdFlags = "-s -w -X main.Version=$Version -X main.CommitSHA=$Commit -X main.BuildTime=$BuildTime"

# --- Build ---

$BinDir = Join-Path $ScriptDir "bin"

if (-not $SkipBuild) {
    Write-Host ""
    Write-Host "==> Building..."

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

    # Main module binaries
    Write-Host "    Building json2pptx..."
    Push-Location $ScriptDir
    go build -ldflags $LdFlags -o (Join-Path $BinDir "json2pptx.exe") ./cmd/json2pptx
    Pop-Location

    # svggen module binaries
    $SvggenDir = Join-Path $ScriptDir "svggen"
    if (Test-Path (Join-Path $SvggenDir "go.mod")) {
        foreach ($cmd in @("svggen", "svggen-server", "svggen-mcp")) {
            $cmdDir = Join-Path $SvggenDir "cmd/$cmd"
            if (Test-Path $cmdDir) {
                Write-Host "    Building $cmd..."
                Push-Location $SvggenDir
                go build -ldflags $LdFlags -o (Join-Path $BinDir "$cmd.exe") "./cmd/$cmd"
                Pop-Location
            }
        }
    }
}

# Verify main binary exists
$MainBinary = Join-Path $BinDir "json2pptx.exe"
if (-not (Test-Path $MainBinary)) {
    Write-Host "ERROR: bin\json2pptx.exe not found. Run without -SkipBuild or build first." -ForegroundColor Red
    exit 1
}

# --- Install binaries ---

Write-Host ""
Write-Host "==> Installing binaries to $Prefix\bin\"
$InstallBinDir = Join-Path $Prefix "bin"
New-Item -ItemType Directory -Force -Path $InstallBinDir | Out-Null

$InstallCmds = @("json2pptx", "svggen", "svggen-server", "svggen-mcp")
foreach ($cmd in $InstallCmds) {
    $src = Join-Path $BinDir "$cmd.exe"
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $InstallBinDir "$cmd.exe") -Force
        Write-Host "    $InstallBinDir\$cmd.exe"
    }
}

# --- Install templates ---

if (-not $SkipTemplates) {
    Write-Host ""
    Write-Host "==> Installing templates..."
    $TemplatesSrc = Join-Path $ScriptDir "templates"
    $TemplatesDst = Join-Path $env:USERPROFILE ".json2pptx\templates"
    New-Item -ItemType Directory -Force -Path $TemplatesDst | Out-Null

    $PptxFiles = Get-ChildItem (Join-Path $TemplatesSrc "*.pptx") -ErrorAction SilentlyContinue
    if ($PptxFiles) {
        Copy-Item $PptxFiles.FullName $TemplatesDst -Force
        Write-Host "    $TemplatesDst ($($PptxFiles.Count) templates)"
    } else {
        Write-Host "    WARNING: No .pptx templates found in templates/" -ForegroundColor Yellow
    }
}

# --- Install Claude Code skill ---

if (-not $SkipSkill) {
    Write-Host ""
    Write-Host "==> Installing Claude Code skills..."

    # Clean up old skill name
    $OldSkillDst = Join-Path $env:USERPROFILE ".claude\skills\make-slides"
    if (Test-Path $OldSkillDst) {
        Remove-Item -Recurse -Force $OldSkillDst
        Write-Host "    Removed old skill: $OldSkillDst"
    }

    $SkillNames = @("template-deck", "generate-deck", "slide-visual-qa")
    foreach ($SkillName in $SkillNames) {
        $SkillSrc = Join-Path $ScriptDir "skills\$SkillName"
        $SkillDst = Join-Path $env:USERPROFILE ".claude\skills\$SkillName"

        if (Test-Path $SkillSrc) {
            New-Item -ItemType Directory -Force -Path $SkillDst | Out-Null
            Copy-Item (Join-Path $SkillSrc "*") $SkillDst -Force
            Write-Host "    $SkillDst"
        } else {
            Write-Host "    Skipped $SkillName (no skill files found)" -ForegroundColor Yellow
        }
    }
}

# --- Install MCP config ---

if (-not $SkipMcp) {
    Write-Host ""
    Write-Host "==> Configuring MCP server..."

    $McpFile = Join-Path $env:USERPROFILE ".claude\mcp.json"
    # Use forward slashes in JSON paths for cross-platform compatibility
    $BinaryPath = (Join-Path $InstallBinDir "json2pptx.exe") -replace '\\', '/'
    $TemplatesPath = (Join-Path $env:USERPROFILE ".json2pptx\templates") -replace '\\', '/'

    $NewServer = @{
        command = $BinaryPath
        args = @("mcp", "--templates-dir", $TemplatesPath, "--output", "./output")
    }

    if (Test-Path $McpFile) {
        $McpConfig = Get-Content $McpFile -Raw | ConvertFrom-Json
    } else {
        New-Item -ItemType Directory -Force -Path (Split-Path $McpFile) | Out-Null
        $McpConfig = [PSCustomObject]@{ mcpServers = [PSCustomObject]@{} }
    }

    # Add or update the json2pptx server entry
    if ($McpConfig.mcpServers.PSObject.Properties["json2pptx"]) {
        $McpConfig.mcpServers.json2pptx = $NewServer
    } else {
        $McpConfig.mcpServers | Add-Member -NotePropertyName "json2pptx" -NotePropertyValue $NewServer
    }

    $McpConfig | ConvertTo-Json -Depth 10 | Set-Content $McpFile -Encoding UTF8
    Write-Host "    $McpFile (json2pptx server configured)"
}

# --- Verify ---

Write-Host ""
Write-Host "==> Verifying..."
$ExePath = Join-Path $InstallBinDir "json2pptx.exe"
try {
    $VersionOut = & $ExePath version 2>&1
    Write-Host "    $VersionOut"
} catch {
    Write-Host "    WARNING: json2pptx version check failed." -ForegroundColor Yellow
}

# --- Summary ---

Write-Host ""
Write-Host "==> Done!" -ForegroundColor Green
Write-Host ""
Write-Host "  Binaries:  $InstallBinDir\"
if (-not $SkipTemplates) {
    Write-Host "  Templates: $env:USERPROFILE\.json2pptx\templates\"
}
if (-not $SkipSkill) {
    Write-Host "  Skills:    ~\.claude\skills\{template-deck,generate-deck,slide-visual-qa}\"
}
if (-not $SkipMcp) {
    Write-Host "  MCP:       ~\.claude\mcp.json"
}

# PATH warning
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($CurrentPath -notlike "*$InstallBinDir*") {
    Write-Host ""
    Write-Host "NOTE: $InstallBinDir is not in your PATH." -ForegroundColor Yellow
    Write-Host "      To add it permanently, run:"
    Write-Host ""
    Write-Host "    [Environment]::SetEnvironmentVariable('PATH', `"$InstallBinDir;`$env:PATH`", 'User')"
    Write-Host ""
    Write-Host "      Then restart your terminal."
}
