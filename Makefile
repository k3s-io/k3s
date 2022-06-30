TARGETS := $(shell ls scripts | grep -v \\.sh)
mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
BASE := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.5.7/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

ifndef SKIP_DAPPER
$(TARGETS): .dapper
	./.dapper $@
endif

.PHONY: deps
deps:
	go mod vendor
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

.PHONY: source-tarball
source-tarball: deps
	./scripts/download
	cd ..; tar -czf $(BASE).tar.gz $(BASE); mv $(BASE).tar.gz $(BASE); cd $(BASE)
