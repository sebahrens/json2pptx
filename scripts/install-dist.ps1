# install.ps1 — Install json2pptx binary, Claude Code skill, and MCP config on Windows.
#
# Usage:
#   .\install.ps1                        # Install to %LOCALAPPDATA%\json2pptx
#   .\install.ps1 -Prefix "C:\tools"     # Custom install prefix
#   .\install.ps1 -SkipSkill             # Binary only, no Claude skill
#   .\install.ps1 -SkipMcp              # Skip MCP server config

param(
    [string]$Prefix = "$env:LOCALAPPDATA\json2pptx",
    [switch]$SkipSkill,
    [switch]$SkipMcp,
    [switch]$Help
)

if ($Help) {
    Write-Host @"
Usage: .\install.ps1 [OPTIONS]

Options:
  -Prefix DIR    Install prefix (default: %LOCALAPPDATA%\json2pptx)
  -SkipSkill     Don't install Claude Code skill
  -SkipMcp       Don't install MCP server config
  -Help          Show this help

Installs:
  <Prefix>\bin\json2pptx.exe                CLI binary (also serves as MCP server)
  ~\.claude\skills\*\                       Claude Code skill files (every skill in the archive)
  claude mcp add --scope user json2pptx     MCP server registration
"@
    exit 0
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "==> json2pptx Windows installer"
Write-Host "    prefix: $Prefix"
Write-Host ""

# --- Install binary ---

Write-Host "==> Installing binary..."
$BinDir = Join-Path $Prefix "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

$Source = Join-Path $ScriptDir "bin\json2pptx.exe"
if (-not (Test-Path $Source)) {
    Write-Host "ERROR: bin\json2pptx.exe not found in archive." -ForegroundColor Red
    exit 1
}

Copy-Item $Source (Join-Path $BinDir "json2pptx.exe") -Force
Write-Host "    $BinDir\json2pptx.exe"

# --- Install templates (lean distribution) ---

$TemplateSrc = Join-Path $ScriptDir "templates"
if (Test-Path $TemplateSrc) {
    Write-Host ""
    Write-Host "==> Installing templates..."
    $TemplatesDst = Join-Path $env:USERPROFILE ".json2pptx\templates"
    New-Item -ItemType Directory -Force -Path $TemplatesDst | Out-Null
    Copy-Item (Join-Path $TemplateSrc "*.pptx") $TemplatesDst -Force
    $TemplateCount = (Get-ChildItem (Join-Path $TemplateSrc "*.pptx")).Count
    Write-Host "    $TemplatesDst ($TemplateCount templates)"
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

    # The archive's skills\ tree was already staged by scripts/stage-skills.sh
    # (every skill, references snapshot, rewritten links); install all of it.
    $SkillsRoot = Join-Path $ScriptDir "skills"
    foreach ($SkillDir in Get-ChildItem $SkillsRoot -Directory -ErrorAction SilentlyContinue) {
        $SkillDst = Join-Path $env:USERPROFILE ".claude\skills\$($SkillDir.Name)"
        New-Item -ItemType Directory -Force -Path $SkillDst | Out-Null
        Copy-Item (Join-Path $SkillDir.FullName "*") $SkillDst -Recurse -Force
        Write-Host "    $SkillDst"
    }
}

# --- Install MCP config ---

if (-not $SkipMcp) {
    Write-Host ""
    Write-Host "==> Configuring MCP server..."

    # Claude Code reads user-scope MCP servers from ~/.claude.json, managed by
    # `claude mcp add --scope user` (it does not read ~/.claude/mcp.json).
    # Use forward slashes in paths for cross-platform compatibility.
    $BinaryPath = (Join-Path $BinDir "json2pptx.exe") -replace '\\', '/'
    $TemplatesPath = (Join-Path $env:USERPROFILE ".json2pptx\templates") -replace '\\', '/'
    $McpArgs = @("mcp", "add", "--scope", "user", "json2pptx", "--", $BinaryPath, "mcp", "--templates-dir", $TemplatesPath, "--output", "./output")
    $McpManual = "claude mcp add --scope user json2pptx -- `"$BinaryPath`" mcp --templates-dir `"$TemplatesPath`" --output ./output"

    if (Get-Command claude -ErrorAction SilentlyContinue) {
        & claude mcp remove --scope user json2pptx *> $null
        & claude @McpArgs *> $null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "    Registered json2pptx with Claude Code (user scope)"
        } else {
            Write-Host "    WARNING: 'claude mcp add' failed. Register manually with:" -ForegroundColor Yellow
            Write-Host "      $McpManual"
        }
    } else {
        Write-Host "    Claude Code CLI ('claude') not found on PATH. Register the server with:" -ForegroundColor Yellow
        Write-Host "      $McpManual"
    }
}

# --- Verify ---

Write-Host ""
Write-Host "==> Verifying..."
$ExePath = Join-Path $BinDir "json2pptx.exe"
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
Write-Host "  Binary:    $BinDir\json2pptx.exe"
if (Test-Path (Join-Path $ScriptDir "templates")) {
    Write-Host "  Templates: $env:USERPROFILE\.json2pptx\templates\"
}
if (-not $SkipSkill) {
    Write-Host "  Skills:    ~\.claude\skills\*\"
}
if (-not $SkipMcp) {
    Write-Host "  MCP:       json2pptx (Claude Code user scope; see: claude mcp get json2pptx)"
}

# PATH warning
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($CurrentPath -notlike "*$BinDir*") {
    Write-Host ""
    Write-Host "NOTE: $BinDir is not in your PATH." -ForegroundColor Yellow
    Write-Host "      To add it permanently, run:"
    Write-Host ""
    Write-Host "    [Environment]::SetEnvironmentVariable('PATH', `"$BinDir;`$env:PATH`", 'User')"
    Write-Host ""
    Write-Host "      Then restart your terminal."
}
