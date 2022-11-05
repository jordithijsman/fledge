#!/bin/sh

# Enter project root
cd "$(dirname "$(go env GOMOD)")" || { echo "error: can't enter module directory"; exit 1; }
set -e

# Build for multiple architectures
export CGO_ENABLED=0
export GOOS=linux
for GOARCH in arm64 amd64; do
  # Build statically linked binary
  dir="./out/${GOOS}/${GOARCH}"
  # Compile debug version
  go build -o "${dir}/fledge.debug" "./cmd/fledge"
  # Strip binary and compile statically
  go build -o "${dir}/fledge" -ldflags="-s -w -extldflags=-static" "./cmd/fledge"
done
