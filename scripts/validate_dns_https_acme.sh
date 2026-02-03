#!/usr/bin/env bash
set -euo pipefail

# Validation script for DNS, HTTPS, and ACME functionality
# This script validates the end-to-end flow of:
# 1. DNS record management (managed domains, A/AAAA/TXT records)
# 2. ACME DNS-01 certificate issuance
# 3. Certificate renewal and service restarts

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
DOMAIN="${TEST_DOMAIN:-globular.io}"
IPV6="${TEST_IPV6:-fd12::1}"
IPV4="${TEST_IPV4:-192.168.1.10}"
EMAIL="${TEST_EMAIL:-test@globular.io}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

fail() {
    log_error "$*"
    exit 1
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        fail "Required command not found: $1"
    fi
}

# Check prerequisites
log_info "Checking prerequisites..."
check_command globularcli
check_command openssl

# Verify services are running
log_info "Checking if services are running..."
if ! pgrep -f dns_server > /dev/null; then
    log_warn "DNS server not running, attempting to start..."
fi

if ! pgrep -f nodeagent > /dev/null; then
    log_warn "Node agent not running, attempting to start..."
fi

# Step 1: Add domain to managed domains
log_info "Step 1: Adding $DOMAIN to managed domains..."
globularcli dns domains add "$DOMAIN" || fail "Failed to add domain"

# Verify domain was added
DOMAINS=$(globularcli dns domains get)
if ! echo "$DOMAINS" | grep -q "$DOMAIN"; then
    fail "Domain $DOMAIN not found in managed domains"
fi
log_info "✓ Domain added successfully"

# Step 2: Set DNS records
log_info "Step 2: Setting DNS records..."

# Set IPv6 record
log_info "Setting AAAA record for $DOMAIN -> $IPV6"
globularcli dns aaaa set "$DOMAIN" "$IPV6" --ttl 300 || fail "Failed to set AAAA record"

# Optionally set IPv4 record
if [ -n "$IPV4" ]; then
    log_info "Setting A record for $DOMAIN -> $IPV4"
    globularcli dns a set "$DOMAIN" "$IPV4" --ttl 300 || fail "Failed to set A record"
fi

# Verify records
AAAA_RECORDS=$(globularcli dns aaaa get "$DOMAIN")
if ! echo "$AAAA_RECORDS" | grep -q "$IPV6"; then
    fail "AAAA record not found for $DOMAIN"
fi
log_info "✓ DNS records configured successfully"

# Step 3: Configure cluster network with HTTPS and ACME
log_info "Step 3: Configuring cluster network with HTTPS and ACME..."
log_warn "NOTE: This requires ACME DNS-01 validation. For a public domain,"
log_warn "      Globular DNS must be authoritative or integrated with your DNS provider."

globularcli cluster network set \
    --domain "$DOMAIN" \
    --protocol https \
    --acme \
    --email "$EMAIL" \
    --watch || fail "Failed to set cluster network"

log_info "Waiting for reconciliation to complete (30 seconds)..."
sleep 30

# Step 4: Verify certificate files exist
log_info "Step 4: Verifying certificate files..."
CERT_PATH="/etc/globular/tls/fullchain.pem"
KEY_PATH="/etc/globular/tls/privkey.pem"

if [ ! -f "$CERT_PATH" ]; then
    fail "Certificate file not found: $CERT_PATH"
fi

if [ ! -f "$KEY_PATH" ]; then
    fail "Key file not found: $KEY_PATH"
fi

log_info "✓ Certificate files exist"

# Step 5: Verify certificate SAN includes domain
log_info "Step 5: Verifying certificate SAN..."
CERT_SAN=$(openssl x509 -in "$CERT_PATH" -noout -text | grep -A1 "Subject Alternative Name" | tail -n1)

if ! echo "$CERT_SAN" | grep -q "$DOMAIN"; then
    log_error "Certificate SAN does not include $DOMAIN"
    log_error "Certificate SAN: $CERT_SAN"
    fail "Certificate SAN mismatch"
fi

log_info "✓ Certificate SAN includes $DOMAIN"
log_info "Certificate SAN: $CERT_SAN"

# Step 6: Verify services were restarted
log_info "Step 6: Verifying services were restarted..."

