#!/bin/bash

if [ ! -d $1/staging/src/k8s.io ]; then
    echo Kubernetes source not found at $1
    exit 1
fi

cd $(dirname $0)/../vendor/k8s.io
for i in $1/staging/src/k8s.io/*; do
    rm -rf $(basename $i)
    ln -s $i .
done
rm -rf kubernetes
mkdir -p kubernetes
cd kubernetes
ln -s $1/{cmd,pkg,plugin,third_party,openapi.json,openapi.pb} .
