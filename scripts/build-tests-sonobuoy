#!/bin/bash
set -e

cd $(dirname $0)/..

REPO="k3s-int-tests"
OUTFILE="./dist/artifacts/k3s-int-tests.yaml"

# Compile all integration tests and containerize them
mkdir -p dist/artifacts

# Integration tests under /pkg
PKG_TO_TEST=$(find ./pkg/ -type f -name "*_int_test.go" | sed -r 's|/[^/]+$||' |sort -u)
for i in $PKG_TO_TEST; do
    name=$(echo "${i##*/}")
    echo $name
    go test -c -ldflags "-X 'github.com/k3s-io/k3s/tests/integration.existingServer=True'" -o dist/artifacts/k3s-integration-$name.test $i -run Integration -ginkgo.v -test.v
done

# Integration tests under /tests
PKG_TO_TEST=$(find ./tests/integration -type f -name "*_int_test.go" | sed -r 's|/[^/]+$||' |sort -u)
for i in $PKG_TO_TEST; do
    name=$(echo "${i##*/}")
    echo $name
    go test -c -ldflags "-X 'github.com/k3s-io/k3s/tests/integration.existingServer=True'" -o dist/artifacts/k3s-integration-$name.test $i -run Integration -ginkgo.v -test.v
done
docker build -f ./tests/integration/Dockerfile.test -t $REPO .
docker save $REPO -o ./dist/artifacts/$REPO.tar

sudo mkdir -p /var/lib/rancher/k3s/agent/images
sudo mv ./dist/artifacts/$REPO.tar /var/lib/rancher/k3s/agent/images/

# If k3s is already running, attempt to import the image
if [[ "$(pgrep k3s | wc -l)" -gt 0 ]]; then
    sudo ./dist/artifacts/k3s ctr images import /var/lib/rancher/k3s/agent/images/$REPO.tar
fi

# Cleanup compiled tests
rm dist/artifacts/k3s-integration-*

# Generate the sonobuoy plugin and inject the necessary 
# podSpec and volume mount modifications
PODSPEC=\
'  hostNetwork: true
  volumes:
  - name: var-k3s
    hostPath:
      path: /var/lib/rancher/k3s/
      type: Directory
  - name: etc-k3s
    hostPath:
      path: /etc/rancher/k3s/
      type: Directory'
VOLMOUNTS=\
'  - mountPath: /var/lib/rancher/k3s/
    name: var-k3s
  - mountPath: /etc/rancher/k3s/
    name: etc-k3s'

sonobuoy gen plugin \
    --format=junit \
    --image ${REPO} \
    --show-default-podspec \
    --name k3s-int \
    --type job \
    --cmd ./test-runner.sh \
    --env KUBECONFIG=/etc/rancher/k3s/k3s.yaml \
    > $OUTFILE
awk -v PS="$PODSPEC" '/podSpec:/{print;print PS;next}1' $OUTFILE > ./dist/artifacts/temp.yaml
mv ./dist/artifacts/temp.yaml $OUTFILE
awk -v VM="$VOLMOUNTS" '/volumeMounts:/{print;print VM;next}1' $OUTFILE  > ./dist/artifacts/temp.yaml
mv ./dist/artifacts/temp.yaml $OUTFILE
