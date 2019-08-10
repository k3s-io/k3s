FILES:=$(wildcard out/*.go)

.PHONY: check
check: errcheck golint $(FILES:.go=.checked)

out/%.checked: out/%.go
	errcheck $<
	go vet --all $<
	golint $<

$(GOPATH)/bin/go-bindata: $(wildcard ../*.go) $(wildcard ../**/*.go)
	go install ../...

out/compress-memcopy.go: 		 $(wildcard in/**/*) $(GOPATH)/bin/go-bindata
	$(GOPATH)/bin/go-bindata                        -o $@ in/...

out/compress-nomemcopy.go: 	 $(wildcard in/**/*) $(GOPATH)/bin/go-bindata
	$(GOPATH)/bin/go-bindata -nomemcopy             -o $@ in/...

out/debug.go: 							 $(wildcard in/**/*) $(GOPATH)/bin/go-bindata
	$(GOPATH)/bin/go-bindata -debug                 -o $@ in/...

out/nocompress-memcopy.go:   $(wildcard in/**/*) $(GOPATH)/bin/go-bindata
	$(GOPATH)/bin/go-bindata -nocompress            -o $@ in/...

out/nocompress-nomemcopy.go: $(wildcard in/**/*) $(GOPATH)/bin/go-bindata
	$(GOPATH)/bin/go-bindata -nocompress -nomemcopy -o $@ in/...

errcheck:
	go get github.com/kisielk/errcheck

golint:
	go get golang.org/x/lint/golint
