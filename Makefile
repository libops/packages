SHELL := /bin/bash
.DEFAULT_GOAL := help
DOCS_IMAGE ?= packages-docs:local
REPOSITORY ?= ghcr.io/libops
PACKAGES_CLI ?= go run ./cmd/packages
DRY_RUN ?= 1
export TARGET ARCH REPOSITORY CONTEXTS PR PACKAGE_REF RELEASE BASE ALL REPORT
export DRY_RUN LOCAL_BUILDER PUBLISH CLEANUP_RUN VALIDATE_ONLY CACHE_GC

.PHONY: help docs-build docs-serve docs-image plan build publish manifest test update revise-base review-lines registry-login registry-clean-pr registry-clean-run registry-clean-cache dev-lint dev-test dev-check dev-test-install dev-generate dev-build builder-list builder-build builder-manifest builder-pin
help: ## Show commands grouped by concern
	@awk 'BEGIN {FS = ":.*## "} /^##@ / {printf "\n%s\n", substr($$0, 5)} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-23s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Documentation (docs-*)
docs-build: ## Build strict Zensical documentation into site/
	docker buildx build -f Dockerfile.docs --target site --output type=local,dest=site .

docs-image: ## Build the pinned documentation preview image
	docker build -f Dockerfile.docs --target tools -t "$(DOCS_IMAGE)" .

docs-serve: docs-image ## Preview documentation at http://localhost:8000
	docker run --rm --user "$$(id -u):$$(id -g)" -e HOME=/tmp -p 127.0.0.1:8000:8000 -v "$(CURDIR):/workspace" "$(DOCS_IMAGE)" serve --dev-addr=0.0.0.0:8000

##@ Software packages
plan: ## List affected packages (BASE=<git ref> or ALL=1)
	@$(PACKAGES_CLI) plan

build: ## Build and load a package carrier locally (TARGET=...)
	$(PACKAGES_CLI) build

publish: ## Build and push one architecture before testing (TARGET=...)
	$(PACKAGES_CLI) publish

manifest: ## Publish the combined architecture manifest (TARGET=...)
	$(PACKAGES_CLI) manifest

test: ## Run upstream and carrier installation tests (TARGET=...)
	$(PACKAGES_CLI) test-package

update: ## Verify upstream source and refresh recipe/hash pins (TARGET=...)
	$(PACKAGES_CLI) update "$(TARGET)"

revise-base: ## Bump packaging revisions for a changed base (Renovate working tree)
	$(PACKAGES_CLI) revise-base

review-lines: ## Detect new runtime lines; REPORT=1 opens a review issue
	$(PACKAGES_CLI) review-lines

##@ Build utility images (builder-*)
builder-list: ## List registered distro/release utility builders
	@$(PACKAGES_CLI) builder-plan

builder-build: ## Build a utility image; PUBLISH=1 pushes one architecture
	$(PACKAGES_CLI) builder-build

builder-manifest: ## Publish the combined utility image manifest
	$(PACKAGES_CLI) builder-manifest

builder-pin: ## Record utility image digest; REPORT=1 opens a pin PR
	$(PACKAGES_CLI) pin-builder "$(TARGET)" "$(DIGEST)" $(if $(filter 1,$(REPORT)),--pull-request)

##@ Registry access and retention (registry-*)
registry-login: ## Authenticate to REPOSITORY using supplied or prompted credentials
	$(PACKAGES_CLI) login

registry-clean-cache: ## Quarantine orphaned BuildKit caches; delete after seven days (DRY_RUN=0)
	CACHE_GC=1 $(PACKAGES_CLI) cleanup

registry-clean-run: ## Clean disposable candidates from a completed main workflow (CLEANUP_RUN=...)
	$(PACKAGES_CLI) cleanup

registry-clean-pr: ## Preview closed-PR GHCR cleanup; PR=... DRY_RUN=0 deletes
	$(PACKAGES_CLI) cleanup

##@ Repository development and CI checks (dev-*)
dev-lint: ## Vet Go, check formatting and lint Actions workflows
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -ignore 'unexpected key "queue" for "concurrency" section'
	go vet ./...
	$(PACKAGES_CLI) fmt-check

dev-test: ## Run Go regression tests, including cleanup safeguards
	mkdir -p build
	go test -coverprofile=build/coverage.out ./...
	$(PACKAGES_CLI) check-coverage build/coverage.out

dev-test-install: ## Test carrier selection against newer distro packages (Docker required)
	$(PACKAGES_CLI) test-install

dev-check: dev-lint dev-test ## Check code, generated configuration and source pins
	$(PACKAGES_CLI) generate --check
	$(PACKAGES_CLI) check-pins

dev-generate: ## Generate Actions and Renovate from packages.json
	$(PACKAGES_CLI) generate

dev-build: ## Compile the shared Go CLI into build/packages
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -o build/packages ./cmd/packages