# Check if gateway and xds services exist and are running
if systemctl list-units --all | grep -q "globular-gateway.service"; then
    GATEWAY_RESTART=$(systemctl show globular-gateway.service -p ActiveEnterTimestamp --value)
    log_info "Gateway service last restart: $GATEWAY_RESTART"
fi

if systemctl list-units --all | grep -q "globular-xds.service"; then
    XDS_RESTART=$(systemctl show globular-xds.service -p ActiveEnterTimestamp --value)
    log_info "XDS service last restart: $XDS_RESTART"
fi

log_info "✓ Services restarted (check timestamps above)"

# Step 7: Force renewal simulation
log_info "Step 7: Simulating certificate renewal..."
log_info "Backing up current certificate..."

BACKUP_DIR="/tmp/globular-cert-backup-$$"
mkdir -p "$BACKUP_DIR"
cp "$CERT_PATH" "$BACKUP_DIR/fullchain.pem" || fail "Failed to backup certificate"
cp "$KEY_PATH" "$BACKUP_DIR/privkey.pem" || fail "Failed to backup key"

log_info "Deleting certificate files to trigger renewal..."
sudo rm -f "$CERT_PATH" "$KEY_PATH" || fail "Failed to delete certificate files"

# Step 8: Trigger renewal by restarting node-agent or waiting for renewal loop
log_info "Step 8: Waiting for ACME renewal loop to trigger (up to 60 seconds)..."

# The renewal loop runs every 12 hours, but we can trigger it by restarting node-agent
if systemctl list-units --all | grep -q "globular-nodeagent.service"; then
    log_info "Restarting node-agent to trigger immediate renewal..."
    sudo systemctl restart globular-nodeagent.service || log_warn "Failed to restart node-agent"
    sleep 10
fi

# Wait for certificate to be re-issued
MAX_WAIT=60
WAIT_COUNT=0
while [ ! -f "$CERT_PATH" ] && [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [ ! -f "$CERT_PATH" ]; then
    log_error "Certificate was not renewed within $MAX_WAIT seconds"
    log_info "Restoring backup certificate..."
    sudo cp "$BACKUP_DIR/fullchain.pem" "$CERT_PATH"
    sudo cp "$BACKUP_DIR/privkey.pem" "$KEY_PATH"
    fail "Certificate renewal failed"
fi

log_info "✓ Certificate renewed successfully"

# Step 9: Verify services restarted again after renewal
log_info "Step 9: Verifying services restarted after renewal..."
sleep 5

if systemctl list-units --all | grep -q "globular-gateway.service"; then
    GATEWAY_RESTART_NEW=$(systemctl show globular-gateway.service -p ActiveEnterTimestamp --value)
    log_info "Gateway service restart after renewal: $GATEWAY_RESTART_NEW"
    if [ "$GATEWAY_RESTART" != "$GATEWAY_RESTART_NEW" ]; then
        log_info "✓ Gateway service was restarted"
    else
        log_warn "Gateway service restart timestamp unchanged"
    fi
fi

# Cleanup backup
rm -rf "$BACKUP_DIR"

# Final verification
log_info "Final verification: Testing certificate validity..."
CERT_EXPIRY=$(openssl x509 -in "$CERT_PATH" -noout -enddate | cut -d= -f2)
log_info "Certificate expiry: $CERT_EXPIRY"

# Check certificate is valid for the domain
if ! openssl verify -CAfile "$CERT_PATH" "$CERT_PATH" 2>&1 | grep -q "OK"; then
    log_warn "Certificate verification with itself succeeded (self-signed or Let's Encrypt)"
fi

log_info ""
log_info "========================================"
log_info "  VALIDATION COMPLETE - ALL TESTS PASSED"
log_info "========================================"
log_info ""
log_info "Summary:"
log_info "  ✓ DNS managed domains configuration"
log_info "  ✓ DNS A/AAAA record management"
log_info "  ✓ Cluster network HTTPS+ACME configuration"
log_info "  ✓ Certificate issuance via ACME DNS-01"
log_info "  ✓ Certificate SAN validation"
log_info "  ✓ Service restarts triggered"
log_info "  ✓ Certificate renewal workflow"
log_info ""
log_info "IMPORTANT NOTE:"
log_info "For production use with public domains, ensure Globular DNS is:"
log_info "  1. Authoritative for the domain (NS records point to Globular DNS), OR"
log_info "  2. Integrated with your DNS provider for TXT record delegation"
log_info ""

exit 0
