#!/bin/bash

set -eux

go get "${1}" >&2
go mod tidy >&2
git diff

exit 0

