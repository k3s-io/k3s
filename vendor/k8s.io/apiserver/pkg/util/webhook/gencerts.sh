#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# gencerts.sh generates the certificates for the webhook tests.
#
# It is not expected to be run often (there is no go generate rule), and mainly
# exists for documentation purposes.

CN_BASE="webhook_tests"

cat > server.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
DNS.1 = localhost
EOF

cat > server_no_san.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
EOF

cat > client.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
EOF

# Create a certificate authority
openssl genrsa -out caKey.pem 2048
openssl req -x509 -new -nodes -key caKey.pem -days 100000 -out caCert.pem -subj "/CN=${CN_BASE}_ca"

# Create a second certificate authority
openssl genrsa -out badCAKey.pem 2048
openssl req -x509 -new -nodes -key badCAKey.pem -days 100000 -out badCACert.pem -subj "/CN=${CN_BASE}_ca"

# Create a server certiticate
openssl genrsa -out serverKey.pem 2048
openssl req -new -key serverKey.pem -out server.csr -subj "/CN=${CN_BASE}_server" -config server.conf
openssl x509 -req -in server.csr -CA caCert.pem -CAkey caKey.pem -CAcreateserial -out serverCert.pem -days 100000 -extensions v3_req -extfile server.conf

# Create a server certiticate w/o SAN
openssl req -new -key serverKey.pem -out serverNoSAN.csr -subj "/CN=localhost" -config server_no_san.conf
openssl x509 -req -in serverNoSAN.csr -CA caCert.pem -CAkey caKey.pem -CAcreateserial -out serverCertNoSAN.pem -days 100000 -extensions v3_req -extfile server_no_san.conf

# Create a client certiticate
openssl genrsa -out clientKey.pem 2048
openssl req -new -key clientKey.pem -out client.csr -subj "/CN=${CN_BASE}_client" -config client.conf
openssl x509 -req -in client.csr -CA caCert.pem -CAkey caKey.pem -CAcreateserial -out clientCert.pem -days 100000 -extensions v3_req -extfile client.conf

outfile=certs_test.go

cat > $outfile << EOF
/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file was generated using openssl by the gencerts.sh script
// and holds raw certificates for the webhook tests.

package webhook
EOF

for file in caKey caCert badCAKey badCACert serverKey serverCert serverCertNoSAN clientKey clientCert; do
	data=$(cat ${file}.pem)
	echo "" >> $outfile
	echo "var $file = []byte(\`$data\`)" >> $outfile
done

# Clean up after we're done.
rm ./*.pem
rm ./*.csr
rm ./*.srl
rm ./*.conf
