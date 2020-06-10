#!/bin/bash
set -e

cd $(dirname $0)/..

. ./scripts/setup-rancher-path.sh

GO=${GO-go}

# Prime sudo
sudo echo Compiling

if [ ! -e bin/containerd ]; then
    ./scripts/build
    ./scripts/package
else
    rm -f ./bin/k3s-agent
    "${GO}" build -tags "apparmor seccomp" -o ./bin/k3s-agent ./cmd/agent/main.go
fi

echo Starting agent
sudo env "PATH=$(pwd)/bin:$PATH" ./bin/k3s-agent --debug agent -s https://localhost:6443 -t $(<${RANCHER_PATH}/k3s/server/node-token) "$@"
