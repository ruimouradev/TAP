# Build tool for the TAP project (all targets required by the subject).
#
# The school's system Go is too old and we cannot sudo to upgrade it (and the
# system relies on that old version), so setup-go downloads a pinned Go into a
# user directory; every target uses it, without touching the system Go.

GO_VERSION := 1.22.5
GO_TARBALL := go$(GO_VERSION).linux-amd64.tar.gz
GO_URL := https://go.dev/dl/$(GO_TARBALL)

# Install Go in the user's space: $HOME/.local at home, /sgoinfre at school.
ifeq ($(shell test -d /sgoinfre && echo yes),)
    GO_INSTALL_DIR := $(HOME)/.local
else
    ifneq ($(shell test -d /sgoinfre/students/$(USER) && echo yes),)
        GO_INSTALL_DIR := /sgoinfre/students/$(USER)/.local
    else ifneq ($(shell test -d /sgoinfre/$(USER) && echo yes),)
        GO_INSTALL_DIR := /sgoinfre/$(USER)/.local
    else
        GO_INSTALL_DIR := /sgoinfre/.local-$(USER)
    endif
endif

GO_BIN_DIR := $(GO_INSTALL_DIR)/go/bin
CUSTOM_GO  := $(GO_BIN_DIR)/go

# Use the downloaded Go if present, otherwise the system 'go'.
GO := $(shell if [ -f $(CUSTOM_GO) ]; then echo $(CUSTOM_GO); else echo "go"; fi)

# Keep Go's cache and workspace in the user dir too.
export GOPATH := $(GO_INSTALL_DIR)/go_workspace
export PATH   := $(GO_BIN_DIR):$(PATH)

.PHONY: setup-go deps run-server run-client run-client-gui fmt lint test-race test-fuzz clean

setup-go:
	@if [ ! -f $(CUSTOM_GO) ]; then \
		echo "Go not found at $(CUSTOM_GO); installing $(GO_VERSION) into $(GO_INSTALL_DIR)..."; \
		mkdir -p $(GO_INSTALL_DIR); \
		cd /tmp && wget -q --show-progress $(GO_URL); \
		tar -C $(GO_INSTALL_DIR) -xzf /tmp/$(GO_TARBALL); \
		rm /tmp/$(GO_TARBALL); \
		echo "Go $(GO_VERSION) installed at $(CUSTOM_GO)"; \
	else \
		echo "Using existing Go at $(CUSTOM_GO)"; \
	fi

deps: setup-go
	$(GO) mod download

run-server: setup-go
	$(GO) run ./cmd/server data/world.json

run-client: setup-go
	$(GO) run ./cmd/cli

run-client-gui: setup-go
	$(GO) run ./cmd/gui

fmt: setup-go
	$(GO)fmt -w .

lint: setup-go
	$(GO)fmt -l . && $(GO) vet ./...

test-race: setup-go
	$(GO) test -race ./...

test-fuzz: setup-go
	$(GO) test -fuzz=FuzzParse -fuzztime=15s ./internal/protocol

clean:
	$(GO) clean && rm -rf bin/