# DNS and PKI — External and Internal

Globular operates two independent certificate systems and a dual-purpose DNS service. This page explains how internal (mTLS) and external (Let's Encrypt) certificates work together, how the DNS service handles both cluster-internal and public-facing domains, and how the domain reconciler automates ACME certificate provisioning.

## Two Certificate Worlds

Globular maintains a strict separation between internal and external certificates:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Certificate Architecture                     │
│                                                                 │
│  INTERNAL (cluster mTLS)          EXTERNAL (public HTTPS)       │
│  ─────────────────────            ────────────────────────      │
│  Issuer: Globular Root CA         Issuer: Let's Encrypt         │
│  Scope:  *.globular.internal      Scope:  *.globular.io         │
│  Used by: gRPC services,          Used by: Envoy gateway        │
│           node agents,                     for browser/API      │
│           controller               clients from internet        │
│  Managed: Internal CA (auto)      Managed: ACME DNS-01 (auto)   │
│  Lifetime: 1 year                 Lifetime: 90 days             │
│  Renewal: On node restart         Renewal: 30 days before exp   │
│                                                                 │
│  File locations:                  File locations:               │
│  /var/lib/globular/pki/           /var/lib/globular/domains/    │
│    issued/services/service.crt      {domain}/fullchain.pem      │
│    issued/services/service.key      {domain}/privkey.pem        │
│    ca.crt                           {domain}/chain.pem          │
│                                     {domain}/account.json       │
└─────────────────────────────────────────────────────────────────┘
```

### Why Two Systems

**Internal certificates** secure service-to-service communication within the cluster. They are signed by the cluster's own CA and contain internal hostnames and private IPs in their SANs. Browsers and external clients do not trust these certificates — and they shouldn't, because the internal CA is private to the cluster.

**External certificates** secure traffic from the internet to the Envoy gateway. They are signed by Let's Encrypt (a publicly trusted CA) and contain the cluster's public domain names. Browsers trust these certificates automatically.

Mixing these would create security problems:
- Using Let's Encrypt for internal mTLS would require exposing internal hostnames to a public CA
- Using the internal CA for external traffic would cause browser certificate errors
- Certificate rotation schedules differ (1 year internal vs 90 days ACME)

## Internal PKI

### Globular Root CA

Every cluster has a self-signed Root CA created during bootstrap. This CA signs all internal certificates:

```
Globular Root CA (self-signed, 10-year lifetime)
├── Subject: CN=Globular Root CA, O=globular.internal
├── Key: /var/lib/globular/pki/ca.key
├── Cert: /var/lib/globular/pki/ca.crt
│
├── Service Certificate (per node)
│   ├── Subject: CN=<hostname>, O=globular.internal
│   ├── SANs: localhost, *.localhost, <hostname>, *.globular.internal,
│   │         globular.internal, <node-ip>
│   ├── Cert: /var/lib/globular/pki/issued/services/service.crt
│   └── Key:  /var/lib/globular/pki/issued/services/service.key
│
├── xDS Server Certificate
│   ├── Subject: CN=xds-server
│   ├── Cert: /var/lib/globular/pki/xds/current/tls.crt
│   └── Key:  /var/lib/globular/pki/xds/current/tls.key
│
└── Envoy xDS Client Certificate
    ├── Subject: CN=envoy-xds-client
    ├── Cert: /var/lib/globular/pki/envoy-xds-client/current/tls.crt
    └── Key:  /var/lib/globular/pki/envoy-xds-client/current/tls.key
```

### Internal Certificate Usage

| Certificate | Used By | Presented To | Purpose |
|------------|---------|-------------|---------|
| Service cert | Every gRPC service | Other gRPC services, Node Agent | mTLS server identity |
| Service cert (as client) | Service-to-service calls | Target service | mTLS client identity |
| xDS server cert | xDS control plane | Envoy proxy | SDS/ADS transport |
| xDS client cert | Envoy proxy | xDS control plane | Client auth for config streaming |
| CA cert | All components | All components | Trust anchor for chain validation |

### Internal Certificate SANs

Each node's service certificate includes SANs for:
- `localhost` and `*.localhost` — loopback access
- The node's hostname (e.g., `globule-ryzen`)
- `*.globular.internal` and `globular.internal` — cluster domain wildcard
- The node's IP address (e.g., `10.0.0.63`)

**Important**: The IP SAN is bound to the node's current IP at certificate generation time. If a node's IP changes, the certificate must be re-provisioned. This is why changing a node's primary IP is a disruptive operation — it invalidates the TLS certificate, breaking all gRPC connections.

### Internal Certificate Lifecycle

| Event | Action | Automatic |
|-------|--------|-----------|
| Bootstrap | Generate CA + first node cert | Yes |
| Node join | Fetch CA from gateway, generate CSR, get signed | Yes |
| Node restart | Check cert validity, re-provision if needed | Yes |
| IP change | Re-generate CSR with new IP, get signed | Requires restart |
| CA expiry (30 days) | Doctor WARN finding | Detection only |
| Cert expiry (30 days) | Doctor WARN finding | Detection only |

## External PKI (Let's Encrypt)

### How ACME Works in Globular

Globular uses the **ACME protocol** with **DNS-01 challenges** to obtain certificates from Let's Encrypt. The process is fully automated through the **domain reconciler** running inside the Cluster Controller.

```
Operator declares domain          Reconciler provisions cert
─────────────────────────         ──────────────────────────
globular domain add               1. Load/create ACME account
  --fqdn globular.io              2. Set DNS-01 challenge provider
  --enable-acme                   3. Request cert from Let's Encrypt
  --use-wildcard-cert             4. LE sends challenge token
  --acme-email admin@...          5. Reconciler creates TXT record:
                                     _acme-challenge.globular.io
                                  6. LE verifies TXT record
                                  7. LE issues certificate
                                  8. Reconciler stages cert (atomic write)
                                  9. xDS detects new cert → pushes to Envoy
                                  10. Envoy serves new cert (no restart)
```

### Why DNS-01 (Not HTTP-01)

Let's Encrypt supports two challenge types:

| Challenge | How It Works | Limitations |
|-----------|-------------|-------------|
| **HTTP-01** | LE requests `http://<domain>/.well-known/acme-challenge/<token>` | Cannot issue wildcard certs. Requires port 80 open. |
| **DNS-01** | LE queries `_acme-challenge.<domain>` TXT record | Works for wildcards. Requires DNS API access. |

Globular uses **DNS-01** because:
1. It supports wildcard certificates (`*.globular.io`)
2. Globular's own DNS service is authoritative — no third-party DNS API needed
3. It works even if port 80 is blocked

### Wildcard Certificates

A wildcard certificate covers the apex domain and all single-level subdomains:

```
Certificate SANs: DNS:*.globular.io, DNS:globular.io

Covers:
  ✓ globular.io
  ✓ www.globular.io
  ✓ app.globular.io
  ✓ api.globular.io
  ✓ anything.globular.io

Does NOT cover:
  ✗ sub.sub.globular.io (second level)
  ✗ globular.com (different domain)
```

One wildcard cert serves all subdomains — no need to request a new certificate for each service or application.

### External Certificate Files

| File | Path | Purpose |
|------|------|---------|
| Full chain | `/var/lib/globular/domains/{domain}/fullchain.pem` | Leaf cert + issuer chain |
| Private key | `/var/lib/globular/domains/{domain}/privkey.pem` | ECDSA private key |
| Chain only | `/var/lib/globular/domains/{domain}/chain.pem` | Issuer certificates |
| ACME account | `/var/lib/globular/domains/{domain}/account.json` | ACME registration + key |

The xDS server reads certificates from a symlinked path:
```
/var/lib/globular/config/tls/acme/{domain}/ → /var/lib/globular/domains/{domain}/
```

### External Certificate Lifecycle

| Event | Action | Automatic |
|-------|--------|-----------|
| `globular domain add --enable-acme` | Reconciler obtains cert via DNS-01 | Yes |
| 30 days before expiry | Reconciler auto-renews (same ACME flow) | Yes |
| `.renew-requested` marker file | Forced renewal even if cert is valid | Manual trigger |
| Cert renewed | xDS detects file change → pushes to Envoy | Yes |
| Domain removed | Cert files remain (not auto-deleted) | Manual cleanup |

### Let's Encrypt Rate Limits

Let's Encrypt enforces rate limits:
- **50 certificates per registered domain per week**
- **5 duplicate certificates per week** (same exact SANs)
- **5 failed validations per hostname per hour**

The domain reconciler handles this by:
- Only requesting certs when needed (not on every reconciliation cycle)
- Checking cert validity before requesting (skip if > 30 days remaining)
- Using staging directory for test runs (`--acme-directory staging`)

## DNS Service

### Dual Role

Globular's DNS service serves two purposes:

1. **Internal DNS**: Resolves `*.globular.internal` for service discovery within the cluster
2. **Authoritative DNS**: Serves public DNS queries for registered domains (e.g., `globular.io`) if the domain's NS records point to the Globular DNS server

```
External query: www.globular.io?
  → Public resolvers (8.8.8.8, 1.1.1.1)
  → NS lookup: dns.globular.io
  → Globular DNS service (port 53)
  → Returns: 96.20.133.54 (public IP)

Internal query: www.globular.io?
  → /etc/hosts: 10.0.0.100 (VIP)  ← hairpin NAT workaround
  → (or Globular DNS: 96.20.133.54 if not in /etc/hosts)
```

### Managed Zones

The DNS service manages one or more zones. Each DNS instance must have the zone registered in its managed domains list:

```bash
# View managed zones
globular dns domains list
# globular.internal.
# globular.io.

# Add a zone (needed after DNS restart if zone was lost)
globular dns domains set globular.internal. globular.io.
```

**Note**: Managed domain lists are stored in ScyllaDB and persist across restarts. All DNS instances in the cluster share the same store. If zones appear missing after restart, verify the domains were set via an authenticated gRPC call (the CLI may fail with "cluster_id required" — use grpcurl directly if needed).

### DNS Records for External Access

For a public domain to work, the DNS must have:

```bash
# Apex domain
globular.io.              A    96.20.133.54   (public IP)

# Wildcard for all subdomains
*.globular.io.            A    96.20.133.54

# Node-specific hostnames
globule-ryzen.globular.io. A   96.20.133.54
globule-nuc.globular.io.   A   96.20.133.54
globule-dell.globular.io.  A   96.20.133.54
```

All external records point to the **public IP**, which the router's DMZ forwards to the keepalived VIP (10.0.0.100), which routes to the active Envoy gateway.

### Split-Horizon DNS (Hairpin NAT Workaround)

Consumer routers (like Videotron Helix) often cannot handle **hairpin NAT** — accessing your own public IP from inside the network. When a machine resolves `www.globular.io` to `96.20.133.54` and tries to connect, the router drops the packet because the source and destination are on the same LAN.

**Solution**: Override DNS resolution on cluster nodes via `/etc/hosts`:

```
# /etc/hosts on each cluster node
10.0.0.100 globular.io www.globular.io globule-ryzen.globular.io globule-nuc.globular.io globule-dell.globular.io
```

This ensures:
- **From inside the network**: `www.globular.io` → `10.0.0.100` (VIP, direct)
- **From the internet**: `www.globular.io` → `96.20.133.54` (public IP → DMZ → VIP)

Both paths reach the same Envoy gateway with the same Let's Encrypt certificate.

### DNS for ACME Challenges

During ACME certificate provisioning, the domain reconciler creates temporary TXT records:

```
_acme-challenge.globular.io.  TXT  "dGVzdC1rZXktYXV0aC0..."   TTL=60
```

The record is created via the local DNS provider (`dnsprovider/local/`), verified for propagation, and cleaned up after Let's Encrypt validates it. The entire flow takes 30-60 seconds.

## Domain Reconciler

### What It Does

The domain reconciler runs inside the Cluster Controller as a periodic loop (default: every 5 minutes). For each registered external domain, it ensures:

1. **DNS records** are correct (A record pointing to the public IP)
2. **ACME certificates** are valid and not expiring within 30 days
3. **Status** is written to etcd for monitoring

### Configuration

Domains are declared via the CLI and stored in etcd:

```bash
# Add a domain with ACME
globular domain add \
  --fqdn globular.io \
  --zone globular.io \
  --provider local-globular-io \
  --target-ip 96.20.133.54 \
  --enable-acme \
  --acme-email admin@globular.io \
  --use-wildcard-cert

# Check status
globular domain status
# FQDN         PHASE  DNS  CERT  INGRESS  UPDATED
# globular.io  Ready  ✓    ✓     ✓        2m ago
```

### DNS Providers

The reconciler uses a DNS provider to manage records and ACME challenges. Supported providers:

| Provider | Type | Credentials | Use Case |
|----------|------|-------------|----------|
| `local` | Globular's own DNS | None (gRPC to local service) | When Globular DNS is authoritative |
| `godaddy` | GoDaddy API | `GODADDY_API_KEY`, `GODADDY_API_SECRET` | GoDaddy-registered domains |
| `cloudflare` | Cloudflare API | `CLOUDFLARE_API_TOKEN` | Cloudflare-managed DNS |
| `route53` | AWS Route 53 | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | AWS-hosted zones |
| `manual` | Manual intervention | None | Testing, manual DNS setups |

```bash
# List configured providers
globular domain provider list
# NAME               TYPE   ZONE         CREDENTIALS
# local-globular-io  local  globular.io  none

# Add a GoDaddy provider
export GODADDY_API_KEY="your-key"
export GODADDY_API_SECRET="your-secret"
globular domain provider add \
  --name my-godaddy \
  --type godaddy \
  --zone example.com
```

### ACME Account

The reconciler creates and persists an ACME account per domain:

```json
// /var/lib/globular/domains/globular.io/account.json
{
  "email": "admin@globular.io",
  "registration": { ... },
  "key": "-----BEGIN EC PRIVATE KEY-----\n..."
}
```

The account is created once and reused for all renewals. If the account email changes, the reconciler detects the mismatch and reports an error. To fix: delete `account.json` and let the reconciler create a new account with the correct email.

### Certificate Staging and Swap

When the reconciler obtains a new certificate, it uses an atomic staging process:

1. Write new cert to `.staging/` subdirectory
2. Validate the staged cert (parse, check domain match, check expiry)
3. Atomically swap: move staged files to the live directory
4. Remove the `.renew-requested` marker (if present)
5. Clean up staging directory

This ensures that a crash during renewal doesn't leave partial or invalid certificates in the live path.

### Forced Renewal

To force renewal (e.g., after revoking a certificate or changing domain coverage):

```bash
# Create the marker file
sudo touch /var/lib/globular/domains/globular.io/.renew-requested

# The reconciler picks it up on the next cycle (within 5 minutes)
# Or restart the controller to trigger immediate reconciliation
```

## How Envoy Serves Certificates

### SDS (Secret Discovery Service)

Envoy does not read certificate files directly. Instead, the xDS server pushes certificates to Envoy via **SDS** (Secret Discovery Service):

```
Certificate files on disk
        │
        ▼
xDS server (file watcher)
        │ detects changes
        ▼
Generates SDS snapshot
        │
        ▼ gRPC push
Envoy receives new cert
        │
        ▼
Envoy serves new cert (no restart)
```

### SNI-Based Certificate Selection

Envoy uses **SNI (Server Name Indication)** to select which certificate to serve:

```
Client sends TLS ClientHello with SNI = "www.globular.io"
    │
    ▼
Envoy checks filter chains:
  1. FC[0]: SNI match ["globular.io", "*.globular.io"]
     → Serve Let's Encrypt cert (fullchain.pem + privkey.pem)
  2. FC[1]: Default (no SNI match)
     → Serve internal cert (service.crt + service.key)
```

This means:
- `https://www.globular.io` → Let's Encrypt cert (trusted by browsers)
- `https://globular.internal` → Internal CA cert (trusted by cluster services)
- Direct IP access without SNI → Internal CA cert (default)

### Certificate Hot-Reload

When the domain reconciler writes a new certificate:

1. xDS server's filesystem watcher detects the file change
2. xDS rebuilds its snapshot with the new certificate content
3. xDS pushes the updated snapshot to Envoy via gRPC
4. Envoy applies the new certificate to its filter chain
5. New TLS connections use the new certificate; existing connections are unaffected

No Envoy restart is needed. The transition is seamless — clients never see an interruption.

## Complete Network and Certificate Flow

```
Browser requests https://www.globular.io
    │
    ▼
DNS resolution:
  External: 8.8.8.8 → dns.globular.io → 96.20.133.54 (public IP)
  Internal: /etc/hosts → 10.0.0.100 (VIP, hairpin workaround)
    │
    ▼
ISP Router (DMZ → 10.0.0.100)
    │
    ▼
keepalived VIP 10.0.0.100 → active gateway node
    │
    ▼
Envoy (port 443)
  SNI: www.globular.io → matches *.globular.io filter chain
  Serves: Let's Encrypt wildcard cert
  TLS terminated
    │
    ▼
Internal routing (HTTP/2)
  Host: www.globular.io → routes to web application
  Or: gRPC-Web → routes to gRPC service
    │
    ▼
Internal gRPC (mTLS with internal CA cert)
  Service-to-service calls use internal PKI
```

## CLI Reference

### Domain Management

```bash
# Add domain with ACME wildcard
globular domain add \
  --fqdn globular.io \
  --zone globular.io \
  --provider local-globular-io \
  --target-ip 96.20.133.54 \
  --enable-acme \
  --acme-email admin@globular.io \
  --use-wildcard-cert

# Check domain status
globular domain status
globular domain status --fqdn globular.io --output json

# Remove domain
globular domain remove --fqdn globular.io

# Force certificate renewal
sudo touch /var/lib/globular/domains/globular.io/.renew-requested
```

### DNS Provider Management

```bash
# List providers
globular domain provider list

# Add local provider (Globular DNS is authoritative)
globular domain provider add --name my-local --type local --zone example.com

# Add external provider
export CLOUDFLARE_API_TOKEN="your-token"
globular domain provider add --name my-cf --type cloudflare --zone example.com
```

### DNS Record Management

```bash
# View managed zones
globular dns domains list

# Set A record
globular dns a set globular.io 96.20.133.54 --ttl 3600

# Set wildcard
globular dns a set "*.globular.io" 96.20.133.54 --ttl 3600

# Set TXT record (manual ACME testing)
globular dns txt set "_acme-challenge.globular.io" "test-value" --ttl 60

# Query records
globular dns inspect --domain globular.io
globular dns lookup --name www.globular.io
```

### Certificate Inspection

```bash
# Internal certificates
globular node certificate-status --node <node>:11000

# External certificates (via gateway admin)
curl -sk https://10.0.0.100/admin/certificates | python3 -m json.tool

# Direct OpenSSL check
openssl x509 -in /var/lib/globular/domains/globular.io/fullchain.pem -noout -text
echo | openssl s_client -connect 10.0.0.100:443 -servername www.globular.io 2>/dev/null | openssl x509 -noout -subject -issuer -ext subjectAltName
```

## Troubleshooting

### ACME Certificate Not Issued

```bash
# Check domain status for error message
globular domain status --fqdn globular.io --output json

# Common errors:
# "account email mismatch" → delete account.json and retry
# "domain not managed by this DNS" → re-add zone to DNS managed domains
# "failed to create DNS-01 challenge" → DNS provider auth issue
```

**DNS zone not managed**: After DNS service restart, the `globular.io` zone may be lost from the in-memory domain list. Re-add it:
```bash
globular dns domains set globular.internal. globular.io.
```

**Account email mismatch**: The ACME account was created with a different email:
```bash
sudo rm /var/lib/globular/domains/globular.io/account.json
# Reconciler will create a new account on next cycle
```

### Browser Shows Internal Certificate

If `https://www.globular.io` shows the internal CA cert (`CN=globule-ryzen, O=globular.internal`) instead of the Let's Encrypt cert:

1. **Check the symlink**: xDS reads from `/var/lib/globular/config/tls/acme/{domain}/`
   ```bash
   ls -la /var/lib/globular/config/tls/acme/globular.io/
   # Should point to /var/lib/globular/domains/globular.io/
   ```

2. **Restart xDS** to reload cert config:
   ```bash
   sudo systemctl restart globular-xds
   ```

3. **Verify** with openssl:
   ```bash
   echo | openssl s_client -connect 10.0.0.100:443 -servername www.globular.io 2>/dev/null | openssl x509 -noout -issuer
   # Should show: issuer=C = US, O = Let's Encrypt, CN = E7
   ```

4. **Clear browser cache**: Chrome aggressively caches TLS failures. Close all Chrome windows and reopen, or clear at `chrome://net-internals/#dns` and `chrome://net-internals/#sockets`.

### Hairpin NAT: Can't Access Public Domain from Inside Network

Your router doesn't support accessing your own public IP from the LAN. Fix with `/etc/hosts` on each cluster node:

```
# /etc/hosts
10.0.0.100 globular.io www.globular.io globule-ryzen.globular.io globule-nuc.globular.io globule-dell.globular.io
```

Verify:
```bash
# From cluster node — should connect to VIP, not public IP
curl -sk https://www.globular.io -o /dev/null -w "Connected to %{remote_ip}\n"
# Connected to 10.0.0.100
```

### DNS Zone Lost After Restart

The DNS service stores records in ScyllaDB (persistent) but the managed domain list is in memory. After restart, zones may need re-registration:

```bash
# Check current zones
globular dns domains list

# Re-add if missing
globular dns domains set globular.internal. globular.io.
```

This should be done on all DNS instances (each node running the DNS service).

## What's Next

- [Certificate Lifecycle](certificate-lifecycle.md) — Internal PKI details: provisioning, rotation, Ed25519 keystores
- [Keepalived and Ingress](keepalived-and-ingress.md) — VIP failover, DMZ setup, external traffic routing
- [Network and Routing](network-and-routing.md) — Envoy gateway, xDS, service discovery, gRPC-Web
