MODULE := $(shell basename $$PWD)

.PHONY: init config apply destroy clean test

init:
	@scripts/perf init

config:
	@scripts/perf config

apply:
	@scripts/perf apply

destroy:
	@scripts/perf destroy

clean:
	@scripts/perf clean

test:
	@scripts/test test_load
