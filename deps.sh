#!/bin/bash
set -e

find -name '*_test.go' -exec git rm {} \;
find -name '*_windows.go' -exec git rm {} \;
find -depth -name testdata -type d -exec git rm -rf {} \;
find -depth -name testing -type d -exec git rm -rf {} \;
git rm -rf test

cat << EOF | sed -E 's!^([^/]+/[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \3!g' | sed -E 's!^((google|[ckv])[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \4!g' > vendor.conf
# package
k8s.io/kubernetes
$(cat ./Godeps/Godeps.json | jq -r '(.Deps | .[] | "\(.ImportPath) \(.Comment) \(.Rev)\n")' | sed 's/null//' | awk '{print $1 " " $2}' | grep -v bitbucket.org/ww/goautoneg | sort -k2,1 | uniq -f1)
bitbucket.org/ww/goautoneg a547fc61f48d567d5b4ec6f8aee5573d8efce11d https://github.com/rancher/goautoneg.git
github.com/ibuildthecloud/kvsql f76ad0737dfb07291925e7c78521dc07ec5506e2
github.com/rancher/norman 04cb04ac06975a37f11a9c805e859a76f6d6ef10
EOF

trash
git rm -rf Godeps
rm trash.lock
cd vendor/k8s.io
ln -s ../../staging/src/k8s.io/* .
cd ../..
git add vendor vendor.conf
git commit -m "Update vendor"
