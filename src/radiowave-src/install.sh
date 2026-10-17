#!/usr/bin/env bash
# Radiowave installer v0.7.1
set -euo pipefail

VERSION="0.7.1"
DEST="${HOME}/.local/bin"

echo "Building Radiowave ${VERSION}..."
go build -trimpath -o radiowave .
mkdir -p "${DEST}"
install -m755 radiowave "${DEST}/radiowave"
echo "Installed ${DEST}/radiowave"
