#!/bin/bash

set -e

# Configuration
REPO="funvibe/funxy"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="funxy"
LSP_BIN_NAME="funxy-lsp"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# 1. Detect OS and Arch
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
    Linux*)     OS='linux';;
    Darwin*)    OS='darwin';;
    OpenBSD*)   OS='openbsd';;
    *)          error "Unsupported operating system: ${OS}";;
esac

case "${ARCH}" in
    x86_64)    ARCH='amd64';;
    arm64|aarch64) ARCH='arm64';;
    *)          error "Unsupported architecture: ${ARCH}";;
esac

log "Detected system: $OS/$ARCH"

# 2. Find latest version
log "Checking latest version..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    error "Could not find latest release for $REPO. Check internet connection or repository name."
fi

log "Latest version is: $LATEST_TAG"

# 3. Download binaries
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

download_binary() {
    local binary_name=$1
    local download_name="${binary_name}-${OS}-${ARCH}"
    local url="https://github.com/$REPO/releases/download/$LATEST_TAG/$download_name"

    log "Downloading $binary_name from $url..."

    # Try downloading directly (assuming raw binary)
    if curl -L -f -o "$TMP_DIR/$binary_name" "$url"; then
        log "Download complete."
    else
        # Fallback: check if it's a tar.gz archive
        url="${url}.tar.gz"
        log "Binary not found, trying archive: $url..."
        if curl -L -f -o "$TMP_DIR/$binary_name.tar.gz" "$url"; then
            tar -xzf "$TMP_DIR/$binary_name.tar.gz" -C "$TMP_DIR"
            log "Download and extraction complete."
        else
            error "Failed to download $binary_name. Asset not found."
        fi
    fi

    chmod +x "$TMP_DIR/$binary_name"
}

download_binary "$BIN_NAME"

# Attempt to download LSP (it might not verify if it's packaged differently, but we try)
log "Attempting to download LSP..."
if download_binary "$LSP_BIN_NAME"; then
    HAS_LSP=true
else
    log "LSP binary not found in release. Skipping."
    HAS_LSP=false
fi

# 4. Choose install directory
echo ""
printf "Install to ${BLUE}$INSTALL_DIR${NC}? [Y/n] "
read -r answer
case "$answer" in
    [nN]*)
        printf "Enter install directory: "
        read -r custom_dir
        if [ -z "$custom_dir" ]; then
            error "No directory specified"
        fi
        # Expand ~ to home directory
        INSTALL_DIR="${custom_dir/#\~/$HOME}"
        ;;
esac

# Create directory if needed
mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"

# Install
log "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
    if [ "$HAS_LSP" = true ]; then
        mv "$TMP_DIR/$LSP_BIN_NAME" "$INSTALL_DIR/$LSP_BIN_NAME"
    fi
else
    log "Requires sudo for $INSTALL_DIR"
    sudo mv "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
    if [ "$HAS_LSP" = true ]; then
        sudo mv "$TMP_DIR/$LSP_BIN_NAME" "$INSTALL_DIR/$LSP_BIN_NAME"
    fi
fi

echo ""
if [ -f "$INSTALL_DIR/$BIN_NAME" ]; then
    success "✓ $INSTALL_DIR/$BIN_NAME"
else
    error "$INSTALL_DIR/$BIN_NAME not found"
fi
if [ "$HAS_LSP" = true ]; then
    if [ -f "$INSTALL_DIR/$LSP_BIN_NAME" ]; then
        success "✓ $INSTALL_DIR/$LSP_BIN_NAME"
    else
        error "$INSTALL_DIR/$LSP_BIN_NAME not found"
    fi
fi
