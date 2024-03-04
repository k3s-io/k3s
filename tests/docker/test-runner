#!/bin/bash
set -x -e
cd $(dirname $0)/../..

# ---

for include in $TEST_INCLUDES; do
  . $include
done

test-setup
provision-cluster
start-test $@
