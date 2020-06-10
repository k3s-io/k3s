# Copyright 2018 The containerd Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO := go
GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)
WHALE = "🇩"
ONI = "👹"
EPOCH_TEST_COMMIT := f9e02affccd51702191e5312665a16045ffef8ab
PROJECT := github.com/containerd/cri
BINDIR := ${DESTDIR}/usr/local/bin
BUILD_DIR := _output
# VERSION is derived from the current commit for HEAD. Version is used
# to set/overide the containerd version in vendor/github.com/containerd/containerd/version.
VERSION := $(shell git rev-parse --short HEAD)
TARBALL_PREFIX := cri-containerd
TARBALL := $(TARBALL_PREFIX)-$(VERSION).$(GOOS)-$(GOARCH).tar.gz
BUILD_TAGS := seccomp apparmor
# Add `-TEST` suffix to indicate that all binaries built from this repo are for test.
GO_LDFLAGS := -X $(PROJECT)/vendor/github.com/containerd/containerd/version.Version=$(VERSION)-TEST
SOURCES := $(shell find cmd/ pkg/ vendor/ -name '*.go')
PLUGIN_SOURCES := $(shell ls *.go)
INTEGRATION_SOURCES := $(shell find integration/ -name '*.go')

all: binaries

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9._-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

verify: lint gofmt boiler check-vendor ## execute the source code verification tools

version: ## print current cri plugin release version
	@echo $(VERSION)

lint:
	@echo "$(WHALE) $@"
	golangci-lint run --skip-files .*_test.go

gofmt:
	@echo "$(WHALE) $@"
	@./hack/verify-gofmt.sh

boiler:
	@echo "$(WHALE) $@"
	@./hack/verify-boilerplate.sh

check-vendor:
	@echo "$(WHALE) $@"
	@./hack/verify-vendor.sh

.PHONY: sort-vendor sync-vendor update-vendor

sort-vendor:
	@echo "$(WHALE) $@"
	@./hack/sort-vendor.sh

sync-vendor:
	@echo "$(WHALE) $@ from containerd"
	@./hack/sync-vendor.sh

update-vendor: sync-vendor sort-vendor ## Syncs containerd/vendor.conf -> vendor.conf and sorts vendor.conf
	@echo "$(WHALE) $@"

$(BUILD_DIR)/containerd: $(SOURCES) $(PLUGIN_SOURCES)
	@echo "$(WHALE) $@"
	$(GO) build -o $@ \
		-tags '$(BUILD_TAGS)' \
		-ldflags '$(GO_LDFLAGS)' \
		-gcflags '$(GO_GCFLAGS)' \
		$(PROJECT)/cmd/containerd

test: ## unit test
	@echo "$(WHALE) $@"
	$(GO) test -timeout=10m -race ./pkg/... \
		-tags '$(BUILD_TAGS)' \
	        -ldflags '$(GO_LDFLAGS)' \
		-gcflags '$(GO_GCFLAGS)'

$(BUILD_DIR)/integration.test: $(INTEGRATION_SOURCES)
	@echo "$(WHALE) $@"
	$(GO) test -c $(PROJECT)/integration -o $(BUILD_DIR)/integration.test

test-integration: $(BUILD_DIR)/integration.test binaries ## integration test
	@echo "$(WHALE) $@"
	@./hack/test-integration.sh

test-cri: binaries ## critools CRI validation test
	@echo "$(WHALE) $@"
	@./hack/test-cri.sh

test-e2e-node: binaries ## e2e node test
	@echo "$(WHALE) $@"
	@VERSION=$(VERSION) ./hack/test-e2e-node.sh

clean: ## cleanup binaries
	@echo "$(WHALE) $@"
	@rm -rf $(BUILD_DIR)/*

binaries: $(BUILD_DIR)/containerd ## build a customized containerd (same result as make containerd)
	@echo "$(WHALE) $@"

static-binaries: GO_LDFLAGS += -extldflags "-fno-PIC -static"
static-binaries: $(BUILD_DIR)/containerd ## build static containerd
	@echo "$(WHALE) $@"

containerd: $(BUILD_DIR)/containerd ## build a customized containerd with CRI plugin for testing
	@echo "$(WHALE) $@"

install-containerd: containerd ## installs customized containerd to system location
	@echo "$(WHALE) $@"
	@install -D -m 755 $(BUILD_DIR)/containerd $(BINDIR)/containerd

install: install-containerd ## installs customized containerd to system location
	@echo "$(WHALE) $@"

uninstall: ## remove containerd from system location
	@echo "$(WHALE) $@"
	@rm -f $(BINDIR)/containerd

$(BUILD_DIR)/$(TARBALL): static-binaries vendor.conf
	@BUILD_DIR=$(BUILD_DIR) TARBALL=$(TARBALL) VERSION=$(VERSION) ./hack/release.sh

release: $(BUILD_DIR)/$(TARBALL) ## build release tarball

push: $(BUILD_DIR)/$(TARBALL) ## push release tarball to GCS
	@echo "$(WHALE) $@"
	@BUILD_DIR=$(BUILD_DIR) TARBALL=$(TARBALL) VERSION=$(VERSION) ./hack/push.sh

proto: ## update protobuf of the cri plugin api
	@echo "$(WHALE) $@"
	@API_PATH=pkg/api/v1 hack/update-proto.sh
	@API_PATH=pkg/api/runtimeoptions/v1 hack/update-proto.sh

.PHONY: install.deps

install.deps: ## install dependencies of cri (default 'seccomp apparmor' BUILDTAGS for runc build)
	@echo "$(WHALE) $@"
	@./hack/install/install-deps.sh

.PHONY: .gitvalidation
# When this is running in travis, it will only check the travis commit range.
# When running outside travis, it will check from $(EPOCH_TEST_COMMIT)..HEAD.
.gitvalidation:
	@echo "$(WHALE) $@"
ifeq ($(TRAVIS),true)
	git-validation -q -run DCO,short-subject
else
	git-validation -v -run DCO,short-subject -range $(EPOCH_TEST_COMMIT)..HEAD
endif

.PHONY: install.tools .install.gitvalidation .install.golangci-lint .install.vndr

install.tools: .install.gitvalidation .install.golangci-lint .install.vndr ## install tools used by verify
	@echo "$(WHALE) $@"

.install.gitvalidation:
	@echo "$(WHALE) $@"
	$(GO) get -u github.com/vbatts/git-validation

.install.golangci-lint:
	@echo "$(WHALE) $@"
	$(GO) get -d github.com/golangci/golangci-lint/cmd/golangci-lint
	@cd $(GOPATH)/src/github.com/golangci/golangci-lint/cmd/golangci-lint; \
		git checkout v1.18.0; \
		go install

.install.vndr:
	@echo "$(WHALE) $@"
	$(GO) get -u github.com/LK4D4/vndr

.PHONY: \
	binaries \
	static-binaries \
	containerd \
	install-containerd \
	release \
	push \
	boiler \
	clean \
	default \
	gofmt \
	help \
	install \
	lint \
	test \
	test-integration \
	test-cri \
	test-e2e-node \
	uninstall \
	version \
	proto \
	check-vendor
