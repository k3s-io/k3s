#!/bin/bash

# Script to set up Docker registry proxies for various public registries
# Creates proxy registries for: 
# - registry-1.docker.io
# - registry.k8s.io
# - gcr.io
# - quay.io
# - ghcr.io
#
# Not persistent - containers will not survive host reboot

declare -A registries
declare -A registry_ports
registries=(
    ["dockerhub"]="registry-1.docker.io"
    ["k8s_io"]="registry.k8s.io"
    ["gcr_io"]="gcr.io"
    ["quay_io"]="quay.io"
    ["ghcr_io"]="ghcr.io"
)
registry_ports=(
    ["dockerhub"]=15000
    ["k8s_io"]=15001
    ["gcr_io"]=15002
    ["quay_io"]=15003
    ["ghcr_io"]=15004
)

# is_registry_running checsk if a registry is already exists
is_registry_running() {
    local name=$1
    docker ps --format '{{.Names}}' | grep -q "^${name}$"
    return $?
}

create_registry_proxy() {
    local name=$1
    local upstream=$2
    local port=$3
    
    echo "Setting up registry proxy for ${upstream} on port ${port}"
    
    docker run -d \
        --name "${name}" \
        -e "REGISTRY_PROXY_REMOTEURL=https://${upstream}" \
        -e "REGISTRY_HTTP_SECRET=shared-secret" \
        -e "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/var/lib/registry/$name" \
        -p "${port}:5000" \
        registry:2
        
    echo "Registry proxy for ${upstream} started on port ${port}"
}



# Set up each registry proxy
for name in "${!registries[@]}"; do
    upstream=${registries[$name]}
    port=${registry_ports[$name]}
    if is_registry_running "registry_${name}"; then
        echo "Registry proxy for ${upstream} already running"
    else
        create_registry_proxy "registry_${name}" "${upstream}" "${port}"
    fi
done