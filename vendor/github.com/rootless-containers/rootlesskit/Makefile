GO=go
GO_FILES=$(shell find . -name *.go)
BINARIES=rootlesskit rootlessctl rootlesskit-docker-proxy

.PHONY: all
all: $(addprefix bin/, $(BINARIES))

.PHONY: clean
clean:
	$(RM) -r bin/

bin/rootlesskit: $(GO_FILES)
	$(GO) build -o $@ -v github.com/rootless-containers/rootlesskit/cmd/rootlesskit

bin/rootlessctl: $(GO_FILES)
	$(GO) build -o $@ -v github.com/rootless-containers/rootlesskit/cmd/rootlessctl

bin/rootlesskit-docker-proxy: $(GO_FILES)
	$(GO) build -o $@ -v github.com/rootless-containers/rootlesskit/cmd/rootlesskit-docker-proxy

.PHONY: test
test:
	./hack/test.sh
