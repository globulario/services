#!/bin/bash
# deploy-docs.sh — Build and deploy the Globular documentation site
#
# Usage:
#   bash scripts/deploy-docs.sh
#
# The docs are served by the Envoy gateway at https://globular.io/docs/
# No separate server or domain needed — just static files in the webroot.
#
# Prerequisites:
#   - mkdocs-material installed: pip3 install --user mkdocs-material
#   - Run from the services/ repository root

set -euo pipefail

DOCS_ROOT="/var/lib/globular/webroot/docs"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Globular Documentation Deployment ==="

# Step 1: Check mkdocs is available
MKDOCS="$(command -v mkdocs 2>/dev/null || echo "$HOME/.local/bin/mkdocs")"
if [[ ! -x "$MKDOCS" ]]; then
    echo "[0/3] Installing mkdocs-material..."
    pip3 install --user --break-system-packages mkdocs-material 2>/dev/null || \
    pip3 install --user mkdocs-material
    MKDOCS="$HOME/.local/bin/mkdocs"
fi

# Step 2: Build the site
echo "[1/3] Building documentation site..."
cd "$REPO_ROOT"
"$MKDOCS" build --quiet
echo "  Built: $(du -sh site/ | cut -f1)"

# Step 3: Deploy to gateway webroot
echo "[2/3] Deploying to $DOCS_ROOT..."
sudo mkdir -p "$DOCS_ROOT"
sudo rm -rf "${DOCS_ROOT:?}/"*
sudo cp -r site/* "$DOCS_ROOT/"
sudo chown -R globular:globular "$DOCS_ROOT"

# Step 4: Verify
echo "[3/3] Verifying..."
if curl -sk https://localhost:443/docs/ 2>/dev/null | grep -q "Globular Documentation"; then
    echo "  OK: https://globular.io/docs/"
else
    echo "  Deployed to $DOCS_ROOT (verify manually)"
fi

echo ""
echo "=== Documentation deployed ==="
echo "  URL: https://globular.io/docs/"
echo ""
echo "To rebuild after doc changes:"
echo "  bash scripts/deploy-docs.sh"
