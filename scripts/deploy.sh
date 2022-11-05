#!/bin/sh

# Enter project root
cd "$(dirname "$(go env GOMOD)}")" || { echo "error: can't enter module directory"; exit 1; }
set -e

# Build binary
"./scripts/build.sh"

# Copy binary
ssh worker0 pkill fledge || true
scp "./out/$(go env GOHOSTOS)/$(go env GOHOSTARCH)/fledge" "worker0:~"

# Run FLEDGE
ssh worker0 KUBERNETES_SERVICE_HOST=10.2.0.132 KUBERNETES_SERVICE_PORT=6443 ./fledge $@
