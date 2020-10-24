#!/bin/bash

cd $(dirname $0)/..

rm -rf build/data
mkdir -p build/data

GO=${GO-go}

echo Running: "${GO}" generate
"${GO}" generate
