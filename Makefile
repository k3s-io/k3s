TARGETS := $(shell ls scripts | grep -v \\.sh)
GO_FILES ?= $$(find . -name '*.go' | grep -v generated)
SHELL := /bin/bash


.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.6.0/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

.PHONY: docker.sock
docker.sock:
	while ! docker version 1>/dev/null; do sleep 1; done

$(TARGETS): .dapper docker.sock
	./.dapper $@

.PHONY: deps
deps:
	go mod tidy

release:
	./scripts/release.sh

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)

build/data:
	mkdir -p $@

.PHONY: binary-size-check
binary-size-check:
	scripts/binary_size_check.sh

.PHONY: image-scan
image-scan:
	scripts/image_scan.sh $(IMAGE)

format:
	gofmt -s -l -w $(GO_FILES)
	goimports -w $(GO_FILES)

.PHONY: local
local:
	DOCKER_BUILDKIT=1 docker build \
		--build-arg="REPO TAG GITHUB_TOKEN GOLANG GOCOVER DEBUG" \
		-t k3s-local -f Dockerfile.local --output=. .
