#!/bin/bash

all_services=(
    coredns
    local-path-provisioner
    metrics-server
    traefik
)

export NUM_SERVERS=1
export NUM_AGENTS=1
export WAIT_SERVICES="${all_services[@]}"

REPO=${REPO:-rancher}
IMAGE_NAME=${IMAGE_NAME:-k3s}
CURRENT_CHANNEL=$(echo ${VERSION_K8S} | awk -F. '{print "v1." $2}')
CURRENT_VERSION=$(curl -s https://update.k3s.io/v1-release/channels/${CURRENT_CHANNEL} -o /dev/null -w '%{redirect_url}' | awk -F/ '{print gensub(/\+/, "-", "g", $NF)}')
if [ -z "${CURRENT_VERSION}" ]; then
    CURRENT_VERSION=${VERSION_TAG}
fi
export K3S_IMAGE_SERVER=${REPO}/${IMAGE_NAME}:${CURRENT_VERSION}${SUFFIX}
export K3S_IMAGE_AGENT=${REPO}/${IMAGE_NAME}:${CURRENT_VERSION}${SUFFIX}

server-pre-hook(){
    local testID=$(basename $TEST_DIR)
    export SERVER_DOCKER_ARGS="\
        --mount type=volume,src=k3s-server-$1-${testID,,}-rancher,dst=/var/lib/rancher/k3s \
        --mount type=volume,src=k3s-server-$1-${testID,,}-log,dst=/var/log \
        --mount type=volume,src=k3s-server-$1-${testID,,}-etc,dst=/etc/rancher"
}
export -f server-pre-hook

agent-pre-hook(){
    local testID=$(basename $TEST_DIR)
    export AGENT_DOCKER_ARGS="\
        --mount type=volume,src=k3s-agent-$1-${testID,,}-rancher,dst=/var/lib/rancher/k3s \
        --mount type=volume,src=k3s-agent-$1-${testID,,}-log,dst=/var/log \
        --mount type=volume,src=k3s-agent-$1-${testID,,}-etc,dst=/etc/rancher"
}
export -f agent-pre-hook

start-test() {
    # Create a pod and print the version before upgrading
    kubectl get node -o wide
    kubectl create -f scripts/airgap/volume-test.yaml

    # Add post-hook sleeps to give the kubelet time to update the version after startup.
    # Server gets an extra 60 seconds to handle the metrics-server service being unavailable:
    # https://github.com/kubernetes/kubernetes/issues/120739
    server-post-hook(){
        sleep 75
    }
    export -f server-post-hook
    agent-post-hook(){
        sleep 15
    }
    export -f agent-post-hook

    # Switch the image back to the current build, delete the node containers, and re-provision with the same datastore volumes
    unset K3S_IMAGE_SERVER
    unset K3S_IMAGE_AGENT
    if [ $NUM_AGENTS -gt 0 ]; then
        for i in $(seq 1 $NUM_AGENTS); do
          docker rm -f -v $(cat $TEST_DIR/agents/$i/metadata/name)
          rm -rf $TEST_DIR/agents/$i
        done
    fi
    for i in $(seq 1 $NUM_SERVERS); do
        docker rm -f -v $(cat $TEST_DIR/servers/$i/metadata/name)
        rm -rf $TEST_DIR/servers/$i
    done
    provision-cluster

    # Confirm that the nodes are running the current build and that the pod we created earlier is still there
    . ./scripts/version.sh || true

    verify-valid-versions $(cat $TEST_DIR/servers/1/metadata/name)

    kubectl get pod -n kube-system volume-test -o wide

    if ! kubectl get node -o wide | grep -qF $VERSION; then
      echo "Expected version $VERSION not found in node list"
      return 1
    fi
}
export -f start-test

test-cleanup-hook(){
    local testID=$(basename $TEST_DIR)
    docker volume ls -q | grep -F ${testID,,} | xargs -r docker volume rm
}
export -f test-cleanup-hook

# --- create a single-node cluster from the latest release, then restart the containers with the current build
LABEL=UPGRADE run-test

cleanup-test-env
