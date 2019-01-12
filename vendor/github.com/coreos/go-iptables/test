#!/usr/bin/env bash
#
# Run all go-iptables tests
#   ./test
#   ./test -v
#
# Run tests for one package
#   PKG=./unit ./test
#   PKG=ssh ./test
#
set -e

# Invoke ./cover for HTML output
COVER=${COVER:-"-cover"}

source ./build

TESTABLE="iptables"
FORMATTABLE="$TESTABLE"

# user has not provided PKG override
if [ -z "$PKG" ]; then
	TEST=$TESTABLE
	FMT=$FORMATTABLE

# user has provided PKG override
else
	# strip out slashes and dots from PKG=./foo/
	TEST=${PKG//\//}
	TEST=${TEST//./}

	# only run gofmt on packages provided by user
	FMT="$TEST"
fi

echo "Checking gofmt..."
fmtRes=$(gofmt -l $FMT)
if [ -n "${fmtRes}" ]; then
	echo -e "gofmt checking failed:\n${fmtRes}"
	exit 255
fi

# split TEST into an array and prepend REPO_PATH to each local package
split=(${TEST// / })
TEST=${split[@]/#/${REPO_PATH}/}

echo "Running tests..."
bin=$(mktemp)

go test -c -o ${bin} ${COVER} -i ${TEST}
if [[ -z "$SUDO_PERMITTED" ]]; then
    echo "Test aborted for safety reasons. Please set the SUDO_PERMITTED variable."
    exit 1
fi

sudo -E bash -c "${bin} $@ ${TEST}"
echo "Success"
rm "${bin}"
