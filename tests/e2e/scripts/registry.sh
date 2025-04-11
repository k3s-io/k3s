#!/bin/bash

# Script to to point k3s to the docker registry running on the host
# This is used to avoid hitting dockerhub rate limits on E2E runners
ip_addr=$1

mkdir -p /etc/rancher/k3s/
echo "mirrors:
  docker.io:
    endpoint:
      - \"http://$ip_addr:15000\"
  registry.k8s.io:
    endpoint:
      - \"http://$ip_addr:15001\"
  gcr.io:
    endpoint:
      - \"http://$ip_addr:15002\"
  quay.io:
    endpoint:
      - \"http://$ip_addr:15003\"
  ghcr.io:
    endpoint:
      - \"http://$ip_addr:15004\"" >> /etc/rancher/k3s/registries.yaml