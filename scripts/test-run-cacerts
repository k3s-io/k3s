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

# -- This test runs in docker mounting the docker socket,
# -- so we can't directly mount files into the test containers. Instead we have to
# -- run a dummy container with a volume, copy files into that volume, and then
# -- share it with the other containers that need the file.
cluster-pre-hook() {
    mkdir -p $TEST_DIR/pause/0/metadata
    local testID=$(basename $TEST_DIR)
    local name=$(echo "k3s-pause-0-${testID,,}" | tee $TEST_DIR/pause/0/metadata/name)
    export SERVER_DOCKER_ARGS="--mount type=volume,src=$name,dst=/var/lib/rancher/k3s/server/tls"

    docker run \
        -d --name $name \
        --hostname $name \
        ${SERVER_DOCKER_ARGS} \
        rancher/mirrored-pause:3.6 \
        >/dev/null

    DATA_DIR="$TEST_DIR/pause/0/k3s" ./contrib/util/generate-custom-ca-certs.sh
    docker cp "$TEST_DIR/pause/0/k3s" $name:/var/lib/rancher
}
export -f cluster-pre-hook

start-test() {
  echo "Cluster is up with custom CA certs"
}
export -f start-test

test-cleanup-hook(){
    local testID=$(basename $TEST_DIR)
    docker volume ls -q | grep -F ${testID,,} | xargs -r docker volume rm
}
export -f test-cleanup-hook

# --- create a basic cluster and check for functionality
LABEL=CUSTOM-CA-CERTS run-test

cleanup-test-env
