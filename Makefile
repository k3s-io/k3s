TARGETS := $(shell ls scripts | grep -v \\.sh)
GO_FILES ?= $$(find . -name '*.go' | grep -v generated)


.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.6.0/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
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


EMPTY_VARS := SKIP_VALIDATE
fill-empty-vars:
	$(foreach var,$(EMPTY_VARS),$(if $($(var)),,$(eval export $(var)='')))

.PHONY: local
local: fill-empty-vars
	K3S_SRC=/go/src/github.com/k3s-io/k3s/
	DOCKER_BUILDKIT=1 docker build \
		--build-arg="SKIP_VALIDATE=${SKIP_VALIDATE}" \
		-t k3s-local -f Dockerfile.local .
	docker run -it \
		--privileged -v ${HOME}/.cache/trivy:/root/.cache/trivy \
		-e ${REPO} -e ${TAG} -e ${DRONE_TAG} -e ${SKIP_AIRGAP} -e ${GOCOVER} -e ${DEBUG}\
		-t k3s-local
	docker cp ${K3S_SRC}/bin .
	docker cp ${K3S_SRC}/dist .
	docker cp ${K3S_SRC}/build/out .
	docker cp ${K3S_SRC}/build/static .
	docker cp ${K3S_SRC}/pkg/static .
	docker cp ${K3S_SRC}/pkg/deploy .
	# ENV DAPPER_RUN_ARGS --privileged -v k3s-cache:/go/src/github.com/k3s-io/k3s/.cache -v trivy-cache:/root/.cache/trivy
	# ENV DAPPER_ENV REPO TAG DRONE_TAG IMAGE_NAME SKIP_VALIDATE SKIP_AIRGAP AWS_SECRET_ACCESS_KEY AWS_ACCESS_KEY_ID GITHUB_TOKEN GOLANG GOCOVER DEBUG
	# ENV DAPPER_SOURCE 
	# ENV DAPPER_OUTPUT ./bin ./dist ./build/out ./build/static ./pkg/static ./pkg/deploy

	# ENV DAPPER_DOCKER_SOCKET true
	# ENV HOME ${DAPPER_SOURCE}
	# ENV CROSS true
	# ENV STATIC_BUILD true