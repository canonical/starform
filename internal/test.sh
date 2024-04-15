#!/bin/sh

set -eu

echo "Trying to clone with all possibilities:" 
false || \
    git clone https://$GITHUB_TOKEN@github.com/canonical/starlark.git || \
    git clone https://$GITHUB_TOKEN:@github.com/canonical/starlark.git || \
    git clone https://oauth2:$GITHUB_TOKEN@github.com/canonical/starlark.git || \
    git clone https://x-access-token:$GITHUB_TOKEN@github.com/canonical/starlark.git || \
    git clone https://oauth2:$GITHUB_TOKEN@github.com/canonical/starlark.git || 
    echo "All failed! :-("


echo "Now trying with netrc"

echo "machine github.com login $GITHUB_USER password $GITHUB_TOKEN" > ~/.netrc
git clone https://github.com/canonical/starlark.git 

# git config --global url."https://x-access-token:$GITHUB_TOKEN@github.com/".insteadOf https://github.com/        
# GOPRIVATE=github.com/canonical/starlark go test ./...
