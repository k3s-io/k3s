#!/bin/bash
set -e

exit()
{
    exit=$?
    kill -9 $ETCD_PID $HYPERKUBE_PID 2>/dev/null || true
    rm -f hyperkube
    return $exit
}

trap exit EXIT

echo Compiling hyperkube
./hack/update-codegen.sh || ./hack/update-codegen.sh
go build -o hyperkube ./cmd/hyperkube
etcd &
ETCD_PID=$!
./hyperkube kube-apiserver --etcd-servers http://localhost:2379 --cert-dir $(pwd)/certs &
HYPERKUBE_PID=$!

while ! curl -f http://localhost:8080/healthz; do
    echo waiting for k8s
    sleep 1
done

curl http://localhost:8080/openapi/v2 > openapi.json
curl -H "Accept: application/com.github.proto-openapi.spec.v2@v1.0+protobuf" http://localhost:8080/openapi/v2 > openapi.pb

git add openapi.json
git add openapi.pb
git commit -m "Save openapi"
