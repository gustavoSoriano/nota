#!/usr/bin/env bash
set -euo pipefail

REPO="soriano/nota"
BINARY="nota"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
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
        echo "Install Homebrew first: https://brew.sh"
      fi
      ;;
    linux)
      curl -sL https://getmic.ro | bash
      [ -w "$INSTALL_DIR" ] && mv micro "$INSTALL_DIR/micro" || sudo mv micro "$INSTALL_DIR/micro"
      ;;
  esac
fi

# Install ollama if not present
if ! command -v ollama &> /dev/null; then
  echo ""
  echo "Installing ollama..."
  case "$OS" in
    darwin)
      if command -v brew &> /dev/null; then
        brew install ollama
      else
        echo "Install ollama manually: https://ollama.com"
      fi
      ;;
    linux)
      curl -fsSL https://ollama.com/install.sh | sh
      ;;
  esac
fi

# Start ollama if not running
if command -v ollama &> /dev/null; then
  if ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo ""
    echo "Starting ollama..."
    ollama serve &
    sleep 3
  fi

  # Pull embedding model
  echo ""
  echo "Pulling nomic-embed-text model..."
  ollama pull nomic-embed-text:latest
fi

echo ""
echo "Running nota setup..."
nota setup

echo ""
echo "Done! nota is ready to use."
echo "Try: nota new"
echo "     nota save \"my first note\" --tags test"
echo "     nota search \"my note\" --json"
