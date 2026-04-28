#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "==================================="
echo "Aegis Proxy MITM Setup (Linux/macOS)"
echo "==================================="
echo ""

if [ "$EUID" -ne 0 ]; then
  echo "ERROR: This script must be run as root (use sudo)"
  exit 1
fi

echo "[1/5] Building aegis binary..."
cd "$PROJECT_DIR"
go build -o aegis ./cmd/aegis
echo "✓ Binary built successfully"
echo ""

echo "[2/5] Generating CA certificate..."
./aegis mitm setup-ca
echo "✓ CA certificate generated"
echo ""

echo "[3/5] Adding entries to /etc/hosts..."
./aegis mitm setup-hosts
echo "✓ Hosts entries added"
echo ""

echo "[4/5] Installing CA to system trust store..."
./aegis mitm setup-trust
echo "✓ CA certificate installed"
echo ""

echo "[5/5] Starting MITM proxy..."
./aegis mitm start &
MITM_PID=$!
echo "✓ MITM proxy started (PID: $MITM_PID)"
echo ""

echo "==================================="
echo "Setup Complete!"
echo "==================================="
echo ""
echo "MITM Proxy is now running on port 8443"
echo ""
echo "Next steps:"
echo "1. Configure Cursor/Trae/Windsurf to use proxy:"
echo "   - HTTP Proxy: http://127.0.0.1:8443"
echo "   - HTTPS Proxy: https://127.0.0.1:8443"
echo ""
echo "2. Start the main proxy server:"
echo "   ./aegis start"
echo ""
echo "3. Test the setup:"
echo "   curl -x http://127.0.0.1:8443 https://api.openai.com/v1/models"
echo ""
echo "To stop MITM proxy:"
echo "   ./aegis mitm stop"
echo ""
echo "To cleanup (remove hosts entries and uninstall CA):"
echo "   sudo ./aegis mitm disable"
echo ""
