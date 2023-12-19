#!/bin/bash
set -e -x

res=$(go mod edit --json | jq -r '.Replace[] | select(.Old.Path | contains("k8s.io/")) | .New.Path' | grep -vE '^(k8s.io/|github.com/k3s-io/)' | wc -l)
if [ $res -gt 0 ];then
  echo "Incorrect kubernetes replacement fork in go.mod"
  exit 1
else
  exit 0
fi
