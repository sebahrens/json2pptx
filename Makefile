# Go Slide Creator Makefile
#
# Targets:
#   make              Build all binaries (current OS/arch)
#   make install      Install to PREFIX (default ~/.local on Unix, %LOCALAPPDATA%\json2pptx on Windows)
#   make dist-linux   Create Linux amd64 distribution archive
#   make dist-windows Create Windows amd64 distribution archive
#   make release      Cross-compile all platforms (fails on dirty tree)
#   make clean        Remove build artifacts
#   make help         Show all targets

# Force bash on all platforms (POSIX commands like mkdir -p, [ -d ], case, etc.)
# On Windows this requires Git Bash, MSYS2, or WSL2.
# /bin/bash works on macOS, Linux, WSL2, and most Git Bash installs.
# On MSYS2/Chocolatey where bash may live elsewhere, override with:
#   make SHELL=$(which bash)
BASH_CANDIDATES := /bin/bash /usr/bin/bash
SHELL := $(firstword $(wildcard $(BASH_CANDIDATES)) bash)

# Ensure common tool paths are available (GUI editors like TextMate/Sublime use /bin/sh without profile)
# macOS:      /usr/local/bin, /opt/homebrew/bin
# Linux/WSL2: /usr/local/go/bin, /snap/bin, /usr/local/bin
# All:        ~/go/bin (GOPATH default)
export PATH := /usr/local/go/bin:/usr/local/bin:/opt/homebrew/bin:/snap/bin:$(HOME)/go/bin:$(PATH)

# Version info from git
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -ldflags "-s -w -X main.Version=$(VERSION) -X main.CommitSHA=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Platform detection
ifeq ($(OS),Windows_NT)
  EXE := .exe
  # Prefer LOCALAPPDATA; fall back to USERPROFILE, then HOME
  _WINBASE := $(or $(LOCALAPPDATA),$(USERPROFILE),$(HOME))
  PREFIX ?= $(_WINBASE)/json2pptx
  # Ensure HOME is set (Git Bash sets it, but MSYS2 minimal may not)
  HOME ?= $(USERPROFILE)
  IS_WINDOWS := 1
else
  EXE :=
  PREFIX ?= $(HOME)/.local
  IS_WINDOWS :=
endif

# Distribution archive settings
DIST_NAME         := json2pptx-$(VERSION)-linux-amd64
DIST_STAGING      := dist/$(DIST_NAME)
DIST_ARCHIVE      := dist/$(DIST_NAME).tar.gz
WIN_DIST_NAME     := json2pptx-$(VERSION)-windows-amd64
WIN_DIST_STAGING  := dist/$(WIN_DIST_NAME)
WIN_DIST_ARCHIVE  := dist/$(WIN_DIST_NAME).tar.gz

# All binaries and their source packages
# Main module (go.mod at root)
MAIN_CMDS := \
	json2pptx:cmd/json2pptx \
	testrand:cmd/testrand \
	pptx2jpg:cmd/pptx2jpg \
	validatepptx:cmd/validatepptx \
	mktemplate:cmd/mktemplate \
	debugcolors:cmd/debugcolors \
	templatecaps:cmd/templatecaps

# svggen module (svggen/go.mod)
SVGGEN_CMDS := \
	svggen:svggen/cmd/svggen \
	svggen-server:svggen/cmd/svggen-server \
	svggen-mcp:svggen/cmd/svggen-mcp

ALL_CMDS := $(MAIN_CMDS) $(SVGGEN_CMDS)

# Binaries that get installed (user-facing tools)
INSTALL_CMDS := json2pptx svggen svggen-server svggen-mcp

.PHONY: all build build-main build-svggen install install-bin install-templates \
        install-skill install-mcp uninstall \
        build-cross build-darwin-amd64 build-darwin-arm64 \
        build-linux-amd64 build-linux-arm64 build-windows-amd64 \
        ensure-templates dist-linux dist-windows release-check release \
        check test test-race test-cover test-svg-stress \
        lint vulncheck security fmt fmt-check \
        run clean ci help

# ─── Build ────────────────────────────────────────────────────────────

# Default: build everything
all: build

build: build-main build-svggen

build-main:
	@mkdir -p bin
	$(foreach pair,$(MAIN_CMDS), \
		go build $(LDFLAGS) -o bin/$(word 1,$(subst :, ,$(pair)))$(EXE) ./$(word 2,$(subst :, ,$(pair))) && \
	) true

