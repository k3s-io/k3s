MODULE := $(shell basename $$PWD)

.PHONY: init config apply destroy clean test info

init:
	@scripts/perf init

config:
	@scripts/perf config

plan:
	@scripts/perf plan

apply:
	@scripts/perf apply

destroy:
	@scripts/perf destroy

clean:
	@scripts/perf clean

test:
	@scripts/test test_load

info:
	@scripts/perf info
