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

start-test() {
    use-local-storage-volume
    docker exec $(cat $TEST_DIR/servers/1/metadata/name) check-config || true
    verify-valid-versions $(cat $TEST_DIR/servers/1/metadata/name)
    verify-airgap-images $(cat $TEST_DIR/{servers,agents}/*/metadata/name)
}
export -f start-test

# -- check for changes to the airgap image list
verify-airgap-images() {
    local airgap_image_list='scripts/airgap/image-list.txt'

    for name in $@; do
        docker exec $name crictl images -o json \
            | jq -r '.images[].repoTags[0] | select(. != null)'
    done | sort -u >$airgap_image_list.tmp

    if ! diff $airgap_image_list{,.tmp}; then
        echo '[ERROR] Failed airgap image check'
        return 1
    fi
}
export -f verify-airgap-images

# -- create a pod that uses local-storage to ensure that the local-path-provisioner
# -- helper image gets used
use-local-storage-volume() {
    local volume_test_manifest='scripts/airgap/volume-test.yaml'
    kubectl apply -f $volume_test_manifest
    wait-for-services volume-test
}
export -f use-local-storage-volume

# --- create a basic cluster and check for valid versions
LABEL=BASICS run-test

cleanup-test-env
