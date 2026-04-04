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
  ~\.claude\skills\*\                       Claude Code skill files (3 skills)
  ~\.claude\mcp.json                        MCP server configuration (merged)
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

    foreach ($SkillName in @("template-deck", "generate-deck", "slide-visual-qa")) {
        $SkillSrc = Join-Path $ScriptDir "skills\$SkillName"
        $SkillDst = Join-Path $env:USERPROFILE ".claude\skills\$SkillName"

        if (Test-Path $SkillSrc) {
            New-Item -ItemType Directory -Force -Path $SkillDst | Out-Null
            Copy-Item (Join-Path $SkillSrc "*") $SkillDst -Force
            Write-Host "    $SkillDst"
        }
    }
}

# --- Install MCP config ---

if (-not $SkipMcp) {
    Write-Host ""
    Write-Host "==> Configuring MCP server..."

    $McpFile = Join-Path $env:USERPROFILE ".claude\mcp.json"
    $BinaryPath = (Join-Path $BinDir "json2pptx.exe") -replace '\\', '/'
    $TemplatesDir = (Join-Path $env:USERPROFILE ".json2pptx\templates") -replace '\\', '/'

    $NewServer = @{
        command = $BinaryPath
        args = @("mcp", "--templates-dir", $TemplatesDir, "--output", "./output")
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
    Write-Host "  Skills:    ~\.claude\skills\{template-deck,generate-deck,slide-visual-qa}\"
}
if (-not $SkipMcp) {
    Write-Host "  MCP:       ~\.claude\mcp.json (json2pptx server)"
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