build-svggen:
	@mkdir -p bin
	$(foreach pair,$(SVGGEN_CMDS), \
		cd svggen && go build $(LDFLAGS) -o ../bin/$(word 1,$(subst :, ,$(pair)))$(EXE) ./$(patsubst svggen/%,%,$(word 2,$(subst :, ,$(pair)))) && cd .. && \
	) true

# Build a single binary: make bin/json2pptx
bin/%: build
	@true

# ─── Install ──────────────────────────────────────────────────────────
#
# Usage:
#   make install                          # Default prefix (~/.local or %LOCALAPPDATA%\json2pptx)
#   make install PREFIX=/usr/local        # Custom prefix
#   make install SKIP_SKILL=1             # Skip Claude Code skill
#   make install SKIP_MCP=1               # Skip MCP config
#   make install SKIP_TEMPLATES=1         # Skip template files

install: build install-bin install-templates install-skill install-mcp
	@echo ""
	@echo "==> Installation complete"
	@echo "    Binaries:   $(PREFIX)/bin/"
ifndef SKIP_TEMPLATES
	@echo "    Templates:  $(HOME)/.json2pptx/templates/"
endif
ifndef SKIP_SKILL
	@echo "    Skill:      $(HOME)/.claude/skills/template-deck/"
endif
ifndef SKIP_MCP
	@echo "    MCP config: $(HOME)/.claude/mcp.json"
endif
	@# PATH check
	@case ":$(PATH):" in \
	  *":$(PREFIX)/bin:"*) ;; \
	  *) echo "" && echo "NOTE: $(PREFIX)/bin is not in your PATH."; \
	     if [ -n "$(IS_WINDOWS)" ]; then \
	       echo "      Add to your PATH (PowerShell, run as admin):"; \
	       echo ""; \
	       echo '    [Environment]::SetEnvironmentVariable("Path", "$(PREFIX)\bin;" + $$env:Path, "User")'; \
	       echo ""; \
	       echo "      Or in Git Bash / MSYS2:"; \
	       echo ""; \
	       echo '    export PATH="$(PREFIX)/bin:$$PATH"'; \
	     else \
	       echo "      Add to your shell profile:"; \
	       echo ""; \
	       echo '    export PATH="$(PREFIX)/bin:$$PATH"'; \
	     fi ;; \
	esac

install-bin: build
	@echo "==> Installing binaries to $(PREFIX)/bin/"
	@mkdir -p "$(PREFIX)/bin"
	@$(foreach cmd,$(INSTALL_CMDS), \
		cp bin/$(cmd)$(EXE) "$(PREFIX)/bin/$(cmd)$(EXE)" && \
		chmod +x "$(PREFIX)/bin/$(cmd)$(EXE)" 2>/dev/null; \
		echo "    $(PREFIX)/bin/$(cmd)$(EXE)" && \
	) true

