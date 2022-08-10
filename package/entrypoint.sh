#!/bin/sh

set -x

if [ "$1" = "" ]; then
    bin/${PROG} agent $@
else
    bin/${PROG} $@
fi
