#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

DESTDIR ?= /usr/local

VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
VERSION_TRIMMED := $(VERSION:v%=%)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)

PKG_MAIN := github.com/containerd/fuse-overlayfs-snapshotter/cmd/containerd-fuse-overlayfs-grpc
PKG_VERSION := github.com/containerd/fuse-overlayfs-snapshotter/cmd/containerd-fuse-overlayfs-grpc/version

GO ?= go
export GO_BUILD=GO111MODULE=on CGO_ENABLED=0 $(GO) build -ldflags "-s -w -X $(PKG_VERSION).Version=$(VERSION) -X $(PKG_VERSION).Revision=$(REVISION)"

bin/containerd-fuse-overlayfs-grpc:
	$(GO_BUILD) -o $@ $(PKG_MAIN)

install:
	install bin/containerd-fuse-overlayfs-grpc $(DESTDIR)/bin

uninstall:
	rm -f $(DESTDIR)/bin/containerd-fuse-overlayfs-grpc

clean:
	rm -rf bin

test:
	DOCKER_BUILDKIT=1 docker build -t containerd-fuse-overlayfs-test --build-arg FUSEOVERLAYFS_COMMIT=${FUSEOVERLAYFS_COMMIT} .
	docker run --rm containerd-fuse-overlayfs-test fuse-overlayfs -V
	docker run --rm --security-opt seccomp=unconfined --security-opt apparmor=unconfined --device /dev/fuse containerd-fuse-overlayfs-test
	docker rmi containerd-fuse-overlayfs-test

_test:
	go test -exec rootlesskit -test.v -test.root

TAR_FLAGS=--transform 's/.*\///g' --owner=0 --group=0

artifacts: clean
	mkdir -p _output
	GOOS=linux GOARCH=amd64       make
	tar $(TAR_FLAGS) -czvf _output/containerd-fuse-overlayfs-$(VERSION_TRIMMED)-linux-amd64.tar.gz  bin/*
	GOOS=linux GOARCH=arm64       make
	tar $(TAR_FLAGS) -czvf _output/containerd-fuse-overlayfs-$(VERSION_TRIMMED)-linux-arm64.tar.gz  bin/*
	GOOS=linux GOARCH=arm GOARM=7 make
	tar $(TAR_FLAGS) -czvf _output/containerd-fuse-overlayfs-$(VERSION_TRIMMED)-linux-arm-v7.tar.gz bin/*
	GOOS=linux GOARCH=ppc64le     make
	tar $(TAR_FLAGS) -czvf _output/containerd-fuse-overlayfs-$(VERSION_TRIMMED)-linux-ppc64le.tar.gz  bin/*
	GOOS=linux GOARCH=s390x       make
	tar $(TAR_FLAGS) -czvf _output/containerd-fuse-overlayfs-$(VERSION_TRIMMED)-linux-s390x.tar.gz  bin/*

.PHONY: bin/containerd-fuse-overlayfs-grpc install uninstall clean test _test artifacts
