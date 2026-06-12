# Owner: Alexandre
# Build tool for the TAP project — all required targets from the spec.

# -----------------------------------------------------------------------------
# Dynamic Go Installation Configuration
# -----------------------------------------------------------------------------
GO_VERSION := 1.22.5
GO_TARBALL := go$(GO_VERSION).linux-amd64.tar.gz
GO_URL := https://go.dev/dl/$(GO_TARBALL)

# 1. Determine where to install Go based on sgoinfre presence
ifeq ($(wildcard /sgoinfre),)
    # At home / sgoinfre does not exist: use $HOME/.local
    GO_INSTALL_DIR := $(HOME)/.local
else
    # At school / sgoinfre exists: isolate it to your user directory
    ifneq ($(wildcard /sgoinfre/students/$(USER)),)
        GO_INSTALL_DIR := /sgoinfre/students/$(USER)/.local
    else ifneq ($(wildcard /sgoinfre/$(USER)),)
        GO_INSTALL_DIR := /sgoinfre/$(USER)/.local
    else
        GO_INSTALL_DIR := /sgoinfre/.local-$(USER)
    endif
endif

# 2. Expose the custom Go binary path
GO_BIN_DIR := $(GO_INSTALL_DIR)/go/bin
CUSTOM_GO  := $(GO_BIN_DIR)/go

# 3. Dynamic intercept: Use custom Go if it exists, otherwise use system 'go'
GO := $(shell if [ -f $(CUSTOM_GO) ]; then echo $(CUSTOM_GO); else echo "go"; fi)

# -----------------------------------------------------------------------------
# CRITICAL FIX: Force Go to use sgoinfre for downloads/cache instead of $HOME
# -----------------------------------------------------------------------------
export GOPATH := $(GO_INSTALL_DIR)/go_workspace
export PATH   := $(GO_BIN_DIR):$(PATH)

# -----------------------------------------------------------------------------
# Rules and Targets
# -----------------------------------------------------------------------------
.PHONY: setup-go deps run-server run-client run-client-gui lint clean

setup-go:
	@if [ ! -f $(CUSTOM_GO) ]; then \
		echo "⚙️ Go binary not found at $(CUSTOM_GO). Starting automated setup..."; \
		echo "📂 Target directory: $(GO_INSTALL_DIR)"; \
		mkdir -p $(GO_INSTALL_DIR); \
		cd /tmp && wget -q --show-progress $(GO_URL); \
		tar -C $(GO_INSTALL_DIR) -xzf /tmp/$(GO_TARBALL); \
		rm /tmp/$(GO_TARBALL); \
		echo "✅ Go $(GO_VERSION) successfully installed at $(CUSTOM_GO)"; \
	else \
		echo "🚀 Using existing Go installation at: $$($(CUSTOM_GO) version)"; \
	fi

deps: setup-go
	$(GO) mod download

run-server: setup-go
	$(GO) run ./cmd/server data/world.json

run-client: setup-go
	$(GO) run ./cmd/cli localhost:4242

run-client-gui: setup-go
	$(GO) run ./cmd/gui

lint: setup-go
	$(GO)fmt -l . && $(GO) vet ./...

clean:
	$(GO) clean && rm -rf bin/