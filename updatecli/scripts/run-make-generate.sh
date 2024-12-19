#!/bin/bash

set -eux

mkdir -p build/data

make download

make generate

git diff

exit 0
