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
  %LOCALAPPDATA%\json2pptx\templates\       PPTX template files
  ~\.claude\skills\*\                       Claude Code skill files (every skill under skills\)
  claude mcp add --scope user json2pptx     MCP server registration
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

    # The minimum comes from go.mod's `go` directive so the check cannot drift
    # from what the build requires. Query the local toolchain from outside the
    # module so go.mod does not trigger a toolchain switch.
    $GoMinRaw = (Select-String -Path (Join-Path $ScriptDir "go.mod") -Pattern '^go\s+(\S+)').Matches[0].Groups[1].Value
    Push-Location "$env:SystemDrive\"
    $GoVersionRaw = (go env GOVERSION) -replace '^go', ''
    Pop-Location
    $GoMin = [version]($GoMinRaw -replace '^(\d+(\.\d+){1,2}).*', '$1')
    $GoFound = [version]($GoVersionRaw -replace '^(\d+(\.\d+){1,2}).*', '$1')
    if ($GoFound -lt $GoMin) {
        Write-Host "ERROR: Go >= $GoMinRaw required by go.mod (found $GoVersionRaw)" -ForegroundColor Red
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
    $TemplatesDst = Join-Path $env:LOCALAPPDATA "json2pptx\templates"
    New-Item -ItemType Directory -Force -Path $TemplatesDst | Out-Null

    # Only the templates embedded in the binary ship: read the list from the
    # //go:embed directive in templates/embed.go (the single source of truth,
    # shared with scripts/shipped-templates.sh) so a local gitignored template
    # such as p-style.pptx is never installed.
    $EmbedLine = Select-String -Path (Join-Path $TemplatesSrc "embed.go") -Pattern '^//go:embed ' | Select-Object -First 1
    $Shipped = @()
    if ($EmbedLine) {
        $Shipped = @(($EmbedLine.Line -split '\s+') | Where-Object { $_ -like '*.pptx' } | ForEach-Object { $_ -replace '\.pptx$', '' })
    }
    $Installed = 0
    foreach ($Name in $Shipped) {
        $Src = Join-Path $TemplatesSrc "$Name.pptx"
        if (Test-Path $Src) {
            Copy-Item $Src $TemplatesDst -Force
            $Installed++
        }
        $PreviewSrc = Join-Path $TemplatesSrc "previews\$Name"
        if (Test-Path $PreviewSrc) {
            $PreviewDst = Join-Path $TemplatesDst "previews"
            New-Item -ItemType Directory -Force -Path $PreviewDst | Out-Null
            Copy-Item $PreviewSrc $PreviewDst -Recurse -Force
        }
    }
    if ($Installed -gt 0) {
        Write-Host "    $TemplatesDst ($Installed templates)"
    } else {
        Write-Host "    WARNING: No templates listed in templates/embed.go were found in templates/" -ForegroundColor Yellow
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

    # PowerShell port of scripts/stage-skills.sh (what `make install` runs):
    # every skill under skills\, the references\repository snapshot, and
    # ../../docs-style links rewritten so they resolve inside ~\.claude\skills.
    $SkillsRoot = Join-Path $ScriptDir "skills"
    $SkillsDst = Join-Path $env:USERPROFILE ".claude\skills"
    foreach ($SkillDir in Get-ChildItem $SkillsRoot -Directory) {
        $SkillDst = Join-Path $SkillsDst $SkillDir.Name
        New-Item -ItemType Directory -Force -Path $SkillDst | Out-Null
        Copy-Item (Join-Path $SkillDir.FullName "*") $SkillDst -Recurse -Force
        Write-Host "    $SkillDst"
    }

    $Refs = Join-Path $SkillsDst "generate-deck\references\repository"
    foreach ($Sub in @("docs", "examples", "internal")) {
        New-Item -ItemType Directory -Force -Path (Join-Path $Refs $Sub) | Out-Null
    }
    foreach ($Doc in @("INPUT_FORMAT", "FIT_FINDINGS", "SEMANTIC_COMPILER", "TEMPLATE_SPEC", "PATH_GRAMMAR", "PATTERNS", "TEMPLATE_ANALYSIS")) {
        Copy-Item (Join-Path $ScriptDir "docs\$Doc.md") (Join-Path $Refs "docs") -Force
    }
    Copy-Item (Join-Path $ScriptDir "examples\semantic") (Join-Path $Refs "examples") -Recurse -Force
    Copy-Item (Join-Path $ScriptDir "internal\tokens") (Join-Path $Refs "internal") -Recurse -Force
    Copy-Item $SkillsRoot $Refs -Recurse -Force
    $Evidence = "tests\quality\evidence\connectors\midnight-blue"
    $EvidenceDst = Join-Path $Refs $Evidence
    New-Item -ItemType Directory -Force -Path $EvidenceDst | Out-Null
    foreach ($Resource in @("source-aware-evidence-route.json", "readable-source-companion-route.json", "powerpoint-slide-4.png")) {
        Copy-Item (Join-Path $ScriptDir "$Evidence\$Resource") $EvidenceDst -Force
    }

    # Only installed entrypoint guides (skills\<name>\*.md) need rerouting.
    foreach ($SkillDir in Get-ChildItem $SkillsRoot -Directory) {
        foreach ($Source in Get-ChildItem $SkillDir.FullName -Filter "*.md" -File) {
            $Guide = Join-Path (Join-Path $SkillsDst $SkillDir.Name) $Source.Name
            $Text = [IO.File]::ReadAllText($Guide)
            foreach ($Sub in @("docs", "examples", "internal", "tests")) {
                $Text = $Text.Replace("](../../$Sub/", "](../generate-deck/references/repository/$Sub/")
            }
            [IO.File]::WriteAllText($Guide, $Text)
        }
    }
}

# --- Install MCP config ---

if (-not $SkipMcp) {
    Write-Host ""
    Write-Host "==> Configuring MCP server..."

    # Claude Code reads user-scope MCP servers from ~/.claude.json, managed by
    # `claude mcp add --scope user` (it does not read ~/.claude/mcp.json).
    # Use forward slashes in paths for cross-platform compatibility.
    $BinaryPath = (Join-Path $InstallBinDir "json2pptx.exe") -replace '\\', '/'
    $TemplatesPath = (Join-Path $env:LOCALAPPDATA "json2pptx\templates") -replace '\\', '/'
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
    Write-Host "  Templates: $env:LOCALAPPDATA\json2pptx\templates\"
}
if (-not $SkipSkill) {
    Write-Host "  Skills:    ~\.claude\skills\*\"
}
if (-not $SkipMcp) {
    Write-Host "  MCP:       json2pptx (Claude Code user scope; see: claude mcp get json2pptx)"
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
