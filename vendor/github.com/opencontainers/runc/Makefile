CONTAINER_ENGINE := docker
GO ?= go

PREFIX ?= /usr/local
BINDIR := $(PREFIX)/sbin
MANDIR := $(PREFIX)/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
RUNC_IMAGE := runc_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
PROJECT := github.com/opencontainers/runc
BUILDTAGS ?= seccomp
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT ?= $(if $(shell git status --porcelain --untracked-files=no),"$(COMMIT_NO)-dirty","$(COMMIT_NO)")
VERSION := $(shell cat ./VERSION)

# TODO: rm -mod=vendor once go 1.13 is unsupported
ifneq ($(GO111MODULE),off)
	MOD_VENDOR := "-mod=vendor"
endif
ifeq ($(shell $(GO) env GOOS),linux)
	ifeq (,$(filter $(shell $(GO) env GOARCH),mips mipsle mips64 mips64le ppc64))
		ifeq (,$(findstring -race,$(EXTRA_FLAGS)))
			GO_BUILDMODE := "-buildmode=pie"
		endif
	endif
endif
GO_BUILD := $(GO) build -trimpath $(MOD_VENDOR) $(GO_BUILDMODE) $(EXTRA_FLAGS) -tags "$(BUILDTAGS)" \
	-ldflags "-X main.gitCommit=$(COMMIT) -X main.version=$(VERSION) $(EXTRA_LDFLAGS)"
GO_BUILD_STATIC := CGO_ENABLED=1 $(GO) build -trimpath $(MOD_VENDOR) $(EXTRA_FLAGS) -tags "$(BUILDTAGS) netgo osusergo" \
	-ldflags "-w -extldflags -static -X main.gitCommit=$(COMMIT) -X main.version=$(VERSION) $(EXTRA_LDFLAGS)"

.DEFAULT: runc

runc:
	$(GO_BUILD) -o runc .

all: runc recvtty

recvtty:
	$(GO_BUILD) -o contrib/cmd/recvtty/recvtty ./contrib/cmd/recvtty

static:
	$(GO_BUILD_STATIC) -o runc .
	$(GO_BUILD_STATIC) -o contrib/cmd/recvtty/recvtty ./contrib/cmd/recvtty

release:
	script/release.sh -r release/$(VERSION) -v $(VERSION)

dbuild: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make clean all

lint:
	golangci-lint run ./...

man:
	man/md2man-all.sh

runcimage:
	$(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_BUILD_FLAGS) -t $(RUNC_IMAGE) .

test: unittest integration rootlessintegration

localtest: localunittest localintegration localrootlessintegration

unittest: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localunittest TESTFLAGS=$(TESTFLAGS)

localunittest: all
	$(GO) test $(MOD_VENDOR) -timeout 3m -tags "$(BUILDTAGS)" $(TESTFLAGS) -v ./...

integration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localintegration TESTPATH=$(TESTPATH)

localintegration: all
	bats -t tests/integration$(TESTPATH)

rootlessintegration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		-e ROOTLESS_TESTPATH \
		$(RUNC_IMAGE) make localrootlessintegration

localrootlessintegration: all
	tests/rootless.sh

shell: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-ti --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) bash

install:
	install -D -m0755 runc $(DESTDIR)$(BINDIR)/runc

install-bash:
	install -D -m0644 contrib/completions/bash/runc $(DESTDIR)$(PREFIX)/share/bash-completion/completions/runc

install-man: man
	install -d -m 755 $(DESTDIR)$(MANDIR)/man8
	install -D -m 644 man/man8/*.8 $(DESTDIR)$(MANDIR)/man8

clean:
	rm -f runc runc-*
	rm -f contrib/cmd/recvtty/recvtty
	rm -rf release
	rm -rf man/man8

cfmt: C_SRC=$(shell git ls-files '*.c' | grep -v '^vendor/')
cfmt:
	indent -linux -l120 -il0 -ppi2 -cp1 -T size_t -T jmp_buf $(C_SRC)

shellcheck:
	shellcheck tests/integration/*.bats tests/integration/*.sh tests/*.sh script/release.sh
	# TODO: add shellcheck for more sh files

shfmt:
	shfmt -ln bats -d -w tests/integration/*.bats
	shfmt -ln bash -d -w man/*.sh script/* tests/*.sh tests/integration/*.bash

vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

verify-dependencies: vendor
	@test -z "$$(git status --porcelain -- go.mod go.sum vendor/)" \
		|| (echo -e "git status:\n $$(git status -- go.mod go.sum vendor/)\nerror: vendor/, go.mod and/or go.sum not up to date. Run \"make vendor\" to update"; exit 1) \
		&& echo "all vendor files are up to date."

cross: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-e BUILDTAGS="$(BUILDTAGS)" --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localcross

localcross:
	CGO_ENABLED=1 GOARCH=arm GOARM=6 CC=arm-linux-gnueabi-gcc   $(GO_BUILD) -o runc-armel .
	CGO_ENABLED=1 GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc $(GO_BUILD) -o runc-armhf .
	CGO_ENABLED=1 GOARCH=arm64 CC=aarch64-linux-gnu-gcc         $(GO_BUILD) -o runc-arm64 .
	CGO_ENABLED=1 GOARCH=ppc64le CC=powerpc64le-linux-gnu-gcc   $(GO_BUILD) -o runc-ppc64le .

.PHONY: runc all recvtty static release dbuild lint man runcimage \
	test localtest unittest localunittest integration localintegration \
	rootlessintegration localrootlessintegration shell install install-bash \
	install-man clean cfmt shfmt shellcheck \
	vendor verify-dependencies cross localcross
