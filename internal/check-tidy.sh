#!/bin/bash

set -eu

cp go.mod go.mod.untidy
cp go.sum go.sum.untidy
go mod tidy
diff -w go.mod.untidy go.mod || { echo "go.mod is not tidy"; exit 1; }
diff -w go.sum.untidy go.sum || { echo "go.sum is not tidy"; exit 1; }
rm go.mod.untidy go.sum.untidy
