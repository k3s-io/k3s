#!/bin/bash

all_services=(
    coredns
    local-path-provisioner
    metrics-server
    traefik
)

export NUM_SERVERS=2
export NUM_AGENTS=0
export WAIT_SERVICES="${all_services[@]}"
export SERVER_1_ARGS="--cluster-init"

REPO=${REPO:-rancher}
IMAGE_NAME=${IMAGE_NAME:-k3s}
PREVIOUS_CHANNEL=$(echo ${VERSION_K8S} | awk -F. '{print "v1." ($2 - 1)}')
PREVIOUS_VERSION=$(curl -s https://update.k3s.io/v1-release/channels/${PREVIOUS_CHANNEL} -o /dev/null -w '%{redirect_url}' | awk -F/ '{print gensub(/\+/, "-", "g", $NF)}')
STABLE_VERSION=$(curl -s https://update.k3s.io/v1-release/channels/stable -o /dev/null -w '%{redirect_url}' | awk -F/ '{print gensub(/\+/, "-", "g", $NF)}')
LATEST_VERSION=$(curl -s https://update.k3s.io/v1-release/channels/latest -o /dev/null -w '%{redirect_url}' | awk -F/ '{print gensub(/\+/, "-", "g", $NF)}')

server-post-hook() {
  if [ $1 -eq 1 ]; then
    local url=$(cat $TEST_DIR/servers/1/metadata/url)
    export SERVER_ARGS="${SERVER_ARGS} --server $url"
  fi
}
export -f server-post-hook

start-test() {
  echo "Cluster is up"
}
export -f start-test

# --- create a basic cluster to test joining managed etcd
LABEL="ETCD-JOIN-BASIC" SERVER_ARGS="" run-test

# --- create a basic cluster to test joining a managed etcd cluster with --agent-token set
LABEL="ETCD-JOIN-AGENTTOKEN" SERVER_ARGS="--agent-token ${RANDOM}${RANDOM}${RANDOM}" run-test

# --- create a cluster with three etcd-only server, two control-plane-only server, and one agent
server-post-hook() {
  if [ $1 -eq 1 ]; then
    local url=$(cat $TEST_DIR/servers/1/metadata/url)
    export SERVER_ARGS="${SERVER_ARGS} --server $url"
  fi
}
export -f server-post-hook
LABEL="ETCD-SPLIT-ROLE" NUM_AGENTS=1 KUBECONFIG_SERVER=4 NUM_SERVERS=5 \
SERVER_1_ARGS="--disable-apiserver --disable-controller-manager --disable-scheduler --cluster-init" \
SERVER_2_ARGS="--disable-apiserver --disable-controller-manager --disable-scheduler" \
SERVER_3_ARGS="--disable-apiserver --disable-controller-manager --disable-scheduler" \
SERVER_4_ARGS="--disable-etcd" \
SERVER_5_ARGS="--disable-etcd" \
run-test


# The following tests deploy clusters of mixed versions. The traefik helm chart may not deploy
# correctly until all servers have been upgraded to the same release, so don't wait for it.
all_services=(
    coredns
    local-path-provisioner
    metrics-server
)
export WAIT_SERVICES="${all_services[@]}"

# --- test joining managed etcd cluster with stable-version first server and current-build second server
# --- this test is skipped if the second node is down-level, as we don't support adding a down-level server to an existing cluster
server-post-hook() {
  if [ $1 -eq 1 ]; then
    SERVER_1_MINOR=$(awk -F. '{print $2}' <<<${K3S_IMAGE_SERVER})
    SERVER_2_MINOR=$(awk -F. '{print $2}' <<<${K3S_IMAGE})
    if [ $SERVER_1_MINOR -gt $SERVER_2_MINOR ]; then
        echo "First server minor version cannot be higher than second server"
        exit 0
    fi

    local url=$(cat $TEST_DIR/servers/1/metadata/url)
    export SERVER_ARGS="${SERVER_ARGS} --server $url"
    export K3S_IMAGE_SERVER=${K3S_IMAGE}
  fi
}
export -f server-post-hook
LABEL="ETCD-JOIN-STABLE-FIRST" K3S_IMAGE_SERVER=${REPO}/${IMAGE_NAME}:${STABLE_VERSION} run-test

# --- test joining managed etcd cluster with latest-version first server and current-build second server
# --- this test is skipped if the second node is down-level, as we don't support adding a down-level server to an existing cluster
server-post-hook() {
  if [ $1 -eq 1 ]; then
    SERVER_1_MINOR=$(awk -F. '{print $2}' <<<${K3S_IMAGE_SERVER})
    SERVER_2_MINOR=$(awk -F. '{print $2}' <<<${K3S_IMAGE})
    if [ $SERVER_1_MINOR -gt $SERVER_2_MINOR ]; then
        echo "First server minor version cannot be higher than second server"
        exit 0
    fi

    local url=$(cat $TEST_DIR/servers/1/metadata/url)
    export SERVER_ARGS="${SERVER_ARGS} --server $url"
    export K3S_IMAGE_SERVER=${K3S_IMAGE}
  fi
}
export -f server-post-hook
LABEL="ETCD-JOIN-LATEST-FIRST" K3S_IMAGE_SERVER=${REPO}/${IMAGE_NAME}:${LATEST_VERSION} run-test

# --- test joining a managed etcd cluster with incompatible configuration
test-post-hook() {
  if [[ $1 -eq 0 ]]; then
    return
  fi
  dump-logs skip-output
  grep -sqF 'critical configuration value mismatch' $TEST_DIR/servers/2/logs/system.log
}
export -f test-post-hook
LABEL="ETCD-JOIN-MISMATCH" SERVER_2_ARGS="--cluster-cidr 10.0.0.0/16" run-test

cleanup-test-env
