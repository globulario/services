#!/bin/bash
# deploy-docs.sh — Build and deploy the Globular documentation site
#
# Usage:
#   bash scripts/deploy-docs.sh [--domain docs.globular.io] [--port 10270]
#
# Prerequisites:
#   - mkdocs-material installed: pip3 install --user mkdocs-material
#   - Wildcard cert for the domain (via globular domain add --use-wildcard-cert)
#   - Run from the services/ repository root
#
# What it does:
#   1. Builds the MkDocs site from docs/
#   2. Deploys static files to /var/lib/globular/webroot-docs/
#   3. Creates/updates the systemd unit for the docs HTTP server
#   4. Registers the domain in Globular for Envoy routing (if --domain provided)
#
# This is a Day-1 operation — run after the cluster is bootstrapped and
# external access (wildcard cert, keepalived) is configured.

set -euo pipefail

# Defaults
DOCS_DOMAIN="${DOCS_DOMAIN:-}"
DOCS_PORT="${DOCS_PORT:-10270}"
DOCS_ROOT="/var/lib/globular/webroot-docs"
DOCS_SERVICE="globular-docs"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        --domain) DOCS_DOMAIN="$2"; shift 2 ;;
        --port)   DOCS_PORT="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: $0 [--domain docs.example.com] [--port 10270]"
            echo ""
            echo "Options:"
            echo "  --domain  Register domain for Envoy routing (e.g., docs.globular.io)"
            echo "  --port    HTTP server port (default: 10270)"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "=== Globular Documentation Deployment ==="
echo ""

# Step 1: Check mkdocs is available
MKDOCS="$(command -v mkdocs 2>/dev/null || echo "$HOME/.local/bin/mkdocs")"
if [[ ! -x "$MKDOCS" ]]; then
    echo "mkdocs not found. Installing mkdocs-material..."
    pip3 install --user --break-system-packages mkdocs-material 2>/dev/null || \
    pip3 install --user mkdocs-material
    MKDOCS="$HOME/.local/bin/mkdocs"
fi

# Step 2: Build the site
echo "[1/4] Building documentation site..."
cd "$REPO_ROOT"
"$MKDOCS" build --quiet
echo "  Built: $(du -sh site/ | cut -f1) in site/"

# Step 3: Deploy static files
echo "[2/4] Deploying to $DOCS_ROOT..."
sudo mkdir -p "$DOCS_ROOT"
sudo rm -rf "${DOCS_ROOT:?}/"*
sudo cp -r site/* "$DOCS_ROOT/"
sudo chown -R globular:globular "$DOCS_ROOT"
echo "  Deployed: $(ls "$DOCS_ROOT" | wc -l) items"

# Step 4: Create/update systemd unit
echo "[3/4] Configuring systemd service on port $DOCS_PORT..."
sudo tee /etc/systemd/system/${DOCS_SERVICE}.service > /dev/null << EOF
[Unit]
Description=Globular Documentation Server
After=network.target

[Service]
Type=simple
User=globular
WorkingDirectory=${DOCS_ROOT}
ExecStart=/usr/bin/python3 -m http.server ${DOCS_PORT} --bind 0.0.0.0
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now "$DOCS_SERVICE" 2>/dev/null
sudo systemctl restart "$DOCS_SERVICE"

# Verify server is responding
sleep 2
if curl -s "http://localhost:${DOCS_PORT}/" > /dev/null 2>&1; then
    echo "  Server running on port $DOCS_PORT"
else
    echo "  WARNING: Server not responding on port $DOCS_PORT"
fi

# Step 5: Register domain (optional)
if [[ -n "$DOCS_DOMAIN" ]]; then
    echo "[4/4] Registering domain $DOCS_DOMAIN for Envoy routing..."

    # Extract zone from domain (e.g., docs.globular.io → globular.io)
    ZONE="$(echo "$DOCS_DOMAIN" | sed 's/^[^.]*\.//')"

    # Check if domain is already registered
    if globular domain status --fqdn "$DOCS_DOMAIN" > /dev/null 2>&1; then
        echo "  Domain already registered, skipping."
    else
        # Find the DNS provider for this zone
        PROVIDER="$(globular domain provider list 2>/dev/null | grep "$ZONE" | awk '{print $1}' | head -1)"
        if [[ -z "$PROVIDER" ]]; then
            echo "  WARNING: No DNS provider found for zone $ZONE"
            echo "  Register manually: globular domain add --fqdn $DOCS_DOMAIN ..."
        else
            # Get public IP
            PUBLIC_IP="$(curl -s --connect-timeout 5 https://api.ipify.org 2>/dev/null || echo "")"
            if [[ -z "$PUBLIC_IP" ]]; then
                echo "  WARNING: Could not detect public IP. Set manually with globular domain add."
            else
                # Create symlink for wildcard cert (if zone cert exists)
                ZONE_CERT_DIR="/var/lib/globular/domains/$ZONE"
                DOMAIN_CERT_DIR="/var/lib/globular/domains/$DOCS_DOMAIN"
                if [[ -f "$ZONE_CERT_DIR/fullchain.pem" ]] && [[ ! -e "$DOMAIN_CERT_DIR" ]]; then
                    sudo ln -sfn "$ZONE_CERT_DIR" "$DOMAIN_CERT_DIR"
                    echo "  Linked wildcard cert: $ZONE_CERT_DIR → $DOMAIN_CERT_DIR"
                fi

                # Get ACME email from zone domain
                ACME_EMAIL="$(globular domain status --fqdn "$ZONE" --output json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0].get('acme',{}).get('email',''))" 2>/dev/null || echo "")"
                if [[ -z "$ACME_EMAIL" ]]; then
                    ACME_EMAIL="admin@$ZONE"
                fi

                globular domain add \
                    --fqdn "$DOCS_DOMAIN" \
                    --zone "$ZONE" \
                    --provider "$PROVIDER" \
                    --target-ip "$PUBLIC_IP" \
                    --enable-acme \
                    --acme-email "$ACME_EMAIL" \
                    --enable-ingress \
                    --ingress-port "$DOCS_PORT" \
                    --ingress-service docs 2>/dev/null

                echo "  Domain registered: $DOCS_DOMAIN → docs:$DOCS_PORT"
                echo "  Envoy will route after xDS picks up the domain (~30s)"
            fi
        fi
    fi
else
    echo "[4/4] Skipping domain registration (no --domain specified)"
    echo "  Docs accessible at: http://localhost:$DOCS_PORT"
    echo "  To register: $0 --domain docs.yourdomain.com"
fi

echo ""
echo "=== Documentation deployed ==="
if [[ -n "$DOCS_DOMAIN" ]]; then
    echo "  URL: https://$DOCS_DOMAIN"
fi
echo "  Local: http://localhost:$DOCS_PORT"
echo ""
echo "To rebuild after doc changes:"
echo "  cd $REPO_ROOT && bash scripts/deploy-docs.sh${DOCS_DOMAIN:+ --domain $DOCS_DOMAIN}"
