#!/bin/bash
set -e

SCRIPT_DIR=$(dirname $0)
pushd $SCRIPT_DIR

./download
./validate
./build
./package

popd

$SCRIPT_DIR/binary_size_check.sh
