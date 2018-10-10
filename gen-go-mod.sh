#!/bin/bash
set -e -x

VER=$1
pushd staging/src/k8s.io
LIST=$(echo *)
popd

for i in $LIST; do
    pushd staging/src/k8s.io/$i
    rm -f go.mod
    git checkout Godeps || true
    go mod init k8s.io/$i
    rm -rf Godeps

    echo 'require (' >> go.mod
    for j in $LIST; do
        echo "k8s.io/$j kubernetes-$VER" >> go.mod
    done
    echo ')' >> go.mod

    echo 'replace (' >> go.mod
    for j in $LIST; do
        echo "k8s.io/$j kubernetes-$VER => ../$j" >> go.mod
    done
    echo ')' >> go.mod

    popd
done

again=true
while [ $again = "true" ]; do
    again=false
    for i in $LIST; do
        pushd staging/src/k8s.io/$i
        go mod tidy || again=true
        popd
    done
done

rm -f go.mod
git checkout Godeps || true
go mod init k8s.io/kubernetes
echo 'require (' >> go.mod
for i in $LIST; do
    echo "k8s.io/$i kubernetes-$VER" >> go.mod
done
echo ')' >> go.mod

echo 'replace (' >> go.mod
for i in $LIST; do
    echo "k8s.io/$i kubernetes-$VER => ./staging/src/k8s.io/$i" >> go.mod
done
echo ')' >> go.mod

cat >> go.mod << EOF
require (
	github.com/coreos/etcd v3.2.24+incompatible
)
replace (
    github.com/coreos/etcd v3.2.24+incompatible => github.com/ibuildthecloud/etcd a7d7329fe0078ac4e461ce299524b874582ff354
)
EOF

go mod tidy
go mod vendor
