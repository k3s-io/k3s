#!/bin/sh

set -x

if [ "$1" = "" ]; then
    exec bin/${PROG} agent $@
fi
exec bin/${PROG} $@
