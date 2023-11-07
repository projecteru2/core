#!/bin/bash

pkg=$1
codeDIR=$2
if [ -z "$pkg" ]; then
    echo "error: package name is null"
    echo "usage: "
    echo "     bash build-goplugin.sh pkg codeDir"
    exit 1
fi 

if [ -z "$codeDIR" ]; then
    echo "error: plugin code dir is null"
    echo "usage: "
    echo "     bash build-goplugin.sh pkg codeDir"
    exit 1
fi
echo "Building $pkg($codeDIR)"

go mod tidy
go mod vendor

tee -a go.mod <<EOF
require $pkg v0.0.0-00010101000000-000000000000
replace $pkg => $codeDIR
EOF

go build -buildmode=plugin -mod=readonly $pkg

git checkout -- go.mod