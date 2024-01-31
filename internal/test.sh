#!/bin/sh

set -eu

git config --global url."ssh://git@github.com/".insteadOf https://github.com/        
go test ./...
