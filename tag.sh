#!/bin/bash
set -e

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
rm -f ./pkg/generated/openapi/zz_generated.openapi.go

# The submodule tagging screws up ./hack/update-codegen.sh so make sure the script find a valid
# semver tag
while true; do
    TAG=$(git describe --tags HEAD)
    if ! git tag -d "$TAG" 2>/dev/null; then
        break
    fi
done
git tag -d v0.0.0 2>/dev/null || true
git tag v0.0.0
trap "git tag -d v0.0.0 >/dev/null 2>&1" exit

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

git add ./pkg/generated/openapi/openapi.go 2>/dev/null || true
git add $F
git commit -m $1
git tag $1
for i in staging/src/k8s.io/*; do
    git tag -d $i/$1 2>/dev/null || true
    git tag $i/$1
done

for i in staging/src/k8s.io/*; do
    echo git push '$REMOTE' $i/$1
done
echo git push '$REMOTE' $1
