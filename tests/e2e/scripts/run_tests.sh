#!/bin/bash
servercount=${5:-3}
agentcount=${6:-2}
db=${7:-"etcd"}
k3s_version=${k3s_version}
k3s_channel=${k3s_channel:-"commit"}
hardened=${8:-""}

E2E_EXTERNAL_DB=$db && export E2E_EXTERNAL_DB

eval openvpn --daemon --config external.ovpn &>/dev/null &
sleep 10

ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s && git pull --rebase origin master'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s && go mod tidy'

echo 'RUNNING CLUSTER VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/validatecluster && vagrant destroy -f'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v validatecluster/validatecluster_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=1h"

echo 'RUNNING SECRETS ENCRYPTION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/secretsencryption && vagrant destroy -f'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v secretsencryption/secretsencryption_test.go -nodeOS="$4" -serverCount=$((servercount)) -timeout=1h"

E2E_RELEASE_VERSION=$k3s_version && export E2E_RELEASE_VERSION
E2E_RELEASE_CHANNEL=$k3s_channel && export E2E_RELEASE_CHANNEL

echo 'RUNNING CLUSTER UPGRADE TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/upgradecluster && vagrant destroy -f'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v upgradecluster/upgradecluster_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=1h"
