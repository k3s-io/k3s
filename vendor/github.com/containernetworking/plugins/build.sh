#!/usr/bin/env bash
set -e

if [ "$(uname)" == "Darwin" ]; then
	export GOOS=linux
fi

ORG_PATH="github.com/containernetworking"
export REPO_PATH="${ORG_PATH}/plugins"

if [ ! -h gopath/src/${REPO_PATH} ]; then
	mkdir -p gopath/src/${ORG_PATH}
	ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
fi

export GO15VENDOREXPERIMENT=1
export GOPATH=${PWD}/gopath

mkdir -p "${PWD}/bin"

echo "Building plugins"
PLUGINS="plugins/meta/* plugins/main/* plugins/ipam/* plugins/sample"
for d in $PLUGINS; do
	if [ -d "$d" ]; then
		plugin="$(basename "$d")"
		echo "  $plugin"
		# use go install so we don't duplicate work
		if [ -n "$FASTBUILD" ]
		then
			GOBIN=${PWD}/bin go install -pkgdir $GOPATH/pkg "$@" $REPO_PATH/$d
		else
			go build -o "${PWD}/bin/$plugin" -pkgdir "$GOPATH/pkg" "$@" "$REPO_PATH/$d"
		fi
	fi
done