install-templates:
ifndef SKIP_TEMPLATES
	@echo "==> Installing templates to $(HOME)/.json2pptx/templates/"
	@mkdir -p "$(HOME)/.json2pptx/templates"
	@cp templates/*.pptx "$(HOME)/.json2pptx/templates/"
	@echo "    $$(ls templates/*.pptx 2>/dev/null | wc -l | tr -d ' ') templates installed"
endif

install-skill:
ifndef SKIP_SKILL
	@echo "==> Installing Claude Code skill (template-deck)..."
	@if [ -d .claude/skills/template-deck ]; then \
		mkdir -p "$(HOME)/.claude/skills/template-deck"; \
		cp .claude/skills/template-deck/* "$(HOME)/.claude/skills/template-deck/"; \
		echo "    $(HOME)/.claude/skills/template-deck/"; \
	else \
		echo "    Skipped (no skill files found)"; \
	fi
endif

install-mcp:
ifndef SKIP_MCP
	@echo "==> Configuring MCP server in $(HOME)/.claude/mcp.json..."
	@mkdir -p "$(HOME)/.claude"
	@# On MSYS2/Git Bash, convert /c/Users/... to C:/Users/... for mcp.json
	@# so that Claude Code (running outside MSYS) can find the binary.
	@_mcp_bin="$(PREFIX)/bin/json2pptx$(EXE)"; \
	_mcp_tdir="$(HOME)/.json2pptx/templates"; \
	if [ -n "$(IS_WINDOWS)" ]; then \
		_mcp_bin=$$(echo "$$_mcp_bin" | sed 's|^/\([a-zA-Z]\)/|\1:/|'); \
		_mcp_tdir=$$(echo "$$_mcp_tdir" | sed 's|^/\([a-zA-Z]\)/|\1:/|'); \
	fi; \
	if command -v jq >/dev/null 2>&1; then \
		TMPFILE="$(HOME)/.claude/mcp.json.$$$$.tmp"; \
		if [ -f "$(HOME)/.claude/mcp.json" ]; then \
			jq --arg bin "$$_mcp_bin" --arg tdir "$$_mcp_tdir" \
				'.mcpServers["json2pptx"] = {command: $$bin, args: ["mcp", "--templates-dir", $$tdir, "--output", "./output"]}' \
				"$(HOME)/.claude/mcp.json" > "$$TMPFILE" && \
			mv "$$TMPFILE" "$(HOME)/.claude/mcp.json"; \
		else \
			printf '{"mcpServers":{"json2pptx":{"command":"%s","args":["mcp","--templates-dir","%s","--output","./output"]}}}\n' \
				"$$_mcp_bin" "$$_mcp_tdir" | jq . > "$$TMPFILE" && \
			mv "$$TMPFILE" "$(HOME)/.claude/mcp.json"; \
		fi; \
		echo "    $(HOME)/.claude/mcp.json (json2pptx server configured)"; \
	else \
		echo "    WARNING: jq not found. Add json2pptx to $(HOME)/.claude/mcp.json manually."; \
	fi
endif

uninstall:
	@echo "==> Removing installed files..."
	@$(foreach cmd,$(INSTALL_CMDS), \
		rm -f "$(PREFIX)/bin/$(cmd)$(EXE)" && \
	) true
	@rm -rf "$(HOME)/.json2pptx"
	@rm -rf "$(HOME)/.claude/skills/template-deck"
	@echo "    Done (MCP config left in place — edit ~/.claude/mcp.json manually)"

# ─── Cross-compilation ────────────────────────────────────────────────

build-cross: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/json2pptx-darwin-amd64 ./cmd/json2pptx/

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/json2pptx-darwin-arm64 ./cmd/json2pptx/

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/json2pptx-linux-amd64 ./cmd/json2pptx/

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/json2pptx-linux-arm64 ./cmd/json2pptx/

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/json2pptx-windows-amd64.exe ./cmd/json2pptx/

# ─── Distribution archives ────────────────────────────────────────────

ensure-templates:
	@if ! ls templates/*.pptx >/dev/null 2>&1; then \
		echo "ERROR: No templates found in templates/. Cannot build."; \
		exit 1; \
	fi

dist-linux: release-check ensure-templates
	@echo "==> Building Linux distribution: $(DIST_NAME)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/json2pptx-linux-amd64 ./cmd/json2pptx/
	@# Stage the archive
	rm -rf $(DIST_STAGING)
	mkdir -p $(DIST_STAGING)/bin $(DIST_STAGING)/templates
	cp bin/json2pptx-linux-amd64 $(DIST_STAGING)/bin/json2pptx
	cp templates/*.pptx $(DIST_STAGING)/templates/
	@if [ -d .claude/skills/template-deck ]; then \
		mkdir -p $(DIST_STAGING)/skills/template-deck; \
		cp .claude/skills/template-deck/* $(DIST_STAGING)/skills/template-deck/; \
	fi
	cp scripts/install-dist.sh $(DIST_STAGING)/install.sh
	chmod +x $(DIST_STAGING)/install.sh
	@echo "==> Creating archive: $(DIST_ARCHIVE)"
	cd dist && tar czf $(DIST_NAME).tar.gz $(DIST_NAME)
	@echo "==> Distribution ready: $(DIST_ARCHIVE)"
	@ls -lh $(DIST_ARCHIVE)

dist-windows: release-check ensure-templates
	@echo "==> Building Windows distribution: $(WIN_DIST_NAME)"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/json2pptx-windows-amd64.exe ./cmd/json2pptx/
	@# Stage the archive
	rm -rf $(WIN_DIST_STAGING)
	mkdir -p $(WIN_DIST_STAGING)/bin $(WIN_DIST_STAGING)/templates
	cp bin/json2pptx-windows-amd64.exe $(WIN_DIST_STAGING)/bin/json2pptx.exe
	cp templates/*.pptx $(WIN_DIST_STAGING)/templates/
	@if [ -d .claude/skills/template-deck ]; then \
		mkdir -p $(WIN_DIST_STAGING)/skills/template-deck; \
		cp .claude/skills/template-deck/* $(WIN_DIST_STAGING)/skills/template-deck/; \
	fi
	cp scripts/install-dist.ps1 $(WIN_DIST_STAGING)/install.ps1
	@echo "==> Creating archive: $(WIN_DIST_ARCHIVE)"
	cd dist && tar czf $(WIN_DIST_NAME).tar.gz $(WIN_DIST_NAME)
	@echo "==> Distribution ready: $(WIN_DIST_ARCHIVE)"
	@ls -lh $(WIN_DIST_ARCHIVE)

# ─── Testing & Quality ────────────────────────────────────────────────

check: build
	go test ./... -count=1 -timeout=120s
	go vet ./...
	cd svggen && go test ./... -count=1 -timeout=120s
	cd svggen && go vet ./...

test:
	go test ./... -v
	cd svggen && go test ./... -v

test-race:
	go test ./... -v -race
	cd svggen && go test ./... -v -race

test-cover:
	go test ./... -v -coverprofile=coverage.out
	go tool cover -func=coverage.out
	cd svggen && go test ./... -v -coverprofile=coverage.out
	cd svggen && go tool cover -func=coverage.out

test-svg-stress: build
	./bin/testrand svg-stress --seed=$${SEED:-0}

lint:
	golangci-lint run ./...
	cd svggen && golangci-lint run ./...

vulncheck:
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...
	cd svggen && govulncheck ./...

security: vulncheck lint

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Please run 'make fmt'" && gofmt -l . && exit 1)

# ─── Run / Clean / Release ────────────────────────────────────────────

run: build
	./bin/json2pptx$(EXE) serve

build-race:
	go build -race -o bin/json2pptx-race$(EXE) ./cmd/json2pptx

clean:
	rm -rf bin/ dist/
	rm -f coverage.out

release-check:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: Working tree is dirty. Commit or stash changes before building a release."; \
		git status --short; \
		exit 1; \
	fi

release: release-check ensure-templates build-cross

ci: fmt-check lint test vulncheck

# ─── Help ─────────────────────────────────────────────────────────────

help:
	@echo "Build:"
	@echo "  make                  Build all 11 binaries (current OS/arch)"
	@echo "  make build-cross      Cross-compile json2pptx for all platforms"
	@echo "  make build-race       Build server with race detector"
	@echo ""
	@echo "Install (macOS / Linux / WSL2):"
	@echo "  make install                     Install to ~/.local/bin/"
	@echo "  make install PREFIX=/usr/local   Install to /usr/local/bin/ (needs sudo)"
	@echo "  make install SKIP_SKILL=1        Skip Claude Code skill"
	@echo "  make install SKIP_MCP=1          Skip MCP server config"
	@echo "  make install SKIP_TEMPLATES=1    Skip template files"
	@echo "  make uninstall                   Remove installed files"
	@echo ""
	@echo "Install (Windows — use Git Bash or MSYS2):"
	@echo "  make install                     Install to %LOCALAPPDATA%/json2pptx/bin/"
	@echo "  make install PREFIX=C:/tools     Custom prefix"
	@echo ""
	@echo "Distribution:"
	@echo "  make dist-linux       Create Linux amd64 tar.gz"
	@echo "  make dist-windows     Create Windows amd64 tar.gz"
	@echo "  make release          Build all platforms (fails if tree is dirty)"
	@echo ""
	@echo "Test & Quality:"
	@echo "  make check            Build + test + vet"
	@echo "  make test             Run all tests"
	@echo "  make test-race        Tests with race detector"
	@echo "  make test-cover       Tests with coverage report"
	@echo "  make lint             Run golangci-lint"
	@echo "  make security         Run vulncheck + lint"
	@echo "  make ci               Full CI pipeline (fmt + lint + test + vulncheck)"
	@echo ""
	@echo "Other:"
	@echo "  make run              Build and run the server"
	@echo "  make fmt              Format all code"
	@echo "  make clean            Remove build artifacts"
	@echo ""
	@echo "Installed binaries ($(words $(INSTALL_CMDS))):"
	@echo "  $(INSTALL_CMDS)"
	@echo ""
	@echo "All binaries ($(words $(ALL_CMDS))):"
	@$(foreach pair,$(ALL_CMDS),echo "  $(word 1,$(subst :, ,$(pair)))	./$(word 2,$(subst :, ,$(pair)))";)
