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

agent-pre-hook() {
    timeout --foreground 2m bash -c "wait-for-nodes $(( NUM_SERVERS ))"
    local server=$(cat $TEST_DIR/servers/1/metadata/name)
    docker exec $server k3s token create --ttl=5m --description=Test > $TEST_DIR/metadata/secret
}
export -f agent-pre-hook

start-test() {
  echo "Cluster is up with ephemeral join token"
}
export -f start-test

# --- create a basic cluster with an agent joined using the ephemeral token and check for functionality
LABEL=BOOTSTRAP-TOKEN run-test

cleanup-test-env
