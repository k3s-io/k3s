PREFIX ?= $(DESTDIR)/usr
BINDIR ?= $(DESTDIR)/usr/bin
TAP ?= tap

BUILDTAGS=
RUNTIME ?= runc
COMMIT=$(shell git rev-parse HEAD 2> /dev/null || true)
VERSION := ${shell cat ./VERSION}
VALIDATION_TESTS ?= $(patsubst %.go,%.t,$(wildcard validation/*.go))

all: tool runtimetest validation-executables

tool:
	go build -tags "$(BUILDTAGS)" -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION}" -o oci-runtime-tool ./cmd/oci-runtime-tool

.PHONY: runtimetest
runtimetest:
	CGO_ENABLED=0 go build -installsuffix cgo -tags "$(BUILDTAGS)" -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION}" -o runtimetest ./cmd/runtimetest

.PHONY: man
man:
	go-md2man -in "man/oci-runtime-tool.1.md" -out "oci-runtime-tool.1"
	go-md2man -in "man/oci-runtime-tool-generate.1.md" -out "oci-runtime-tool-generate.1"
	go-md2man -in "man/oci-runtime-tool-validate.1.md" -out "oci-runtime-tool-validate.1"

install: man
	install -d -m 755 $(BINDIR)
	install -m 755 oci-runtime-tool $(BINDIR)
	install -d -m 755 $(PREFIX)/share/man/man1
	install -m 644 *.1 $(PREFIX)/share/man/man1
	install -d -m 755 $(PREFIX)/share/bash-completion/completions
	install -m 644 completions/bash/oci-runtime-tool $(PREFIX)/share/bash-completion/completions

uninstall:
	rm -f $(BINDIR)/oci-runtime-tool
	rm -f $(PREFIX)/share/man/man1/oci-runtime-tool*.1
	rm -f $(PREFIX)/share/bash-completion/completions/oci-runtime-tool

clean:
	rm -f oci-runtime-tool runtimetest *.1 $(VALIDATION_TESTS)

localvalidation:
	@for EXECUTABLE in runtimetest $(VALIDATION_TESTS); \
	do \
		if test ! -x "$${EXECUTABLE}"; \
		then \
			echo "missing test executable $${EXECUTABLE}; run 'make runtimetest validation-executables'" >&2; \
			exit 1; \
		fi; \
	done
	RUNTIME=$(RUNTIME) $(TAP) $(VALIDATION_TESTS)

.PHONY: validation-executables
validation-executables: $(VALIDATION_TESTS)

.PRECIOUS: $(VALIDATION_TESTS)
.PHONY: $(VALIDATION_TESTS)
$(VALIDATION_TESTS): %.t: %.go
	go build -tags "$(BUILDTAGS)" ${TESTFLAGS} -o $@ $<

.PHONY: test .gofmt .govet .golint

PACKAGES = $(shell go list ./... | grep -v vendor)
test: .gofmt .govet .golint .gotest

.gofmt:
	OUT=$$(go fmt $(PACKAGES)); if test -n "$${OUT}"; then echo "$${OUT}" && exit 1; fi

.govet:
	go vet -x $(PACKAGES)

.golint:
	golint -set_exit_status $(PACKAGES)

UTDIRS = ./filepath/... ./validate/...
.gotest:
	go test $(UTDIRS)
