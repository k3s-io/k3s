GO_FILES ?= $$(find . -name '*.go')
SHELL := /bin/bash


.PHONY: docker.sock
docker.sock:
	while ! docker version 1>/dev/null; do sleep 1; done

.PHONY: deps
deps:
	go mod tidy

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

.PHONY tag-image-latest
	scripts/tag-image-latest

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

.PHONY: ci
ci:  binary image airgap
