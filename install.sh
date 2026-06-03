#!/usr/bin/env bash
set -euo pipefail

REPO="gustavoSoriano/nota"
BINARY="nota"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"

echo "Installing nota..."
echo "  OS:   ${OS}"
echo "  ARCH: ${ARCH}"
echo "  URL:  ${URL}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL -o "${TMPDIR}/${BINARY}" "${URL}"
chmod +x "${TMPDIR}/${BINARY}"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo "nota installed to ${INSTALL_DIR}/${BINARY}"

# Install micro editor if not present
if ! command -v micro &> /dev/null; then
  echo ""
  echo "Installing micro editor..."
  case "$OS" in
    darwin)
      if command -v brew &> /dev/null; then
        brew install micro
      else
        echo "  brew not found — install micro manually: https://micro-editor.github.io"
      fi
      ;;
    linux)
      curl -sL https://getmic.ro | bash
      if [ -w "$INSTALL_DIR" ]; then
        mv micro "$INSTALL_DIR/micro"
      else
        sudo mv micro "$INSTALL_DIR/micro"
      fi
      ;;
  esac
fi

echo ""
echo "Running nota setup..."
nota setup

echo ""
echo "Done! nota is ready to use."
echo ""
echo "Quick start:"
echo "  nota new                          # create a note"
echo "  nota save \"text\" --tags dev       # quick capture"
echo "  nota search \"query\" --json        # search notes"
echo "  nota serve                        # open web UI"
echo ""
echo "To update nota in the future:"
echo "  nota update"
