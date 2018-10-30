mkfile_path	:= $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir	:= $(patsubst %/,%,$(dir $(mkfile_path)))

MESOS_API_VERSION := v1
API_PKG    := ./api/${MESOS_API_VERSION}/lib
API_VENDOR := ${current_dir}/api/${MESOS_API_VERSION}/vendor
CMD_PKG    := ./api/${MESOS_API_VERSION}/cmd
CMD_VENDOR := ./api/${MESOS_API_VERSION}/cmd/vendor

PROTO_PATH := ${GOPATH}/src/:${API_VENDOR}/:.
PROTO_PATH := ${PROTO_PATH}:${API_VENDOR}/github.com/gogo/protobuf/protobuf
PROTO_PATH := ${PROTO_PATH}:${API_VENDOR}/github.com/gogo/protobuf/gogoproto

PACKAGES ?= $(shell go list ${API_PKG}/...|grep -v vendor)
TEST_DIRS ?= $(sort $(dir $(shell find ${API_PKG} -name '*_test.go' | grep -v vendor)))
BINARIES ?= $(shell go list -f "{{.Name}} {{.ImportPath}}" ${CMD_PKG}/...|grep -v -e vendor|grep -e ^main|cut -f2 -d' ')
TEST_FLAGS ?= -race
COVERAGE_TARGETS = ${TEST_DIRS:%/=%.cover}

GO_VERSION := $(shell go version|cut -f3 -d' '|dd bs=1 count=6 2>/dev/null)

.PHONY: all
all: test

.PHONY: install
install: INSTALL_FLAGS +=
install:
	go install $(INSTALL_FLAGS) $(BINARIES)

.PHONY: test
test:
	go $@ $(TEST_FLAGS) $(TEST_DIRS)

.PHONY: test-verbose
test-verbose: TEST_FLAGS += -v
test-verbose: test

.PHONY: coverage $(COVERAGE_TARGETS)
coverage: REPORT=_output/coverage.out
coverage: COVER_PACKAGE = $(shell go list ${API_PKG}/...|egrep -v 'vendor|cmd'|tr '\n' ','|sed -e 's/,$$//')
coverage: TEST_FLAGS = -v -cover -coverpkg=$(COVER_PACKAGE)
coverage: $(COVERAGE_TARGETS)
	echo "mode: set" >$(REPORT) && cat _output/*.cover | grep -v mode: | sort -r | \
		awk '{if($$1 != last) {print $$0;last=$$1}}' >> $(REPORT)
$(COVERAGE_TARGETS): %.cover :
	mkdir -p _output && go test ./$* $(TEST_FLAGS) -coverprofile=_output/$(subst /,___,$@)

.PHONY: vet
vet:
	go $@ $(PACKAGES) $(BINARIES)

.PHONY: codecs
codecs: protobufs ffjson

.PHONY: protobufs-requirements
protobufs-requirements: REQUIRED_PROTOC_BINARIES = protoc-gen-gogo
protobufs-requirements:
	@for i in ${REQUIRED_PROTOC_BINARIES}; do which $$i >/dev/null || { echo "failed to locate binary: $$i"; exit 1; }; done

.PHONY: protobufs
protobufs: protobufs-requirements clean-protobufs
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. *.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./scheduler/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./executor/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./agent/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./quota/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./allocator/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./maintenance/*.proto)
	(cd ${API_PKG}; protoc --proto_path="${PROTO_PATH}" --gogo_out=. ./master/*.proto)

.PHONY: clean-protobufs
clean-protobufs:
	(cd ${API_PKG}; rm -f *.pb.go */*.pb.go)

.PHONY: ffjson
ffjson: clean-ffjson
	(cd ${API_PKG}; ffjson *.pb.go)
	(cd ${API_PKG}; ffjson scheduler/*.pb.go)
	(cd ${API_PKG}; ffjson executor/*.pb.go)
	(cd ${API_PKG}; ffjson agent/*.pb.go)
	(cd ${API_PKG}; ffjson quota/*.pb.go)
	(cd ${API_PKG}; ffjson allocator/*.pb.go)
	(cd ${API_PKG}; ffjson maintenance/*.pb.go)
	(cd ${API_PKG}; ffjson master/*.pb.go)

.PHONY: clean-ffjson
clean-ffjson:
	(cd ${API_PKG}; rm -f *ffjson.go */*ffjson.go)

