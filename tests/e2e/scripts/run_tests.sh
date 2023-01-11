#!/bin/bash

nodeOS=${1:-"generic/ubuntu2004"}
servercount=${2:-3}
agentcount=${3:-1}
db=${4:-"etcd"}
hardened=${5:-""}
k3s_version=${k3s_version}
k3s_channel=${k3s_channel:-"commit"}

E2E_EXTERNAL_DB=$db && export E2E_EXTERNAL_DB
E2E_REGISTRY=true && export E2E_REGISTRY

cd
cd k3s && git pull --rebase origin master
/usr/local/go/bin/go mod tidy

cd tests/e2e
OS=$(echo "$nodeOS"|cut -d'/' -f2)
echo "$OS"

# create directory if it does not exists
# create directory if it does not exists
if [ ! -d createreport ]
then
	mkdir createreport
fi

count=0
run_tests() {
  vagrant global-status | awk '/running/'|cut -c1-7| xargs -r -d '\n' -n 1 -- vagrant destroy -f

	echo 'RUNNING DUALSTACK TEST'
	E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v dualstack/dualstack_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1  -timeout=30m -json -ci |tee  createreport/k3s_"$OS".log

	echo 'RUNNING CLUSTER VALIDATION TEST'
	E2E_REGISTRY=true E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v validatecluster/validatecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING SECRETS ENCRYPTION TEST'
	/usr/local/go/bin/go test -v secretsencryption/secretsencryption_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -timeout=1h -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUN CLUSTER RESET TEST'
	/usr/local/go/bin/go test -v clusterreset/clusterreset_test.go -nodeOS="$nodeOS" -serverCount=3 -agentCount=1 -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING SPLIT SERVER VALIDATION TEST'
	E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v splitserver/splitserver_test.go -nodeOS="$nodeOS" -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING DOCKER CRI VALIDATION TEST'
	/usr/local/go/bin/go test -v docker/docker_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING EXTERNAL IP TEST'
	/usr/local/go/bin/go test -v externalip/externalip_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING PRE-BUNDLED-BIN IP TEST'
	/usr/local/go/bin/go test -v preferbundled/preferbundled_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	echo 'RUNNING SNAPSHOT AND RESTORE TEST'
	/usr/local/go/bin/go test -v snapshotrestore/snapshotrestore_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1 -timeout=30m -json -ci | tee -a createreport/k3s_"$OS".log

	E2E_RELEASE_VERSION=$k3s_version && export E2E_RELEASE_VERSION
	E2E_RELEASE_CHANNEL=$k3s_channel && export E2E_RELEASE_CHANNEL

  echo 'RUNNING CLUSTER UPGRADE TEST'
	E2E_REGISTRY=true /usr/local/go/bin/go test -v upgradecluster/upgradecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=1h -json -ci | tee -a createreport/k3s_"$OS".log
}

ls createreport/k3s_"$OS".log 2>/dev/null && rm createreport/k3s_"$OS".log
run_tests

while [ -f createreport/k3s_"$OS".log ] && grep -w ":\"fail" createreport/k3s_"$OS".log >>data && [ $count -le 2 ]
do
	echo "Re-running tests"
	cp createreport/k3s_"$OS".log createreport/k3s_"$OS"_"$count".log
	run_tests
done