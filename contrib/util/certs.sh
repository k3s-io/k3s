#!/usr/bin/env bash

# Example K3s CA certificate generation script.
# 
# This script will generate files sufficient to bootstrap K3s cluster certificate
# authorities.  By default, the script will create the required files under
# /var/lib/rancher/k3s/server/tls, where they will be found and used by K3s during initial
# cluster startup. Note that these files MUST be present before K3s is started the first
# time; certificate data SHOULD NOT be changed once the cluster has been initialized.
#
# The output path may be overridden with the DATA_DIR environment variable.
# 
# This script will also auto-generate certificates and keys for both root and intermediate
# certificate authorities if none are found.
# If you have only an existing root CA, provide:
#   root-ca.pem
#   root-ca.key.
# If you have an existing root and intermediate CA, provide:
#   root-ca.pem
#   intermediate-ca.pem
#   intermediate-ca.key.

set -e

CONFIG="
[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer:always
basicConstraints=CA:true"
TIMESTAMP=$(date +%s)
PRODUCT="${PRODUCT:-k3s}"
DATA_DIR="${DATA_DIR:-/var/lib/rancher/${PRODUCT}}"

if type -t openssl-3 &>/dev/null; then
  OPENSSL=openssl-3
else
  OPENSSL=openssl
fi

echo "Using $(which ${OPENSSL}): $(${OPENSSL} version)"

if ! ${OPENSSL} ecparam -help &>/dev/null; then
  echo "openssl not found or missing Elliptic Curve (ecparam) support."
  exit 1
fi

if ! ${OPENSSL} req -help 2>&1 | grep -q CAkey; then
  echo "openssl req missing -CAkey support; please use OpenSSL 3.0.0 or newer"
  exit 1
fi

mkdir -p "${DATA_DIR}/server/tls/etcd"
cd "${DATA_DIR}/server/tls"

# Don't overwrite the service account issuer key; we pass the key into both the controller-manager
# and the apiserver instead of passing a cert list into the apiserver, so there's no facility for
# rotation and things will get very angry if all the SA keys are invalidated.
if [[ -e service.key ]]; then
  echo "Generating additional Kubernetes service account issuer RSA key"
  OLD_SERVICE_KEY="$(cat service.key)"
else
  echo "Generating Kubernetes service account issuer RSA key"
fi
${OPENSSL} genrsa -traditional -out service.key 2048
echo "${OLD_SERVICE_KEY}" >> service.key

# Use existing root CA if present
if [[ -e root-ca.pem ]]; then
  echo "Using existing root certificate"
else
  echo "Generating root certificate authority RSA key and certificate"
  ${OPENSSL} genrsa -out root-ca.key 4096
  ${OPENSSL} req -x509 -new -nodes -key root-ca.key -sha256 -days 7300 -out root-ca.pem -subj "/CN=${PRODUCT}-root-ca@${TIMESTAMP}" -config <(echo "${CONFIG}") -extensions v3_ca
fi
cat root-ca.pem > root-ca.crt

# Use existing intermediate CA if present
if [[ -e intermediate-ca.pem ]]; then
  echo "Using existing intermediate certificate"
else
  if [[ ! -e root-ca.key ]]; then
    echo "Cannot generate intermediate certificate without root certificate private key"
    exit 1
  fi

  echo "Generating intermediate certificate authority RSA key and certificate"
  ${OPENSSL} genrsa -out intermediate-ca.key 4096
  ${OPENSSL} req -x509 -new -nodes -CAkey root-ca.key -CA root-ca.crt -key intermediate-ca.key -sha256 -days 7300 -out intermediate-ca.pem -subj "/CN=${PRODUCT}-intermediate-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:1") -extensions v3_ca
fi
cat intermediate-ca.pem root-ca.pem > intermediate-ca.crt

if [[ ! -e intermediate-ca.key ]]; then
  echo "Cannot generate leaf certificates without intermediate certificate private key"
  exit 1
fi

# Generate new leaf CAs for all the control-plane and etcd components
echo "Generating Kubernetes server leaf certificate authority EC key and certificate"
${OPENSSL} ecparam -name prime256v1 -genkey -out client-ca.key
${OPENSSL} req -x509 -new -nodes -CAkey intermediate-ca.key -CA intermediate-ca.crt -key client-ca.key -sha256 -days 3650 -out client-ca.pem -subj "/CN=${PRODUCT}-client-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:0") -extensions v3_ca
cat client-ca.pem intermediate-ca.pem root-ca.pem > client-ca.crt

echo "Generating Kubernetes client leaf certificate authority EC key and certificate"
${OPENSSL} ecparam -name prime256v1 -genkey -out server-ca.key
${OPENSSL} req -x509 -new -nodes -CAkey intermediate-ca.key -CA intermediate-ca.crt -key server-ca.key -sha256 -days 3650 -out server-ca.pem -subj "/CN=${PRODUCT}-server-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:0") -extensions v3_ca
cat server-ca.pem intermediate-ca.pem root-ca.pem > server-ca.crt

echo "Generating Kubernetes request-header leaf certificate authority EC key and certificate"
${OPENSSL} ecparam -name prime256v1 -genkey -out request-header-ca.key
${OPENSSL} req -x509 -new -nodes -CAkey intermediate-ca.key -CA intermediate-ca.crt -key request-header-ca.key -sha256 -days 3560 -out request-header-ca.pem -subj "/CN=${PRODUCT}-request-header-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:0") -extensions v3_ca
cat request-header-ca.pem intermediate-ca.pem root-ca.pem > request-header-ca.crt

echo "Generating etcd peer leaf certificate authority EC key and certificate"
${OPENSSL} ecparam -name prime256v1 -genkey -out etcd/peer-ca.key
${OPENSSL} req -x509 -new -nodes -CAkey intermediate-ca.key -CA intermediate-ca.crt -key etcd/peer-ca.key -sha256 -days 3650 -out etcd/peer-ca.pem -subj "/CN=etcd-peer-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:0") -extensions v3_ca
cat etcd/peer-ca.pem intermediate-ca.pem root-ca.pem > etcd/peer-ca.crt

echo "Generating etcd server leaf certificate authority EC key and certificate"
${OPENSSL} ecparam -name prime256v1 -genkey -out etcd/server-ca.key
${OPENSSL} req -x509 -new -nodes -CAkey intermediate-ca.key -CA intermediate-ca.crt -key etcd/server-ca.key -sha256 -days 3650 -out etcd/server-ca.pem -subj "/CN=etcd-server-ca@${TIMESTAMP}" -config <(echo "${CONFIG}, pathlen:0") -extensions v3_ca
cat etcd/server-ca.pem intermediate-ca.pem root-ca.pem > etcd/server-ca.crt

echo
echo "CA certificate generation complete. Required files are now present in: ${DATA_DIR}/server/tls"
echo "For security purposes, you should make a secure copy of the following files and remove them from cluster members:"
ls ${DATA_DIR}/server/tls/root-ca.* ${DATA_DIR}/server/tls/intermediate-ca.* | xargs -n1 echo -e "\t"
