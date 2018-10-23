#!/bin/bash
set -e

git rm -rf test
find -name '*_test.go' -exec git rm {} \;
find -name '*_windows.go' -exec git rm {} \;
find -depth -name testdata -type d -exec git rm -rf {} \;
find -depth -name testing -type d -exec git rm -rf {} \;

cat << EOF | sed -E 's!^([^/]+/[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \3!g' | sed -E 's!^((google|[ckv])[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \4!g' > vendor.conf
# package
k8s.io/kubernetes
$(cat ./Godeps/Godeps.json | jq -r '(.Deps | .[] | "\(.ImportPath) \(.Comment) \(.Rev)\n")' | sed 's/null//' | awk '{print $1 " " $2}' | grep -v bitbucket.org/ww/goautoneg | sort -k2,1 | uniq -f1)
bitbucket.org/ww/goautoneg a547fc61f48d567d5b4ec6f8aee5573d8efce11d https://github.com/rancher/goautoneg.git
github.com/ibuildthecloud/kvsql 93ec16ba63d05c14c5ffdd7ad31b6eefb49c9e21
EOF

trash
git rm -rf Godeps
rm trash.lock
cd vendor/k8s.io
ln -s ../../staging/src/k8s.io/* .
cd ../..
git add vendor vendor.conf
go build ./cmd/hyperkube
go build
rm hyperkube
rm $(basename $(pwd))
git commit -m "Update vendor"
