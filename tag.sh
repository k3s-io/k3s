#!/bin/bash

if [ -z "$1" ]; then
    echo usage: $0 TAG
    exit 1
fi

F="pkg/version/base.go staging/src/k8s.io/client-go/pkg/version/base.go"
for i in $F; do
cat > $i << EOF
package version

var (
	gitMajor = "1"
	gitMinor = "12"
	gitVersion   = "$1"
	gitCommit    = "$(git rev-parse HEAD)"
	gitTreeState = "clean" 
	buildDate = "$(date -u -Iminutes)Z"
)
EOF
done

git add $F
git commit -m $1
git tag $1
