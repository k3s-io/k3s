#!/bin/bash

nodeOS=${1:-"bento/ubuntu-24.04"}
servercount=${2:-3}
agentcount=${3:-1}
db=${4:-"etcd"}
hardened=${5:-""}

E2E_EXTERNAL_DB=$db && export E2E_EXTERNAL_DB
E2E_REGISTRY=true && export E2E_REGISTRY

cd
cd k3s && git pull --rebase origin master
/usr/local/go/bin/go mod tidy

cd tests/e2e
OS=$(echo "$nodeOS"|cut -d'/' -f2)
echo "$OS"

vagrant global-status | awk '/running/'|cut -c1-7| xargs -r -d '\n' -n 1 -- vagrant destroy -f
E2E_RELEASE_CHANNEL="commit" && export E2E_RELEASE_CHANNEL

echo 'RUNNING DUALSTACK TEST'
E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v dualstack/dualstack_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1  -timeout=30m -json -ci |tee  k3s_"$OS".log

echo 'RUNNING CLUSTER VALIDATION TEST'
E2E_REGISTRY=true E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v validatecluster/validatecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=30m -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING SECRETS ENCRYPTION TEST'
/usr/local/go/bin/go test -v secretsencryption/secretsencryption_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -timeout=1h -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING SPLIT SERVER VALIDATION TEST'
E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v splitserver/splitserver_test.go -nodeOS="$nodeOS" -timeout=30m -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING STARTUP VALIDATION TEST'
/usr/local/go/bin/go test -v startup/startup_test.go -nodeOS="$nodeOS" -timeout=30m -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING EXTERNAL IP TEST'
/usr/local/go/bin/go test -v externalip/externalip_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING SNAPSHOT AND RESTORE TEST'
/usr/local/go/bin/go test -v snapshotrestore/snapshotrestore_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a k3s_"$OS".log

echo 'RUNNING ROTATE CUSTOM CA TEST'
/usr/local/go/bin/go test -v rotateca/rotateca_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a k3s_"$OS".log

E2E_RELEASE_CHANNEL="latest" && export E2E_RELEASE_CHANNEL
echo 'RUNNING CLUSTER UPGRADE TEST'
E2E_REGISTRY=true /usr/local/go/bin/go test -v upgradecluster/upgradecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=1h -json -ci | tee -a k3s_"$OS".log
