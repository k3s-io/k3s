#!/bin/bash
set -e

. ./scripts/version.sh

cd $(dirname $0)/..

. ./scripts/setup-rancher-path.sh

GO=${GO-go}

# Prime sudo
sudo echo Compiling

if [ ! -e bin/containerd ]; then
    ./scripts/build
    ./scripts/package
else
    rm -f ./bin/${PROG}-agent
    "${GO}" build -tags "apparmor seccomp" -o ./bin/${PROG}-agent ./cmd/agent/main.go
fi

echo Starting agent
sudo env "PATH=$(pwd)/bin:$PATH" ./bin/${PROG}-agent --debug agent -s https://localhost:6443 -t $(<${RANCHER_PATH}/${PROG}/server/node-token) "$@"
