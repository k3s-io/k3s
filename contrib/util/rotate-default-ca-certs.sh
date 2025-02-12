#!/usr/bin/env bash
set -e
umask 027

# Example K3s self-signed CA rotation script.
# 
# This script will generate new self-signed root CA certificates, and cross-sign them with the
# current self-signed root CA certificates. It will then generate new leaf CA certificates
# signed by the new self-signed/cross-signed root CAs. The resulting cluster CA bundle will
# allow existing certificates to be trusted up until the original root CAs expire.
#
CONFIG="
[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer:always
basicConstraints=CA:true"
TIMESTAMP=$(date +%s)
PRODUCT="${PRODUCT:-k3s}"
DATA_DIR="${DATA_DIR:-/var/lib/rancher/${PRODUCT}}"
TEMP_DIR="${DATA_DIR}/server/rotate-ca"

if type -t openssl-3 &>/dev/null; then
  OPENSSL=openssl-3
else
  OPENSSL=openssl
fi

echo "Using $(type -p ${OPENSSL}): $(${OPENSSL} version)"

if ! ${OPENSSL} ecparam -name prime256v1 -genkey -out /dev/null &>/dev/null; then
  echo "openssl not found or missing Elliptic Curve (ecparam) support."
  exit 1
fi

${OPENSSL} version | grep -qF 'OpenSSL 3' && OPENSSL_GENRSA_FLAGS=-traditional

mkdir -p ${TEMP_DIR}/tls/etcd
cd ${TEMP_DIR}/tls

# Set up temporary openssl configuration
mkdir -p ".ca/certs"
trap "rm -rf .ca" EXIT
touch .ca/index
openssl rand -hex 8 > .ca/serial
cat >.ca/config <<'EOF'
[ca]
default_ca = ca_default
[ca_default]
dir = ./.ca
database = $dir/index
serial = $dir/serial
new_certs_dir = $dir/certs
default_md = sha256
policy = policy_anything
[policy_anything]
commonName = supplied
[req]
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, keyEncipherment, keyCertSign
EOF

for TYPE in client server request-header etcd/peer etcd/server; do
  if [ ! -f ${DATA_DIR}/server/tls/${TYPE}-ca.crt ]; then 
    echo "Current ${TYPE} CA cert does not exist; cannot continue"
    exit 1
  fi

  if [ "$(grep -cF 'END CERTIFICATE' ${DATA_DIR}/server/tls/${TYPE}-ca.crt)" -gt "1" ]; then
    awk 'BEGIN { RS = "-----END CERTIFICATE-----\n" } NR==2 { print $0 RS }' ${DATA_DIR}/server/tls/${TYPE}-ca.crt > ${TYPE}-root-old.pem
    awk 'BEGIN { RS = "-----END EC PRIVATE KEY-----\n" } NR==2 { print $0 RS }' ${DATA_DIR}/server/tls/${TYPE}-ca.key > ${TYPE}-root-old.key
  else
    cat ${DATA_DIR}/server/tls/${TYPE}-ca.crt > ${TYPE}-root-old.pem
    cat ${DATA_DIR}/server/tls/${TYPE}-ca.key > ${TYPE}-root-old.key
  fi

  CERT_NAME="${PRODUCT}-$(echo ${TYPE} | tr / -)-root"
  echo "Generating ${CERT_NAME} root and cross-signed certificate authority key and certificates"
  ${OPENSSL} ecparam -name prime256v1 -genkey -noout -out ${TYPE}-root.key
  ${OPENSSL} req -x509 -new -nodes -sha256 -days 7300 \
                 -subj "/CN=${CERT_NAME}@${TIMESTAMP}" \
                 -key ${TYPE}-root.key \
                 -out ${TYPE}-root-ssigned.pem \
                 -config ./.ca/config \
                 -extensions v3_ca
  ${OPENSSL} req -new -nodes \
                 -subj "/CN=${CERT_NAME}@${TIMESTAMP}" \
                 -key ${TYPE}-root.key |
  ${OPENSSL} ca  -batch -notext -days 7300 \
                 -in /dev/stdin \
                 -out ${TYPE}-root-xsigned.pem \
                 -keyfile ${TYPE}-root-old.key \
                 -cert ${TYPE}-root-old.pem \
                 -config ./.ca/config \
                 -extensions v3_ca
  CERT_NAME="${PRODUCT}-$(echo ${TYPE} | tr / -)-ca"
  echo "Generating ${CERT_NAME} intermediate certificate authority key and certificates"
  ${OPENSSL} ecparam -name prime256v1 -genkey -noout -out ${TYPE}-ca.key
  ${OPENSSL} req -new -nodes \
                 -subj "/CN=${CERT_NAME}@${TIMESTAMP}" \
                 -key ${TYPE}-ca.key |
  ${OPENSSL} ca  -batch -notext -days 7300 \
                 -in /dev/stdin \
                 -out ${TYPE}-ca.pem \
                 -keyfile ${TYPE}-root.key \
                 -cert ${TYPE}-root-ssigned.pem \
                 -config ./.ca/config \
                 -extensions v3_ca

  cat ${TYPE}-ca.pem \
      ${TYPE}-root-ssigned.pem \
      ${TYPE}-root-xsigned.pem \
      ${TYPE}-root-old.pem > ${TYPE}-ca.crt
  cat ${TYPE}-root.key >> ${TYPE}-ca.key
done
  
${OPENSSL} genrsa ${OPENSSL_GENRSA_FLAGS:-} -out service.key 2048
cat ${DATA_DIR}/server/tls/service.key >> service.key
  
export SERVER_CA_HASH=$(${OPENSSL} x509 -noout -fingerprint -sha256 -in server-ca.pem | awk -F= '{ gsub(/:/, "", $2); print tolower($2) }')
SERVER_TOKEN=$(awk -F:: '{print "K10" ENVIRON["SERVER_CA_HASH"] FS $2}' ${DATA_DIR}/server/token)
AGENT_TOKEN=$(awk -F:: '{print "K10" ENVIRON["SERVER_CA_HASH"] FS $2}' ${DATA_DIR}/server/agent-token)
  
echo
echo "Cross-signed CA certs and keys now available in ${TEMP_DIR}"
echo "Updated server token:    ${SERVER_TOKEN}"
echo "Updated agent token:     ${AGENT_TOKEN}"
echo
echo "To update certificates, you may now run:"
echo "    ${PRODUCT} certificate rotate-ca --path=${TEMP_DIR}"