.PHONY: sync
sync:
	@which govendor >/dev/null || { echo "govendor not found in $$PATH"; exit 1; }
	(cd ${API_VENDOR}; govendor sync)
	(cd ${CMD_VENDOR}; govendor sync)

.PHONY: generate
generate: GENERATE_PACKAGES = ./api/v1/lib/extras/executor/eventrules ./api/v1/lib/extras/executor/callrules ./api/v1/lib/extras/scheduler/eventrules ./api/v1/lib/extras/scheduler/callrules ./api/v1/lib/executor/events ./api/v1/lib/executor/calls ./api/v1/lib/scheduler/events ./api/v1/lib/scheduler/calls ./api/v1/lib/agent/calls ./api/v1/lib/master/calls ./api/v1/lib/httpcli/httpagent ./api/v1/lib/httpcli/httpmaster ./api/v1/lib/httpcli/httpexec
generate:
	go generate -x ${GENERATE_PACKAGES}

GOPKG		:= github.com/mesos/mesos-go
GOPKG_DIRNAME	:= $(shell dirname $(GOPKG))
UID		?= $(shell id -u $$USER)
GID		?= $(shell id -g $$USER)

DOCKER_IMAGE_REPO       ?= jdef
DOCKER_IMAGE_NAME       ?= example-scheduler-httpv1
DOCKER_IMAGE_VERSION    ?= latest
DOCKER_IMAGE_TAG        ?= $(DOCKER_IMAGE_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_VERSION)

GOLDFLAGS	= -X $(GOPKG)/api/${MESOS_API_VERSION}/cmd.DockerImageTag=$(DOCKER_IMAGE_TAG)

BUILD_STEP	:= 'ln -s /src /go/src/$(GOPKG) && cd /go/src/$(GOPKG) && go install -ldflags "$(GOLDFLAGS)" $(BINARIES)'
COPY_STEP	:= 'cp /go/bin/* /src/_output/ && chown $(UID):$(GID) /src/_output/*'

# required for docker/Makefile
export DOCKER_IMAGE_TAG

.PHONY: docker
docker:
	mkdir -p _output
	test -n "$(UID)" || (echo 'ERROR: $$UID is undefined'; exit 1)
	test -n "$(GID)" || (echo 'ERROR: $$GID is undefined'; exit 1)
	docker run --rm -v "$$PWD":/src -w /go/src/$(GOPKG_DIRNAME) golang:1.10.1-alpine sh -c $(BUILD_STEP)' && '$(COPY_STEP)
	make -C api/${MESOS_API_VERSION}/docker

.PHONY: coveralls
coveralls: IGNORE_FILES = $(shell { find api/v1/cmd -type d ; ls api/v1/lib{,/scheduler,/executor,/agent,/quota,/allocator,/maintenance,/master}/*.pb{,_ffjson}.go ; find api/v0 -type d; } | tr '\n' ,)
coveralls: SHELL := /bin/bash
coveralls:
	test "$(TRAVIS)" = "" || rm -rf $$HOME/gopath/pkg
	$(MAKE) coverage
	$$HOME/gopath/bin/goveralls -service=travis-ci -coverprofile=_output/coverage.out -ignore=$(IGNORE_FILES)

# re-generate protobuf and json code, check that there are no differences w/ respect to what's been checked in
.PHONY: validate-protobufs
ifeq ($(GO_VERSION),go1.10)
validate-protobufs: SHELL := /bin/bash
validate-protobufs:
	(cd api/v1; govendor install +vendor,program) && $(MAKE) -s protobufs ffjson && [[ `{ git status --porcelain || echo "failed"; } | tee /tmp/status | wc -l` = "0" ]] || { cat /tmp/status; git diff; false; }
else
validate-protobufs:
endif
