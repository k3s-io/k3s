#/bin/sh

set -x

# Fix up generated code.
gofix()
{
    IN=$1
    OUT=$2

    # Change types that are a nuisance to deal with in Go, use byte for
    # consistency, and produce gofmt'd output.
    sed 's/]u*int8/]byte/g' $1 | gofmt -s > $2
}

echo -e "//+build openbsd,amd64\n" > /tmp/wgamd64.go
GOARCH=amd64 go tool cgo -godefs defs.go >> /tmp/wgamd64.go

echo -e "//+build openbsd,386\n" > /tmp/wg386.go
GOARCH=386 go tool cgo -godefs defs.go >> /tmp/wg386.go

gofix /tmp/wgamd64.go defs_openbsd_amd64.go
gofix /tmp/wg386.go defs_openbsd_386.go

rm -rf _obj/ /tmp/wg*.go
