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


# Base path used to install.
DESTDIR ?= /usr/local

COMMANDS=ctd-decoder ctr-enc

BINARIES=$(addprefix bin/,$(COMMANDS))

.PHONY: check build ctd-decoder

all: build

build: $(BINARIES)

FORCE:

bin/ctd-decoder: cmd/ctd-decoder FORCE
	go build -o $@ -v ./cmd/ctd-decoder/

bin/ctr-enc: cmd/ctr FORCE
	go build -o $@ -v ./cmd/ctr/

check:
	@echo "$@"
	@golangci-lint run
	@script/check_format.sh

install:
	@echo "$@"
	@mkdir -p $(DESTDIR)/bin
	@install $(BINARIES) $(DESTDIR)/bin

uninstall:
	@echo "$@"
	@rm -f $(addprefix $(DESTDIR)/bin/,$(notdir $(BINARIES)))

clean:
	@echo "$@"
	@rm -f $(BINARIES)

test:
	@echo "$@"
	@go test ./...
