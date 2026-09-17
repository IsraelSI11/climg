#!/usr/bin/env bash
# Packages Google's precompiled static cwebp binary as a Lambda layer.
# Lambda extracts layers to /opt, and always adds /opt/bin to PATH, so the
# image-processor Lambda can invoke "cwebp" directly via os/exec.
set -euo pipefail

VERSION="1.6.0"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

curl -sL -o "$WORKDIR/libwebp.tar.gz" \
  "https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-${VERSION}-linux-x86-64.tar.gz"

tar -xzf "$WORKDIR/libwebp.tar.gz" -C "$WORKDIR"

mkdir -p "$DIR/build/bin"
cp "$WORKDIR/libwebp-${VERSION}-linux-x86-64/bin/cwebp" "$DIR/build/bin/cwebp"
chmod +x "$DIR/build/bin/cwebp"

(cd "$DIR/build" && zip -q -r -X layer.zip bin)

echo "Built $DIR/build/layer.zip"
