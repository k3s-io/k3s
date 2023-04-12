TARGETS := $(shell ls scripts | grep -v \\.sh)

.dapper:
	@if command -v dapper; then \
			ln -s $$(command -v dapper) .dapper; \
		else \
			echo Downloading dapper; \
			curl -sLfo .dapper.tmp https://releases.rancher.com/dapper/v0.6.0/dapper-$$(uname -s)-$$(uname -m) \
			chmod +x .dapper.tmp; \
			./.dapper.tmp -v; \
			mv .dapper.tmp .dapper; \
		fi

$(TARGETS): .dapper
	./.dapper $@

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
