TARGETS := $(shell ls scripts | grep -v \\.sh)
GO_FILES ?= $$(find . -name '*.go')
SHELL := /bin/bash

.PHONY: deps
deps:
	go mod tidy

release:
	./scripts/release.sh

.DEFAULT_GOAL := ci

.PHONY: ci
ci:  binary image airgap

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


.PHONY: validate
validate:
	DOCKER_BUILDKIT=1 docker build \
		--build-arg="SKIP_VALIDATE=$(SKIP_VALIDATE)" \
		--build-arg="DEBUG=$(DEBUG)" \
		--progress=plain \
		-f Dockerfile --target=validate .

.PHONY: binary
binary:
	@echo "INFO: Building K3s binaries and assets..."
	. ./scripts/git_version.sh && \
	DOCKER_BUILDKIT=1 docker build \
		--build-arg "GIT_TAG=$$GIT_TAG" \
		--build-arg "TREE_STATE=$$TREE_STATE" \
		--build-arg "COMMIT=$$COMMIT" \
		--build-arg "DIRTY=$$DIRTY" \
		--build-arg="GOCOVER=$(GOCOVER)" \
		--build-arg="GOOS=$(GOOS)" \
		--build-arg="DEBUG=$(DEBUG)" \
		-f Dockerfile --target=result --output=. .

.PHONY: image
image: binary
	@echo "INFO: Building K3s image..."
	./scripts/package-image

.PHONY: airgap
airgap: 
	@echo "INFO: Building K3s airgap tarball..."
	./scripts/package-airgap

BRANCH := $(shell git rev-parse --abbrev-ref HEAD | sed 's/\//-/g')
in-docker-%: ## Advanced: wraps any script in Docker environment, for example: in-docker-package-cli
	mkdir -p ./bin/ ./dist ./build
	docker buildx build -t k3s:$(BRANCH) --target infra -f Dockerfile .
	docker run --privileged --rm --network host \
		-v $${PWD}:/go/src/github.com/k3s-io/k3s -v /var/run/docker.sock:/var/run/docker.sock -v /tmp:/tmp -v k3s-pkg:/go/pkg -v k3s-cache:/root/.cache/go-build \
		-e GODEBUG -e CI -e GOCOVER -e REPO -e TAG -e GITHUB_ACTION_TAG -e KUBERNETES_VERSION -e IMAGE_NAME -e AWS_SECRET_ACCESS_KEY -e AWS_ACCESS_KEY_ID \
		-e DOCKER_PASSWORD -e DOCKER_USERNAME -e GH_TOKEN -e SKIP_VALIDATE -e SKIP_IMAGE -e SKIP_AIRGAP -e GITHUB_TOKEN \
		-e GIT_CONFIG_COUNT=1 -e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0=/go/src/github.com/k3s-io/k3s \
		k3s:$(BRANCH) ./scripts/$*