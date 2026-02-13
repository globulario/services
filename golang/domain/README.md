# External Domain Management

Automated management of external domains with DNS record provisioning and ACME certificate acquisition for public-facing Globular nodes.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Use Cases](#use-cases)
- [CLI Commands](#cli-commands)
- [DNS Providers](#dns-providers)
- [Workflows](#workflows)
- [Troubleshooting](#troubleshooting)

## Overview

The external domain system enables Globular nodes to be accessible via public FQDNs (e.g., `api.example.com`) with:

- **Automatic DNS management** via provider APIs (Route53, GoDaddy, etc.)
- **ACME certificate acquisition** via Let's Encrypt DNS-01 challenge
- **Ingress routing** via Envoy SNI (when enabled)

### Architecture

```
User → globular CLI
         ↓
    ExternalDomainSpec → etcd
         ↓
    Domain Reconciler
         ↓ (DNS)          ↓ (ACME)
    DNS Provider    Let's Encrypt
         ↓                ↓
    A Record        Certificate
    Created         Obtained
         ↓                ↓
    /var/lib/globular/domains/<fqdn>/
    ├── fullchain.pem
    ├── privkey.pem
    └── chain.pem
```

## Quick Start

### 1. Configure DNS Provider

**GoDaddy Example:**
```bash
# Set credentials
export GODADDY_API_KEY="your-api-key"
export GODADDY_API_SECRET="your-api-secret"

# Add provider
globular dns provider add \
  --name my-godaddy \
  --type godaddy \
  --zone example.com
```

**Route53 Example (with IAM role):**
```bash
# No credentials needed if running on EC2 with IAM role
globular dns provider add \
  --name my-route53 \
  --type route53 \
  --zone example.com
```

### 2. Register External Domain

```bash
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider my-godaddy \
  --target-ip auto \
  --enable-acme \
  --acme-email ops@example.com
```

### 3. Check Status

```bash
globular domain status --fqdn api.example.com

# Output:
# FQDN              DNS    CERT          INGRESS  UPDATED
# api.example.com   ✓ ok   ✓ valid (89d) ✓ ready  2m ago
```

### 4. Access Your Node

```bash
curl https://api.example.com/ServiceName/Method
```

## Use Cases

### Use Case 1: Public API Gateway

**Scenario:** Expose Globular gRPC services via public HTTPS endpoint

**Setup:**
```bash
# 1. Configure DNS provider
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"

globular dns provider add \
  --name production-route53 \
  --type route53 \
  --zone mycompany.com

# 2. Register domain
globular domain add \
  --fqdn api.mycompany.com \
  --zone mycompany.com \
  --provider production-route53 \
  --target-ip auto \
  --enable-acme \
  --acme-email ops@mycompany.com

# 3. Wait for reconciliation (check status)
globular domain status --fqdn api.mycompany.com

# 4. Access via HTTPS
curl https://api.mycompany.com/authentication.AuthenticationService/Validate
```

**Result:**
- DNS A record created automatically
- Let's Encrypt certificate obtained
- HTTPS endpoint accessible worldwide

### Use Case 2: Multi-Node Cluster with Node-Specific FQDNs

**Scenario:** Each node in cluster has its own public FQDN

**Node 1 Setup:**
```bash
# On node-01
globular domain add \
  --fqdn node-01.cluster.example.com \
  --zone cluster.example.com \
  --provider my-route53 \
  --target-ip auto \
  --node-id node-01 \
  --enable-acme \
  --acme-email cluster@example.com
```

**Node 2 Setup:**
```bash
# On node-02
globular domain add \
  --fqdn node-02.cluster.example.com \
  --zone cluster.example.com \
  --provider my-route53 \
  --target-ip auto \
  --node-id node-02 \
  --enable-acme \
  --acme-email cluster@example.com
```

**Result:**
- Each node accessible via dedicated FQDN
- Separate certificates per node
- Load balancing via DNS round-robin possible

### Use Case 3: Development/Staging/Production Environments

**Scenario:** Different domains for different environments

**Development:**
```bash
globular domain add \
  --fqdn dev-api.example.com \
  --zone example.com \
  --provider my-route53 \
  --target-ip auto \
  --enable-acme \
  --acme-email dev@example.com \
  --acme-directory staging  # Use Let's Encrypt staging
```

**Staging:**
```bash
globular domain add \
  --fqdn staging-api.example.com \
  --zone example.com \
  --provider my-route53 \
  --target-ip auto \
  --enable-acme \
  --acme-email staging@example.com
```

**Production:**
```bash
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider my-route53 \
  --target-ip 203.0.113.42 \  # Explicit IP
  --enable-acme \
  --acme-email ops@example.com
```

### Use Case 4: Manual DNS with ACME

**Scenario:** Enterprise with change control process

**Setup:**
```bash
# 1. Use manual provider
globular dns provider add \
  --name manual \
  --type manual \
  --zone example.com

# 2. Register domain (won't create DNS automatically)
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider manual \
  --target-ip 203.0.113.42 \
  --enable-acme \
  --acme-email ops@example.com

# 3. Manual provider will print DNS operations
# Output:
# ╔════════════════════════════════════════════════════════╗
# ║ DNS Operation Required (Manual Provider)              ║
# ╠════════════════════════════════════════════════════════╣
# ║ Operation: Create A Record                            ║
# ║ Zone:      example.com                                ║
# ║ Name:      api                                         ║
# ║ Value:     203.0.113.42                               ║
# ║ TTL:       600                                         ║
# ╚════════════════════════════════════════════════════════╝

# 4. Execute DNS changes via your change control process

# 5. ACME certificate will still be obtained automatically
```

**Result:**
- DNS changes go through approval process
- ACME still automated (using manual provider's DNS-01 implementation)
- Full audit trail

## CLI Commands

### `globular domain`

Main command for external domain management.

#### `domain add` - Register External Domain

**Syntax:**
```bash
globular domain add \
  --fqdn <domain> \
  --zone <zone> \
  --provider <name> \
  [--target-ip <ip|auto>] \
  [--ttl <seconds>] \
  [--node-id <id>] \
  [--enable-acme] \
  [--acme-email <email>] \
  [--acme-directory <url|staging>] \
  [--enable-ingress] \
  [--ingress-service <name>] \
  [--ingress-port <port>]
```

**Required Flags:**
- `--fqdn` - Fully-qualified domain name (e.g., `api.example.com`)
- `--zone` - DNS zone (e.g., `example.com`)
- `--provider` - DNS provider name (must be configured first)

**Optional Flags:**
- `--target-ip` - Target IP address or `auto` (default: auto)
- `--ttl` - DNS TTL in seconds (default: 600)
- `--node-id` - Node identifier (default: auto-detect from hostname)
- `--enable-acme` - Enable ACME certificate acquisition (default: false)
- `--acme-email` - ACME account email (required if --enable-acme)
- `--acme-directory` - ACME directory: empty (prod), `staging`, or custom URL
- `--enable-ingress` - Enable Envoy ingress routing (default: false)
- `--ingress-service` - Backend service name (default: gateway)
- `--ingress-port` - Backend port (default: 8080)

**Examples:**

**Minimal (DNS only):**
```bash
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider my-godaddy \
  --target-ip 203.0.113.42
```

**With ACME:**
```bash
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider my-route53 \
  --target-ip auto \
  --enable-acme \
  --acme-email ops@example.com
```

**Full Featured:**
```bash
globular domain add \
  --fqdn api.example.com \
  --zone example.com \
  --provider my-route53 \
  --target-ip auto \
  --ttl 300 \
  --node-id globule-node-01 \
  --enable-acme \
  --acme-email ops@example.com \
  --enable-ingress \
  --ingress-service gateway \
  --ingress-port 8080
```

#### `domain status` - Check Domain Status

**Syntax:**
```bash
globular domain status [--fqdn <domain>]
```

**Flags:**
- `--fqdn` - Show specific domain (omit to show all)

**Examples:**

**List all domains:**
```bash
globular domain status

# Output:
# FQDN                     DNS    CERT          INGRESS  UPDATED
# api.example.com          ✓ ok   ✓ valid (89d) ✓ ready  2m ago
# node-01.cluster.com      ✓ ok   ✓ valid (45d) ✗ n/a    5m ago
# test.example.com         ✗ fail - pending     ✗ n/a    1m ago
```

**Show specific domain:**
```bash
globular domain status --fqdn api.example.com

# Output:
# Domain: api.example.com
# Status: Ready
# Last Reconciled: 2026-02-11 14:32:15
#
# DNS:
#   Record Type: A
#   Value: 203.0.113.42
#   TTL: 600
#   Provider: my-route53
#   Status: ✓ Record exists
#
# ACME:
#   Enabled: true
#   Email: ops@example.com
#   Directory: https://acme-v02.api.letsencrypt.org/directory
#   Certificate: ✓ Valid
#   Expires: 2026-05-12 14:30:00 (90 days)
#   Path: /var/lib/globular/domains/api.example.com/fullchain.pem
#
# Ingress:
#   Enabled: true
#   Service: gateway
#   Port: 8080
#   Status: ✓ Configured
```

#### `domain remove` - Remove Domain

**Syntax:**
```bash
globular domain remove \
  --fqdn <domain> \
  [--cleanup-dns] \
  [--cleanup-certs]
```

**Flags:**
- `--fqdn` - Domain to remove (required)
- `--cleanup-dns` - Also delete DNS record (default: false)
- `--cleanup-certs` - Also delete certificate files (default: false)

**Examples:**

**Remove spec only (keep DNS and certs):**
```bash
globular domain remove --fqdn api.example.com
```

**Full cleanup:**
```bash
globular domain remove \
  --fqdn api.example.com \
  --cleanup-dns \
  --cleanup-certs
```

### `globular dns provider`

DNS provider configuration commands.

#### `dns provider add` - Configure DNS Provider

**Syntax:**
```bash
globular dns provider add \
  --name <name> \
  --type <godaddy|route53|cloudflare|manual> \
  --zone <zone> \
  [--ttl <seconds>]
```

**Required Flags:**
- `--name` - Provider name (used in domain add --provider)
- `--type` - Provider type (godaddy, route53, cloudflare, manual)
- `--zone` - DNS zone this provider manages

**Optional Flags:**
- `--ttl` - Default TTL for records (default: 600)

**Provider-Specific Credentials:**

**GoDaddy:**
```bash
export GODADDY_API_KEY="your-api-key"
export GODADDY_API_SECRET="your-api-secret"

globular dns provider add \
  --name my-godaddy \
  --type godaddy \
  --zone example.com
```

**Route53:**
```bash
# Option 1: Environment variables
export AWS_ACCESS_KEY_ID="your-key-id"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"  # Optional

# Option 2: IAM role (no env vars needed)
# Just run the command if EC2 instance has IAM role

globular dns provider add \
  --name my-route53 \
  --type route53 \
  --zone example.com
```

**Cloudflare:**
```bash
# Option 1: API token (recommended)
export CLOUDFLARE_API_TOKEN="your-api-token"

# Option 2: Global API key
export CLOUDFLARE_API_KEY="your-api-key"
export CLOUDFLARE_EMAIL="your-email@example.com"

globular dns provider add \
  --name my-cloudflare \
  --type cloudflare \
  --zone example.com
```

**Manual:**
```bash
# No credentials needed
globular dns provider add \
  --name manual \
  --type manual \
  --zone example.com
```

#### `dns provider list` - List Providers

**Syntax:**
```bash
globular dns provider list
```

**Example Output:**
```
NAME              TYPE      ZONE              TTL   UPDATED
my-godaddy        godaddy   example.com       600   2m ago
my-route53        route53   aws.example.com   300   5m ago
manual            manual    corp.example.com  600   1h ago
```

## DNS Providers

### Supported Providers

| Provider | Type | Credentials | IAM Support | Wildcard |
|----------|------|-------------|-------------|----------|
| GoDaddy | `godaddy` | API Key + Secret | No | Yes (DNS-01) |
| AWS Route53 | `route53` | IAM Role / Access Keys | Yes ✅ | Yes (DNS-01) |
| Cloudflare | `cloudflare` | API Token / API Key | No | Yes (DNS-01) |
| Manual | `manual` | None | N/A | Yes (DNS-01) |

### Provider Details

#### GoDaddy

**Features:**
- Simple REST API
- API key + secret authentication
- Credentials stored in etcd (encrypted recommended)

**Credentials:**
1. Get API credentials from https://developer.godaddy.com/keys
2. Set environment variables
3. Run `dns provider add`

**Limitations:**
- Manual credential rotation required
- No IAM role support

#### Route53

**Features:**
- AWS SDK credential chain
- IAM role support (recommended)
- Automatic credential rotation
- No credentials stored in etcd

**Credentials (Multiple Options):**

**Option 1: IAM Role (Production Recommended)**
```bash
# Attach IAM policy to EC2 instance role
# No environment variables needed
globular dns provider add --name my-route53 --type route53 --zone example.com
```

**Option 2: IAM User**
```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
globular dns provider add --name my-route53 --type route53 --zone example.com
```

**Option 3: AWS Profile**
```bash
export AWS_PROFILE="production"
globular dns provider add --name my-route53 --type route53 --zone example.com
```

**Required IAM Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "route53:ChangeResourceRecordSets",
      "route53:ListResourceRecordSets",
      "route53:ListHostedZones",
      "route53:GetChange"
    ],
    "Resource": ["arn:aws:route53:::hostedzone/*"]
  }]
}
```

#### Manual Provider

**Features:**
- Prints DNS operations for manual execution
- No API calls made
- Useful for change control processes

**Use Case:**
- Enterprise environments with strict DNS change approval
- Testing and validation
- Disaster recovery scenarios

**Behavior:**
```bash
globular domain add --provider manual ...

# Outputs:
# ╔════════════════════════════════════════════════════════╗
# ║ DNS Operation Required                                 ║
# ╠════════════════════════════════════════════════════════╣
# ║ Operation: CREATE A                                    ║
# ║ Zone:      example.com                                 ║
# ║ Name:      api                                         ║
# ║ Value:     203.0.113.42                               ║
# ║ TTL:       600                                         ║
# ╚════════════════════════════════════════════════════════╝
```

## Workflows

### Complete Setup Workflow

```
┌─────────────────────────────────────────────────────────┐
│ 1. Configure DNS Provider                              │
│    $ export GODADDY_API_KEY=...                         │
│    $ globular dns provider add --name my-godaddy ...   │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│ 2. Register Domain                                      │
│    $ globular domain add --fqdn api.example.com ...    │
│    → Creates ExternalDomainSpec in etcd                │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│ 3. Domain Reconciler (Automatic)                       │
│    ├─ Create DNS A record via provider                 │
│    ├─ Obtain ACME certificate (if enabled)             │
│    └─ Update status to "Ready"                         │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│ 4. XDS Watcher (Automatic)                             │
│    ├─ Load domain from etcd                            │
│    ├─ Create SDS secret for certificate                │
│    └─ Configure Envoy routing                          │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│ 5. Domain Accessible                                   │
│    $ curl https://api.example.com/Service/Method       │
│    ✓ DNS resolves to node IP                           │
│    ✓ TLS with Let's Encrypt certificate                │
│    ✓ Routes to gateway service                         │
└─────────────────────────────────────────────────────────┘
```

### Certificate Renewal Workflow

```
Certificate Expiry < 30 days
         │
         ▼
Domain Reconciler Detects
         │
         ▼
┌────────────────────┐
│ Renew via ACME     │
│ (DNS-01 challenge) │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Write new cert to  │
│ /var/lib/...       │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ XDS detects change │
│ Updates SDS secret │
└────────┬───────────┘
         │
         ▼
Envoy hot-reloads cert
(No downtime!)
```

## Troubleshooting

### Issue: Domain status shows "DNS fail"

**Check:**
```bash
globular domain status --fqdn api.example.com
```

**Possible Causes:**
1. **DNS provider credentials invalid**
   - Verify environment variables are set
   - Check provider configuration: `globular dns provider list`

2. **Zone mismatch**
   - FQDN must be subdomain of zone
   - Example: `api.example.com` requires zone `example.com`

3. **Provider API errors**
   - Check logs: `journalctl -u globular-reconciler -f`
   - Verify API quotas not exceeded

**Fix:**
```bash
# Update provider credentials
export GODADDY_API_KEY="new-key"
globular dns provider add --name my-godaddy --type godaddy --zone example.com

# Trigger reconciliation
globular domain add --fqdn api.example.com ... # Re-run to update
```

### Issue: ACME certificate not obtained

**Check:**
```bash
ls -la /var/lib/globular/domains/api.example.com/
```

**Possible Causes:**
1. **DNS record not propagated**
   - Wait 5-10 minutes for DNS propagation
   - Check: `dig api.example.com`

2. **ACME email not provided**
   - Required when `--enable-acme`
   - Add: `--acme-email ops@example.com`

3. **Rate limit hit**
   - Let's Encrypt has rate limits
   - Use staging: `--acme-directory staging`

4. **DNS-01 challenge failed**
   - Check TXT record creation works
   - Verify provider permissions

**Fix:**
```bash
# Use staging for testing
globular domain add \
  --fqdn test.example.com \
  --zone example.com \
  --provider my-godaddy \
  --enable-acme \
  --acme-email ops@example.com \
  --acme-directory staging

# Check reconciler logs
journalctl -u globular-reconciler -f
```

### Issue: "Provider not found"

**Error:**
```
Error: DNS provider "my-godaddy" not found
```

**Fix:**
```bash
# List configured providers
globular dns provider list

# Add missing provider
globular dns provider add \
  --name my-godaddy \
  --type godaddy \
  --zone example.com
```

### Issue: Certificate files missing

**Check:**
```bash
ls /var/lib/globular/domains/api.example.com/
```

**Expected Files:**
```
fullchain.pem   # Certificate + chain
privkey.pem     # Private key
chain.pem       # ACME issuer
account.key     # ACME account key
```

**Possible Causes:**
1. **Reconciler not running**
   ```bash
   systemctl status globular-reconciler
   systemctl start globular-reconciler
   ```

2. **Permission issues**
   ```bash
   sudo chown -R globular:globular /var/lib/globular/domains
   ```

3. **ACME not enabled**
   ```bash
   # Check domain spec
   globular domain status --fqdn api.example.com
   # Ensure "ACME: Enabled: true"
   ```

### Debug Commands

**View domain spec in etcd:**
```bash
# View spec (user intent)
ETCDCTL_API=3 etcdctl get /globular/domains/v1/api.example.com

# View status (reconciler state - stored separately)
ETCDCTL_API=3 etcdctl get /globular/domains/v1/api.example.com/status
```

**Check reconciler logs:**
```bash
journalctl -u globular-reconciler -f --since "10 minutes ago"
```

**Verify DNS resolution:**
```bash
dig api.example.com
nslookup api.example.com
```

**Test certificate:**
```bash
openssl x509 -in /var/lib/globular/domains/api.example.com/fullchain.pem -text -noout
```

## Advanced Topics

### Custom ACME Directory

```bash
# Use private ACME server
globular domain add \
  --fqdn internal.corp.com \
  --zone corp.com \
  --provider my-route53 \
  --enable-acme \
  --acme-email pki@corp.com \
  --acme-directory https://acme.corp.com/directory
```

### Multiple Zones with Same Provider

```bash
# Configure provider for each zone
globular dns provider add --name godaddy-com --type godaddy --zone example.com
globular dns provider add --name godaddy-net --type godaddy --zone example.net

# Register domains in different zones
globular domain add --fqdn api.example.com --provider godaddy-com ...
globular domain add --fqdn api.example.net --provider godaddy-net ...
```

### Wildcard Certificates

**Note:** Wildcard certificates not yet supported in domain reconciler. Use node PKI manager for wildcard certs.

**Planned Support:**
```bash
# Future feature
globular domain add \
  --fqdn "*.example.com" \
  --zone example.com \
  --provider my-route53 \
  --enable-acme \
  --acme-email ops@example.com
```

## See Also

- [DNS Provider Credential Guide](/home/dave/Documents/tmp/CREDENTIAL_HANDLING_GUIDE.md)
- [Domain Reconciler Implementation](/home/dave/Documents/tmp/pr3-implementation-summary.md)
- [XDS Integration](/home/dave/Documents/tmp/pr3c-implementation-summary.md)
