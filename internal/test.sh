#!/bin/sh

set -eu

GOPRIVATE=github.com/canonical/starlark go test ./...
