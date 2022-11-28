#!/bin/bash

# Check for AWS Credentials
[ -n "$AWS_SECRET_ACCESS_KEY" ] || {
  echo "AWS_SECRET_ACCESS_KEY is not set"
  exit 0
}
[ -n "$AWS_ACCESS_KEY_ID" ] || {
  echo "AWS_ACCESS_KEY_ID is not set"
  exit 0
}

[ -x "$1" ] || {
  echo "First argument should be an executable" >&2
  exit 1
}
[ -n "$2" ] || {
  echo "Second argument should be a commit hash" >&2
  exit 1
}

umask 077

TMPDIR=$(mktemp -d)
cleanup() {
  exit_code=$?
  trap - EXIT INT
  rm -rf ${TMPDIR}
  exit ${exit_code}
}
trap cleanup EXIT INT

BUILD_NAME=$(basename $1)-$2
(cd $(dirname $1) && sha256sum $(basename $1)) >${TMPDIR}/${BUILD_NAME}.sha256sum
cp $1 ${TMPDIR}/${BUILD_NAME}

for FILE in ${TMPDIR}/${BUILD_NAME}*; do
  aws s3 cp ${FILE} s3://k3s-ci-builds || exit 1
done

echo "Build uploaded" >&2
echo "https://k3s-ci-builds.s3.amazonaws.com/${BUILD_NAME}"
