#!/bin/bash
set -e

find -name '*_test.go' -exec rm {} \;
find -name '*_windows.go' -exec rm {} \;
find -depth -name testdata -type d -exec rm -rf {} \;
find -depth -name testing -type d -exec rm -rf {} \;
rm -rf test

cat << EOF | sed -E 's!^([^/]+/[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \3!g' | sed -E 's!^((google|[ckv])[^/]+/[^/]+)(/[^ ]+) (.*)!\1 \4!g' > vendor.conf
# package
k8s.io/kubernetes
$(cat ./Godeps/Godeps.json | jq -r '(.Deps | .[] | "\(.ImportPath) \(.Comment) \(.Rev)\n")' | sed 's/null//' | awk '{print $1 " " $2}' | grep -v bitbucket.org/ww/goautoneg | sort -k2,1 | uniq -f1)
bitbucket.org/ww/goautoneg a547fc61f48d567d5b4ec6f8aee5573d8efce11d https://github.com/rancher/goautoneg.git
github.com/ibuildthecloud/kvsql a0e152e9c6106f43de3ec1d0303ec849fbaba861
github.com/rancher/norman 04cb04ac06975a37f11a9c805e859a76f6d6ef10
EOF

#trash 2>&1 | tee trash.log
#for i in $(grep 'level=warning msg="Package' trash.log | awk '{print $4}' | sed "s/'//g"); do
#    echo Removing $i
#    cat vendor.conf | grep -v $i > vendor.conf.new
#    mv vendor.conf.new vendor.conf
#done
trash
rm -rf Godeps trash.log trash.lock
cd vendor/k8s.io
ln -s ../../staging/src/k8s.io/* .
