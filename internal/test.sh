#!/bin/sh

set -eu

git config --global url."https://$GITHUB_USER:$GITHUB_TOKEN@github.com/".insteadOf https://github.com/        
GOPRIVATE=github.com/canonical/starlark go test ./...
