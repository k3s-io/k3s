#!/bin/bash
set -e

cd $(dirname $0)/..

. ./scripts/setup-rancher-path.sh

IP=$(ip addr show dev docker0 | grep -w inet | awk '{print $2}' | cut -f1 -d/)
docker run \
    --read-only \
    --tmpfs /run \
    --tmpfs /var/run \
    --tmpfs /tmp \
    -v /lib/modules:/lib/modules:ro \
    -v /lib/firmware:/lib/firmware:ro \
    -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro \
    -v $(pwd)/bin:/usr/bin \
    -v /var/log \
    -v /var/lib/kubelet \
    -v /var/lib/rancher/k3s \
    -v /var/lib/cni \
    -v /usr/lib/x86_64-linux-gnu/libsqlite3.so.0:/usr/lib/x86_64-linux-gnu/libsqlite3.so.0:ro \
    --privileged \
    ubuntu:18.04 /usr/bin/k3s-agent agent -t $(<${RANCHER_PATH}/k3s/server/node-token) -s https://${IP}:6443
