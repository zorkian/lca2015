#!/bin/bash

export PATH="$(pwd)/go/bin:$PATH"
export GOROOT="$(pwd)/go"
export GOPATH="$(pwd)/gopath"
mkdir -p $GOPATH
