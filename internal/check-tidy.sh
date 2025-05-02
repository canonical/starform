#!/bin/sh

set -eu

cleanup() {
	[ -f go.mod ] && mv go.mod.untidy go.mod || true
	[ -f go.sum ] && mv go.sum.untidy go.sum || true
}

cp go.mod go.mod.untidy
cp go.sum go.sum.untidy
GOPRIVATE=github.com/canonical/ go mod tidy
diff -w go.mod.untidy go.mod || { echo "go.mod is not tidy"; cleanup; exit 1; }
diff -w go.sum.untidy go.sum || { echo "go.sum is not tidy"; cleanup; exit 1; }
cleanup
