#!/bin/bash

TRAEFIK_CHART_VERSION=$(yq e '.spec.chart' manifests/traefik.yaml | awk 'match($0, /([0-9.]+)([0-9]{2})/, m) { print m[1]; exit; }')
TRAEFIK_PACKAGE_VERSION=$(yq e '.spec.chart' manifests/traefik.yaml | awk 'match($0, /([0-9.]+)([0-9]{2})/, m) { print m[2]; exit; }')
TRAEFIK_FILE=traefik-${TRAEFIK_CHART_VERSION}${TRAEFIK_PACKAGE_VERSION}.tgz
TRAEFIK_URL=https://github.com/traefik/traefik-helm-chart/raw/gh-pages/traefik-${TRAEFIK_CHART_VERSION}.tgz
TRAEFIK_CRD_FILE=traefik-crd-${TRAEFIK_CHART_VERSION}${TRAEFIK_PACKAGE_VERSION}.tgz
