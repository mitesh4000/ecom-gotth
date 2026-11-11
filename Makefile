SHELL := /bin/bash
.DEFAULT_GOAL := build

APP_NAME        := server
BIN_DIR         := bin
TOOLS_DIR       := .tools
TMP_DIR         := tmp

TEMPL_VERSION      := v0.2.793
TAILWIND_VERSION   := v4.3.5
AIR_VERSION        := v1.67.4

TEMPL_BIN       := $(TOOLS_DIR)/templ
TAILWIND_BIN    := $(TOOLS_DIR)/tailwindcss
AIR_BIN         := $(TOOLS_DIR)/air

# Tailwind publishes standalone binaries instead of requiring Node.js.
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
  TAILWIND_OS := macos
else ifeq ($(UNAME_S),Linux)
  TAILWIND_OS := linux
else ifneq (,$(filter MINGW% MSYS% CYGWIN%,$(UNAME_S)))
  TAILWIND_OS := windows
else
  $(error Unsupported operating system for the Tailwind standalone CLI: $(UNAME_S))
endif

ifeq ($(UNAME_M),x86_64)
  TAILWIND_ARCH := x64
else ifeq ($(UNAME_M),amd64)
  TAILWIND_ARCH := x64
else ifeq ($(UNAME_M),arm64)
  TAILWIND_ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
  TAILWIND_ARCH := arm64
else
  $(error Unsupported architecture for the Tailwind standalone CLI: $(UNAME_M))
endif

TAILWIND_ASSET ?= tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)
ifeq ($(TAILWIND_OS),windows)
  TAILWIND_ASSET := $(TAILWIND_ASSET).exe
endif

.PHONY: all tools generate css css-watch templ-watch build run dev clean test fmt vet

all: build

tools: $(TEMPL_BIN) $(TAILWIND_BIN)

$(BIN_DIR) $(TOOLS_DIR):
	@mkdir -p $@

$(TEMPL_BIN): | $(TOOLS_DIR)
	@echo "==> installing templ $(TEMPL_VERSION)"
	@GOBIN=$(CURDIR)/$(TOOLS_DIR) go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)

$(TAILWIND_BIN): | $(TOOLS_DIR)
	@echo "==> downloading Tailwind CSS $(TAILWIND_VERSION) ($(TAILWIND_ASSET))"
	@curl -fsSL -o $@ \
		"https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$(TAILWIND_ASSET)"
	@chmod +x $@

$(AIR_BIN): | $(TOOLS_DIR)
	@echo "==> installing air $(AIR_VERSION)"
	@GOBIN=$(CURDIR)/$(TOOLS_DIR) go install github.com/air-verse/air@$(AIR_VERSION)

generate: $(TEMPL_BIN)
	@echo "==> generating Templ Go files"
	@$(TEMPL_BIN) generate

css: $(TAILWIND_BIN)
	@echo "==> building minified Tailwind CSS"
	@mkdir -p static/css
	@$(TAILWIND_BIN) -i ./styles/input.css -o ./static/css/app.css --minify

css-watch: $(TAILWIND_BIN)
	@mkdir -p static/css
	@$(TAILWIND_BIN) -i ./styles/input.css -o ./static/css/app.css --watch=always

templ-watch: $(TEMPL_BIN)
	@$(TEMPL_BIN) generate --watch

build: generate css
	@echo "==> compiling Go binaries"
	@mkdir -p $(BIN_DIR)
	@go build -trimpath -o $(BIN_DIR)/server ./cmd/server
	@go build -trimpath -o $(BIN_DIR)/seed ./cmd/seed
	@echo "==> built $(BIN_DIR)/server and $(BIN_DIR)/seed"

run: build
	@./$(BIN_DIR)/server

dev: generate css $(AIR_BIN)
	@./scripts/dev.sh

test:
	@go test ./...

fmt:
	@gofmt -w $$(find . -name '*.go' -not -path './vendor/*' -not -path './.tools/*')

vet:
	@go vet ./...

clean:
	@rm -rf $(BIN_DIR) $(TMP_DIR)
	@echo "==> removed build output"
