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

.PHONY: release-dry-run
release-dry-run: ## Run GoReleaser in dry-run mode (no publish)
	@$(ENGINE) run \
		--rm \
		-e CGO_ENABLED=0 \
		-v `pwd`:/go/src/$(PACKAGE_NAME)$(CONTAINER_MOUNTOPT) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean --skip=validate --skip=publish

.PHONY: release
release: ## Create a release with GoReleaser (requires GITHUB_TOKEN)
	@$(ENGINE) run \
		--rm \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-e CGO_ENABLED=0 \
		-v `pwd`:/go/src/$(PACKAGE_NAME)$(CONTAINER_MOUNTOPT) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean

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
