#!/bin/bash
set -e

F="
.bazelrc
.github
.kazelcfg.json
BUILD.bazel
CHANGELOG-1.10.md
CHANGELOG.md
CONTRIBUTING.md
Makefile
Makefile.generated_files
OWNERS
OWNERS_ALIASES
SUPPORT.md
WORKSPACE
api
build
cluster
code-of-conduct.md
docs
examples
hack
labels.yaml
logo
test
translations
"

git rm -r $F
rm -rf $F
git commit -m "Generated: Delete supporting files"
