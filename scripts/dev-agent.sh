#!/bin/bash
set -e

cd $(dirname $0)/../bin

# Prime sudo
sudo echo Compiling CLI
go build -tags "k8s no_etcd" -o rio-agent ../cli/main.go

echo Building image and agent
../image/build

echo Running
exec sudo ENTER_ROOT=../image/main.squashfs ./rio-agent --debug agent -s https://localhost:7443 -t $(<${HOME}/.rancher/rio/server/node-token)
