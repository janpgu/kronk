#!/bin/sh
# kronk installer for Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/janpgu/kronk/main/install.sh | sh

set -e

REPO="janpgu/kronk"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colour helpers — degrade gracefully if the terminal doesn't support them.
if [ -t 1 ]; then
    GREEN="\033[32m"; CYAN="\033[36m"; RED="\033[31m"; RESET="\033[0m"; BOLD="\033[1m"
else
    GREEN=""; CYAN=""; RED=""; RESET=""; BOLD=""
fi

step() { printf "${CYAN}  --> ${RESET}%s\n" "$1"; }
ok()   { printf "${GREEN}  OK  ${RESET}%s\n" "$1"; }
fail() { printf "${RED}  ERR  ${RESET}%s\n" "$1"; exit 1; }

echo ""
printf "${BOLD}kronk installer${RESET}\n"
printf "---------------\n\n"

# --- Detect OS and architecture ---
step "Detecting platform..."
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)
        case "$ARCH" in
            x86_64)  PLATFORM="linux-amd64" ;;
            aarch64) PLATFORM="linux-arm64" ;;
            *)        fail "Unsupported architecture: $ARCH" ;;
        esac
        ;;
    Darwin)
        case "$ARCH" in
            x86_64)  PLATFORM="darwin-amd64" ;;
            arm64)   PLATFORM="darwin-arm64" ;;
            *)        fail "Unsupported architecture: $ARCH" ;;
        esac
        ;;
    *)
        fail "Unsupported OS: $OS. Use install.ps1 on Windows."
        ;;
esac
ok "Platform: $PLATFORM"

# --- Resolve latest version ---
step "Resolving latest version..."
if command -v curl > /dev/null 2>&1; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
elif command -v wget > /dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
else
    fail "Neither curl nor wget found. Install one and try again."
fi

[ -z "$VERSION" ] && fail "Could not determine latest version."
ok "Version: $VERSION"

# --- Download binary ---
URL="https://github.com/$REPO/releases/download/$VERSION/kronk-$PLATFORM"
TMP="$(mktemp)"
step "Downloading $URL..."
if command -v curl > /dev/null 2>&1; then
    curl -fsSL "$URL" -o "$TMP"
else
    wget -qO "$TMP" "$URL"
fi
chmod +x "$TMP"
ok "Downloaded"

# --- Install binary ---
step "Installing to $INSTALL_DIR..."
if [ ! -w "$INSTALL_DIR" ]; then
    # Try with sudo if the directory isn't writable.
    if command -v sudo > /dev/null 2>&1; then
        sudo mv "$TMP" "$INSTALL_DIR/kronk"
        sudo chmod +x "$INSTALL_DIR/kronk"
    else
        fail "$INSTALL_DIR is not writable and sudo is not available. Set INSTALL_DIR to a writable path and retry."
    fi
else
    mv "$TMP" "$INSTALL_DIR/kronk"
fi
ok "Installed: $INSTALL_DIR/kronk"

# --- Add crontab entry ---
step "Checking crontab..."
CRON_LINE="* * * * * $INSTALL_DIR/kronk tick"

if crontab -l 2>/dev/null | grep -qF "$INSTALL_DIR/kronk tick"; then
    ok "Crontab entry already present"
else
    ( crontab -l 2>/dev/null; echo "$CRON_LINE" ) | crontab -
    ok "Added crontab entry: $CRON_LINE"
fi

# --- Done ---
echo ""
printf "${GREEN}${BOLD}kronk $VERSION installed.${RESET}\n\n"
echo "Quick start:"
echo "  kronk add \"echo hello\" --name hello --schedule \"every night\""
echo "  kronk status"
echo "  kronk history"
echo ""
echo "Run 'kronk doctor' to verify your setup."
echo ""
