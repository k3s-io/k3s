#!/bin/bash
set -e

cd $(dirname $0)/../bin

echo Running
go run -tags "k8s no_etcd" ../cli/main.go --debug server --disable-controllers --disable-agent
