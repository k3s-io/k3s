DESTDIR ?= /usr/local

bin/containerd-fuse-overlayfs-grpc:
	go build -o $@ ./cmd/containerd-fuse-overlayfs-grpc

install:
	install bin/containerd-fuse-overlayfs-grpc $(DESTDIR)/bin

uninstall:
	rm -f $(DESTDIR)/bin/containerd-fuse-overlayfs-grpc

clean:
	rm -rf bin

test:
	DOCKER_BUILDKIT=1 docker build -t containerd-fuse-overlayfs-test .
	docker run --rm --security-opt seccomp=unconfined --security-opt apparmor=unconfined --device /dev/fuse containerd-fuse-overlayfs-test
	docker rmi containerd-fuse-overlayfs-test

_test:
	go test -exec rootlesskit -test.v -test.root

.PHONY: bin/containerd-fuse-overlayfs-grpc install uninstall clean test _test
