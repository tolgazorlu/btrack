#!/usr/bin/env bash
# btrack installer for macOS and Linux
set -euo pipefail

REPO="tolgaozgun/btrack"
BINARY="btrack"
INSTALL_DIR="/usr/local/bin"

detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
  esac
  echo "${OS}-${ARCH}"
}

PLATFORM=$(detect_platform)
echo "→ Detected platform: $PLATFORM"

LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

echo "→ Downloading btrack $LATEST..."
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}-${PLATFORM}"
curl -fsSL "$DOWNLOAD_URL" -o "/tmp/$BINARY"
chmod +x "/tmp/$BINARY"

echo "→ Installing to $INSTALL_DIR/$BINARY"
if [[ -w "$INSTALL_DIR" ]]; then
  mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "✓ btrack $LATEST installed successfully!"
echo ""
echo "  Get started:"
echo "    btrack start \"my first task\""
echo "    btrack log \"making progress\""
echo "    btrack stop -m \"completed the task #feature\""
echo ""
echo "  Config: ~/.config/btrack/config.yaml"
