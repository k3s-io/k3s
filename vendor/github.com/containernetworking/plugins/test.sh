#!/usr/bin/env bash
#
# Run CNI plugin tests.
# 
# This needs sudo, as we'll be creating net interfaces.
#
set -e

source ./build.sh

echo "Running tests"

# test everything that's not in vendor
pushd "$GOPATH/src/$REPO_PATH" >/dev/null
  ALL_PKGS="$(go list ./... | grep -v vendor | xargs echo)"
popd >/dev/null

GINKGO_FLAGS="-p --randomizeAllSpecs --randomizeSuites --failOnPending --progress"

# user has not provided PKG override
if [ -z "$PKG" ]; then
  GINKGO_FLAGS="$GINKGO_FLAGS -r ."
  LINT_TARGETS="$ALL_PKGS"

# user has provided PKG override
else
  GINKGO_FLAGS="$GINKGO_FLAGS $PKG"
  LINT_TARGETS="$PKG"
fi

cd "$GOPATH/src/$REPO_PATH"
sudo -E bash -c "umask 0; PATH=${GOROOT}/bin:$(pwd)/bin:${PATH} ginkgo ${GINKGO_FLAGS}"

echo "Checking gofmt..."
fmtRes=$(go fmt $LINT_TARGETS)
if [ -n "${fmtRes}" ]; then
	echo -e "go fmt checking failed:\n${fmtRes}"
	exit 255
fi

echo "Checking govet..."
vetRes=$(go vet $LINT_TARGETS)
if [ -n "${vetRes}" ]; then
	echo -e "govet checking failed:\n${vetRes}"
	exit 255
fi
