#!/bin/bash
servercount=${5:-3}
agentcount=${6:-1}
db=${7:-"etcd"}
k3s_version=${k3s_version}
k3s_channel=${k3s_channel:-"commit"}
hardened=${8:-""}

E2E_EXTERNAL_DB=$db && export E2E_EXTERNAL_DB
E2E_REGISTRY=true && export E2E_REGISTRY

eval openvpn --daemon --config external.ovpn &>/dev/null &
sleep 10

ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s && git pull --rebase origin master'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s && /usr/local/go/bin/go mod tidy'

ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e/dualstack && vagrant global-status | awk '/running/'|cut -c1-7| xargs -r -d '\n' -n 1 -- vagrant destroy -f"

echo 'RUNNING DUALSTACK VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v dualstack/dualstack_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/dualstack && vagrant destroy -f'

echo 'RUNNING CLUSTER VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_REGISTRY=true E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v validatecluster/validatecluster_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/validatecluster && vagrant destroy -f'

echo 'RUNNING SECRETS ENCRYPTION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v secretsencryption/secretsencryption_test.go -nodeOS="$4" -serverCount=$((servercount)) -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/secretsencryption && vagrant destroy -f'

echo 'RUNNING SNAPSHOT RESTORE TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v snapshotrestore/snapshotrestore_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/secretsencryption && vagrant destroy -f'

echo 'RUNNING SPLIT SERVER VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v splitserver/splitserver_test.go -nodeOS="$4" -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/splitserver && vagrant destroy -f'

E2E_RELEASE_VERSION=$k3s_version && export E2E_RELEASE_VERSION
E2E_RELEASE_CHANNEL=$k3s_channel && export E2E_RELEASE_CHANNEL

echo 'RUNNING CLUSTER UPGRADE TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_REGISTRY=true /usr/local/go/bin/go test -v upgradecluster/upgradecluster_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/upgradecluster && vagrant destroy -f'

echo 'RUN CLUSTER RESET TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v clusterreset/clusterreset_test.go -nodeOS="$4" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/clusterreset && vagrant destroy -f'

echo 'RUNNING DOCKER CRI VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && /usr/local/go/bin/go test -v docker/docker_test.go -nodeOS="$4" -serverCount=1 -agentCount=1 -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/docker && vagrant destroy -f'

echo 'RUNNING EXTERNALIP VALIDATION TEST'
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 "cd k3s/tests/e2e && E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v externalip/externalip_test.go -nodeOS="$4" -serverCount=1 -agentCount=1  -timeout=30m -json" | tee -a testreport.log
ssh -i "$1"  -o "StrictHostKeyChecking no" $2@$3 'cd k3s/tests/e2e/dualstack && vagrant destroy -f'
