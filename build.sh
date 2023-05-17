#!/bin/sh

set -e
export GOFLAGS="-mod=vendor"

rm -rf bin
mkdir bin
env GOPRIVATE=github.com/gkhit
go build -ldflags "-s -w" -o bin/webocrd main.go
cp -r dist bin
