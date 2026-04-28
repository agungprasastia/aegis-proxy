#!/bin/sh
# Setup Python environment for enowxai auth automation
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VENV_DIR="$SCRIPT_DIR/.venv"

# Colors
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() { printf "${CYAN}→${NC} %s\n" "$1"; }
success() { printf "${GREEN}✓${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}!${NC} %s\n" "$1"; }
error() { printf "${RED}✗${NC} %s\n" "$1"; exit 1; }

# Detect Python 3.10+
PYTHON=""
for cmd in python3 python; do
    if command -v "$cmd" >/dev/null 2>&1; then
        version=$("$cmd" -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')" 2>/dev/null)
        major=$(echo "$version" | cut -d. -f1)
        minor=$(echo "$version" | cut -d. -f2)
        if [ "$major" -ge 3 ] && [ "$minor" -ge 10 ]; then
            PYTHON="$cmd"
            break
        fi
    fi
done

if [ -z "$PYTHON" ]; then
    error "Python 3.10+ is required but not found. Install Python 3.10+ and try again."
fi

info "Using $PYTHON ($version)"

# Create venv if it doesn't exist
if [ ! -d "$VENV_DIR" ]; then
    info "Creating virtual environment..."
    "$PYTHON" -m venv "$VENV_DIR"
else
    info "Virtual environment already exists"
fi

# Install/upgrade pip
info "Upgrading pip..."
"$VENV_DIR/bin/pip" install --quiet --upgrade pip

# Install dependencies
info "Installing dependencies..."
"$VENV_DIR/bin/pip" install --quiet -r "$SCRIPT_DIR/requirements.txt"

# Install Camoufox browser
info "Fetching Camoufox browser (this may take a minute)..."
"$VENV_DIR/bin/python" -m camoufox fetch

success "Auth automation setup complete."
success "Python venv: $VENV_DIR"
