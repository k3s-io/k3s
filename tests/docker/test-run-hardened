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
export AGENT_ARGS="--selinux=true \
--protect-kernel-defaults=true \
--kubelet-arg=streaming-connection-idle-timeout=5m \
--kubelet-arg=make-iptables-util-chains=true"
export SERVER_ARGS="--selinux=true \
--protect-kernel-defaults=true \
--kubelet-arg=streaming-connection-idle-timeout=5m \
--kubelet-arg=make-iptables-util-chains=true \
--secrets-encryption=true \
--kube-apiserver-arg=audit-log-path=/tmp/audit-log \
--kube-apiserver-arg=audit-log-maxage=30 \
--kube-apiserver-arg=audit-log-maxbackup=10 \
--kube-apiserver-arg=audit-log-maxsize=100 \
--kube-apiserver-arg=enable-admission-plugins=NodeRestriction,NamespaceLifecycle,ServiceAccount \
--kube-apiserver-arg=admission-control-config-file=/opt/rancher/k3s/cluster-level-pss.yaml \
--kube-controller-manager-arg=terminated-pod-gc-threshold=10 \
--kube-controller-manager-arg=use-service-account-credentials=true"

# -- This test runs in docker mounting the docker socket,
# -- so we can't directly mount files into the test containers. Instead we have to
# -- run a dummy container with a volume, copy files into that volume, and then
# -- share it with the other containers that need the file.
cluster-pre-hook() {
    mkdir -p $TEST_DIR/pause/0/metadata
    local testID=$(basename $TEST_DIR)
    local name=$(echo "k3s-pause-0-${testID,,}" | tee $TEST_DIR/pause/0/metadata/name)
    export SERVER_DOCKER_ARGS="--mount type=volume,src=$name,dst=/opt/rancher/k3s"

    docker run \
        -d --name $name \
        --hostname $name \
        ${SERVER_DOCKER_ARGS} \
        rancher/mirrored-pause:3.6 \
        >/dev/null

    docker cp scripts/hardened/cluster-level-pss.yaml $name:/opt/rancher/k3s/cluster-level-pss.yaml
}
export -f cluster-pre-hook

# -- deploy and wait for a daemonset to run on all nodes, then wait a couple more
# -- seconds for traefik to see the service endpoints before testing.
start-test() {
    find ./scripts/hardened/ -name 'hardened-k3s-*.yaml' -printf '-f\0%p\0' | xargs -tr0 kubectl create
    kubectl rollout status daemonset/example --watch --timeout=5m
    sleep 15
    verify-ingress
    verify-nodeport
}
export -f start-test

test-cleanup-hook(){
    local testID=$(basename $TEST_DIR)
    docker volume ls -q | grep -F ${testID,,} | xargs -r docker volume rm
}
export -f test-cleanup-hook

# -- confirm we can make a request through the ingress
verify-ingress() {
    local ips=$(cat $TEST_DIR/{servers,agents}/*/metadata/ip)
    local schemes="http https"
    for ip in $ips; do
      for scheme in $schemes; do
        curl -vksf -H 'Host: example.com' ${scheme}://${ip}/
      done
    done
}
export -f verify-ingress

# -- confirm we can make a request through the nodeport service
verify-nodeport() {
    local ips=$(cat $TEST_DIR/{servers,agents}/*/metadata/ip)
    local ports=$(kubectl get service/example -o 'jsonpath={.spec.ports[*].nodePort}')
    for ip in $ips; do
        for port in $ports; do
            curl -vksf -H 'Host: example.com' http://${ip}:${port}
        done
    done
}
export -f verify-nodeport

# --- create a basic cluster and check for functionality
LABEL=HARDENED run-test

cleanup-test-env
