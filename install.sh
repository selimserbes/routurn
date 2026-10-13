#!/bin/sh
set -eu

REPO="selimserbes/routurn"
INSTALL_DIR="${ROUTURN_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${ROUTURN_VERSION:-latest}"

fail() {
    echo "routurn install: $*" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

OS_RAW=$(uname -s)
ARCH_RAW=$(uname -m)

case "$OS_RAW" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    *) fail "unsupported operating system: $OS_RAW" ;;
esac

case "$ARCH_RAW" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) fail "unsupported architecture: $ARCH_RAW" ;;
esac

ASSET="routurn_${OS}_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
    BASE="https://github.com/$REPO/releases/latest/download"
else
    BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

need tar

if command -v curl >/dev/null 2>&1; then
    download() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
    download() { wget -q "$1" -O "$2"; }
else
    fail "curl or wget is required"
fi

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t routurn)
trap 'rm -rf "$TMP"' EXIT INT TERM HUP

echo "Installing Routurn (${VERSION}) for ${OS}/${ARCH}..."
download "$BASE/$ASSET" "$TMP/$ASSET"
download "$BASE/checksums.txt" "$TMP/checksums.txt"

EXPECTED=$(awk -v name="$ASSET" '$2 == name {print $1}' "$TMP/checksums.txt")
[ -n "$EXPECTED" ] || fail "checksum for $ASSET not found"

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$TMP/$ASSET" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')
else
    fail "sha256sum or shasum is required for verification"
fi

[ "$EXPECTED" = "$ACTUAL" ] || fail "checksum verification failed"

tar -xzf "$TMP/$ASSET" -C "$TMP"
[ -f "$TMP/routurn" ] || fail "release archive does not contain routurn"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP/routurn" "$INSTALL_DIR/routurn" 2>/dev/null || {
    cp "$TMP/routurn" "$INSTALL_DIR/routurn"
    chmod 0755 "$INSTALL_DIR/routurn"
}

echo "Installed: $INSTALL_DIR/routurn"
"$INSTALL_DIR/routurn" --version

case ":${PATH:-}:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "Note: add $INSTALL_DIR to your PATH." ;;
esac
