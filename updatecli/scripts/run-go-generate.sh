#!/bin/bash

set -eux

./scripts/download >&2
go generate >&2
git diff

exit 0

