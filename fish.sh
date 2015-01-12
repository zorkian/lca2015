#!/usr/bin/fish

set -xg PATH (pwd)/go/bin $PATH
set -xg GOROOT (pwd)/go
set -xg GOPATH (pwd)/gopath
mkdir -p $GOPATH
