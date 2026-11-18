#!/bin/sh
# Builds single file, dependency free executables into dist/
set -e
cd "$(dirname "$0")"
mkdir -p dist
export CGO_ENABLED=0
F="-trimpath -ldflags=-s -ldflags=-w"
GOOS=windows GOARCH=amd64 go build $F -o dist/netproof.exe .
GOOS=linux   GOARCH=amd64 go build $F -o dist/netproof-linux-amd64 .
GOOS=linux   GOARCH=arm64 go build $F -o dist/netproof-linux-arm64 .
GOOS=darwin  GOARCH=arm64 go build $F -o dist/netproof-mac-arm64 .
GOOS=darwin  GOARCH=amd64 go build $F -o dist/netproof-mac-intel .
ls -l dist
