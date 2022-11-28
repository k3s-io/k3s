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

[ -d "$1" ] || {
  echo "First argument should be a directory" >&2
  exit 1
}

umask 077

GO=${GO-go}

TMPDIR=$(mktemp -d)
cleanup() {
  exit_code=$?
  trap - EXIT INT
  rm -rf ${TMPDIR}
  exit ${exit_code}
}
trap cleanup EXIT INT

LOG_TGZ=k3s-log-$(date +%s)-$("${GO}" env GOARCH)-$(git rev-parse --short HEAD)-$(basename $1).tgz

tar -cz -f ${TMPDIR}/${LOG_TGZ} -C $(dirname $1) $(basename $1)
aws s3 cp ${TMPDIR}/${LOG_TGZ} s3://k3s-ci-logs || exit 1
echo "Logs uploaded" >&2
echo "https://k3s-ci-logs.s3.amazonaws.com/${LOG_TGZ}"

