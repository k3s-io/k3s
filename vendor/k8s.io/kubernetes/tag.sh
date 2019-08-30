#!/bin/bash

if [ -z "$1" ]; then
    echo usage: $0 TAG
    exit 1
fi

if [ -n "$(git tag -l $1)" ]; then
    echo $1 tag exists run
    echo "    " git tag -d $1
    exit 1
fi

rm -f ./pkg/generated/openapi/openapi.go

./hack/update-codegen.sh
go run ./pkg/generated/openapi/gen/main.go > ./pkg/generated/openapi/gen/openapi.go
mv ./pkg/generated/openapi/gen/openapi.go ./pkg/generated/openapi/openapi.go
rm -f ./pkg/generated/openapi/zz_generated.openapi.go

F="pkg/version/base.go staging/src/k8s.io/client-go/pkg/version/base.go"
for i in $F; do
cat > $i << EOF
package version

var (
	gitMajor = "1"
	gitMinor = "$(echo $1 | cut -f2 -d.)"
	gitVersion   = "$1"
	gitCommit    = "$(git rev-parse HEAD)"
	gitTreeState = "clean"
	buildDate = "$(date -u -Iminutes)Z"
)
EOF
done

GO111MODULE=on go list -m all | grep -v ./staging | grep '=>' | sed 's/.*=>//' | awk '{print $1 " " $2}' | sed -e 's/+incompatible//' -e 's/ v.*-.*-\(.*\)$/ \1/g' > vendor.conf

git add ./pkg/generated/openapi/openapi.go vendor.conf
git add $F
git commit -m $1
git tag $1
