.DEFAULT_GOAL := help

PACKAGE_NAME          := github.com/openshift/oc-tnf
GOLANG_CROSS_VERSION  ?= v1.25.0

HOST_OS := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)

GOOS ?= $(HOST_OS)
GOARCH ?= $(HOST_ARCH)

OUTPUT_DIR := _output
GO_BUILD_BINDIR ?= $(OUTPUT_DIR)/bin
CROSS_BUILD_BINDIR ?= $(OUTPUT_DIR)/bin

ifneq ($(strip $(OS_GIT_VERSION)),)
	BUILD_VERSION ?= $(OS_GIT_VERSION)
else
	BUILD_VERSION ?= $(shell git describe --tags 2>/dev/null | sed -e 's/^v//' || echo "unreleased")
endif
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_LDFLAGS := -ldflags="-s -w -X 'main.version=$(BUILD_VERSION)' -X 'main.date=$(BUILD_DATE)'"

PREFIX ?= /usr/local
DESTDIR ?= $(PREFIX)/bin

ifeq ($(origin ENGINE), undefined)
  ENGINE = podman
  ifeq ($(shell which $(ENGINE) 2>/dev/null), )
    ENGINE = docker
  endif
endif

ifeq ($(ENGINE), podman)
  CONTAINER_MOUNTOPT = :Z
else
  CONTAINER_MOUNTOPT =
endif

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the oc-tnf binary for current platform
	mkdir -p $(GO_BUILD_BINDIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_LDFLAGS) -o $(GO_BUILD_BINDIR)/oc-tnf ./cmd/oc-tnf/

.PHONY: install
install: build ## Install oc-tnf to /usr/local/bin (may require sudo)
	install $(GO_BUILD_BINDIR)/oc-tnf $(DESTDIR)

.PHONY: test
test: ## Run unit tests
	go test --race ./pkg/...

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint
	hack/golangci-lint.sh

.PHONY: cross-build-linux-amd64
cross-build-linux-amd64:
	+@GOOS=linux GOARCH=amd64 GO_BUILD_BINDIR=$(CROSS_BUILD_BINDIR)/linux_amd64 $(MAKE) --no-print-directory build

.PHONY: cross-build-linux-arm64
cross-build-linux-arm64:
	+@GOOS=linux GOARCH=arm64 GO_BUILD_BINDIR=$(CROSS_BUILD_BINDIR)/linux_arm64 $(MAKE) --no-print-directory build

.PHONY: cross-build-darwin-amd64
cross-build-darwin-amd64:
	+@GOOS=darwin GOARCH=amd64 GO_BUILD_BINDIR=$(CROSS_BUILD_BINDIR)/darwin_amd64 $(MAKE) --no-print-directory build

.PHONY: cross-build-darwin-arm64
cross-build-darwin-arm64:
	+@GOOS=darwin GOARCH=arm64 GO_BUILD_BINDIR=$(CROSS_BUILD_BINDIR)/darwin_arm64 $(MAKE) --no-print-directory build

.PHONY: cross-build
cross-build: cross-build-linux-amd64 cross-build-linux-arm64 cross-build-darwin-amd64 cross-build-darwin-arm64 ## Build for all supported platforms

.PHONY: release-preflight
release-preflight:
	@test -n "$(VERSION)" || { echo "ERROR: VERSION=vX.Y.Z is required"; exit 1; }
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "ERROR: VERSION must be semver with leading v (e.g. v0.0.2)"; exit 1; }
	@test -n "$(GITHUB_TOKEN)" || { echo "ERROR: GITHUB_TOKEN is required"; exit 1; }
	@git diff-index --quiet HEAD -- || { echo "ERROR: working tree is dirty — commit or stash changes first"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "main" || { echo "ERROR: releases must be cut from main"; exit 1; }
	@git fetch -q upstream main 2>/dev/null || git fetch -q origin main 2>/dev/null
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse upstream/main 2>/dev/null || git rev-parse origin/main)" || { echo "ERROR: HEAD is not in sync with upstream/main — pull first"; exit 1; }
	@! git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null 2>&1 || { echo "ERROR: tag $(VERSION) already exists"; exit 1; }

.PHONY: release
release: release-preflight ## Tag, publish release, and regenerate Krew manifest. Usage: make release VERSION=v0.0.2 GITHUB_TOKEN=...
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push upstream $(VERSION)
	@$(ENGINE) run \
		--rm \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-e CGO_ENABLED=0 \
		-v `pwd`:/go/src/$(PACKAGE_NAME)$(CONTAINER_MOUNTOPT) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean
	@$(MAKE) --no-print-directory krew-manifest
	@echo ""
	@echo ">>> Release $(VERSION) published: https://github.com/openshift/oc-tnf/releases/tag/$(VERSION)"
	@echo ">>> plugins/tnf.yaml updated. Now commit it on a branch and open a PR."
	@echo ""
	@echo "Recovery if GoReleaser failed mid-run:"
	@echo "  Fix the issue and rerun GoReleaser for the existing tag, or delete and retag:"
	@echo "  git push --delete upstream $(VERSION) && git tag -d $(VERSION)"

.PHONY: release-dry-run
release-dry-run: ## Run GoReleaser in dry-run mode (no publish)
	@$(ENGINE) run \
		--rm \
		-e CGO_ENABLED=0 \
		-v `pwd`:/go/src/$(PACKAGE_NAME)$(CONTAINER_MOUNTOPT) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean --skip=validate --skip=publish

.PHONY: krew-manifest
krew-manifest: ## Regenerate plugins/tnf.yaml from dist/checksums.txt
	@hack/update-krew-manifest.sh

.PHONY: clean
clean: ## Clean build artifacts
	$(RM) -r '$(OUTPUT_DIR)'
	$(RM) -r dist/

.PHONY: version
version: build ## Display version of built binary
	@$(GO_BUILD_BINDIR)/oc-tnf --version
