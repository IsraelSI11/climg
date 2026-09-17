#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR/../.."

mkdir -p "$DIR/build"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$DIR/build/bootstrap" ./lambda/image-processor
(cd "$DIR/build" && zip -q -j -X function.zip bootstrap)

echo "Built $DIR/build/function.zip"
