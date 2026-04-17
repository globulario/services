# Certificate Lifecycle (Internal PKI)

This page covers how **internal** TLS certificates are managed in a Globular cluster: the cluster CA, service certificates for mTLS, Ed25519 keystores, provisioning, rotation, and troubleshooting.

For **external** certificates (Let's Encrypt, public domains, ACME), see [DNS and PKI](operators/dns-and-pki.md).

## Why Certificate Management Matters

Every gRPC connection in Globular uses TLS. Certificates establish trust between components:
- The Node Agent trusts the Controller (and vice versa)
- Services trust each other for inter-service calls
- The Gateway presents certificates to external clients
- Clients verify they're talking to the real cluster (not a man-in-the-middle)

If certificates expire, become invalid, or are not properly distributed, services cannot communicate. Certificate management is therefore a critical operational concern.

## Certificate Architecture

### Trust Hierarchy

```
Cluster CA (self-signed root)
    ├── CA cert: /var/lib/globular/pki/ca.crt
    ├── CA key:  /var/lib/globular/pki/ca.key
    │
    ├── Node Service Certificate (one per node, used for both server and client mTLS)
    │   ├── Subject: CN=<hostname>, O=globular.internal
    │   ├── SANs: localhost, *.localhost, <hostname>, *.globular.internal,
    │   │         globular.internal, <node-ip>
    │   ├── Cert: /var/lib/globular/pki/issued/services/service.crt
    │   └── Key:  /var/lib/globular/pki/issued/services/service.key
    │
    ├── xDS Server Certificate
    │   ├── Cert: /var/lib/globular/pki/xds/current/tls.crt
    │   └── Key:  /var/lib/globular/pki/xds/current/tls.key
    │
    └── Envoy xDS Client Certificate
        ├── Cert: /var/lib/globular/pki/envoy-xds-client/current/tls.crt
        └── Key:  /var/lib/globular/pki/envoy-xds-client/current/tls.key
```

### File Locations

| File | Path | Permissions | Purpose |
|------|------|-------------|---------|
| Service certificate | `/var/lib/globular/pki/issued/services/service.crt` | 0644 | Server and client identity for mTLS |
| Service private key | `/var/lib/globular/pki/issued/services/service.key` | 0600 | Signs TLS handshake |
| CA certificate | `/var/lib/globular/pki/ca.crt` | 0644 | Trust anchor for all internal certificates |
| CA private key | `/var/lib/globular/pki/ca.key` | 0600 | Signs node certificates (bootstrap node only) |
| xDS server cert | `/var/lib/globular/pki/xds/current/tls.crt` | 0644 | xDS control plane identity |
| xDS server key | `/var/lib/globular/pki/xds/current/tls.key` | 0600 | xDS TLS handshake |
| Envoy xDS client cert | `/var/lib/globular/pki/envoy-xds-client/current/tls.crt` | 0644 | Envoy authenticates to xDS |
| Envoy xDS client key | `/var/lib/globular/pki/envoy-xds-client/current/tls.key` | 0600 | Envoy xDS TLS handshake |
| Ed25519 signing keys | `/var/lib/globular/keys/<id>_private` | 0600 | JWT token signing |
| Ed25519 public keys | `/var/lib/globular/keys/<id>_public` | 0644 | JWT token verification |

## Initial Provisioning

### During Bootstrap

When the first node bootstraps, certificates are generated as part of the initialization:

1. **CA generation**: A self-signed CA key pair is created. The CA is the root of trust for the entire cluster.
2. **CA distribution**: The CA certificate is stored in etcd and made available via the Gateway's HTTP endpoint
3. **Server certificate**: A server certificate is generated for the bootstrap node, signed by the CA
4. **Client certificate**: A client certificate is generated for mTLS authentication
5. **Certificate storage**: All certificates are written to `/var/lib/globular/pki/`

### During Node Join

When a new node joins the cluster:

1. **CA fetch**: The Node Agent fetches the cluster CA from the Gateway:
   ```
   GET https://<gateway>/get_ca_certificate
   ```
   The protocol is auto-detected based on the gateway port:
   - Ports ending in 43 (443, 8443, 9443) → HTTPS
   - Other ports → HTTP fallback

   For self-signed CAs, the first fetch uses TLS with certificate verification disabled (bootstrap trust). Once the CA is installed locally, subsequent fetches verify against it.

2. **CSR generation**: The node generates a key pair and creates a Certificate Signing Request (CSR)
3. **CSR submission**: The CSR is submitted to the Gateway's signing endpoint:
   ```
   GET https://<gateway>/sign_ca_certificate?csr=<base64-encoded-csr>
   ```
4. **Certificate receipt**: The Gateway signs the CSR with the cluster CA and returns the certificate
5. **Certificate installation**: The certificate is written atomically to the node's credential directory

### Atomic Certificate Writes

Certificate installation uses a safe write pattern to prevent corruption:

1. Write the new certificate to a temporary file (same directory)
2. Create a backup of the existing certificate (if any)
3. Atomically rename the temporary file to the target path
4. If the rename fails, restore from backup
5. A `.cert.lock` file prevents concurrent writes (10-minute stale detection)

This ensures that a crash during certificate installation doesn't leave the node with a corrupt or missing certificate.

## Certificate Rotation

### When to Rotate

Certificates should be rotated:
- Before they expire (the platform monitors expiry and warns in advance)
- After a security incident (potential key compromise)
- When changing the cluster domain
- When the CA certificate is about to expire

### How Rotation Works

Certificate rotation in Globular follows a pattern that maintains continuous service availability:

1. **Generate new key pair**: The node creates a new Ed25519 key pair
2. **Create CSR**: A new CSR is generated with the updated SANs
3. **Sign**: The CSR is submitted to the Gateway's signing endpoint
4. **Install**: The new certificate is installed atomically (backup → write temp → rename)
5. **Reload**: Services detect the new certificate and reload their TLS configuration

The CA's SPKI (Subject Public Key Info) fingerprint is used to detect CA changes. If the fingerprint changes during rotation, all node certificates are re-provisioned to maintain trust chain validity.

### Ed25519 Key Rotation

The Ed25519 keystore supports key rotation with backward compatibility:

**Key ID (KID)**: Each key pair has a KID derived from the SHA256 hash of the public key (first 16 characters, base64url-encoded).

**Rotation process**:
1. Generate a new Ed25519 key pair
2. Write it with a KID-qualified filename: `<node_id>_<kid>_private`
3. The old key remains at: `<node_id>_private` (legacy format)
4. New tokens are signed with the new key (KID in JWT header)
5. Old tokens are still valid (old public key still available for verification)
6. After all old tokens expire, the old key can be removed

**Lookup order**: When verifying a token, the keystore checks:
1. Rotated key files (KID-qualified names)
2. Legacy key files (node ID only)
3. Legacy config directory (backward compatibility)

## Monitoring Certificates

### Certificate Status

Query certificate status on any node:

```bash
globular node certificate-status --node <node>:11000
```

The `GetCertificateStatus` RPC returns:

```
Server Certificate:
  Subject: node-1.mycluster.local
  Issuer: Globular CA
  SANs: node-1.mycluster.local, 192.168.1.10, localhost
  Not Before: 2025-01-01T00:00:00Z
  Not After: 2026-01-01T00:00:00Z
  Days Until Expiry: 264
  SHA256 Fingerprint: ab:cd:ef:12:...
  Chain Valid: true

CA Certificate:
  Subject: Globular CA
  Not Before: 2025-01-01T00:00:00Z
  Not After: 2030-01-01T00:00:00Z
  Days Until Expiry: 1729
  SHA256 Fingerprint: 12:34:56:78:...
```

### Expiry Monitoring

The Cluster Doctor checks certificate expiry as part of its invariant checks:

- **WARN**: Certificate expires within 30 days
- **ERROR**: Certificate expires within 7 days
- **CRITICAL**: Certificate has expired

Prometheus metrics can also track certificate expiry:
```promql
globular_certificate_expiry_days{node="node-1", cert_type="server"}
```

### Cluster-Wide Certificate Audit

Check all nodes' certificate status:

```bash
# Using the MCP tools
# For each node:
globular node certificate-status --node <node>:11000

# Or via the doctor report which includes certificate findings
globular doctor report --fresh
```

## Troubleshooting

### Certificate Expired

**Symptoms**: Services fail with "tls: certificate has expired" or "x509: certificate has expired or is not yet valid"

**Fix**:
```bash
# Check current certificate status
globular node certificate-status --node <node>:11000

# Re-provision the certificate
# The node agent can request a new certificate from the gateway:
# 1. Restart the node agent (it re-provisions on startup)
sudo systemctl restart globular-node-agent

# 2. Or manually trigger certificate refresh
# (depends on the node agent's certificate management implementation)
```

### CA Mismatch

**Symptoms**: Services fail with "x509: certificate signed by unknown authority"

**Cause**: The node's CA certificate doesn't match the cluster's CA. This can happen if:
- A node joined before a CA rotation
- The CA file was manually modified
- The node was restored from a backup with an old CA

**Fix**:
```bash
# Fetch the current CA from the gateway
# The node agent does this automatically on restart:
sudo systemctl restart globular-node-agent

# Or manually fetch and install:
curl -k https://<gateway>/get_ca_certificate > /var/lib/globular/pki/ca.crt
# Then restart services to pick up the new CA
```

### TLS Handshake Failure

**Symptoms**: Services fail with "tls: handshake failure" or "tls: bad certificate"

**Diagnosis**:
```bash
# Check that SANs include the correct hostname/IP
openssl x509 -in /var/lib/globular/pki/issued/services/service.crt -text -noout | grep -A5 "Subject Alternative Name"

# Check that the certificate chain is complete
openssl verify -CAfile /var/lib/globular/pki/ca.crt /var/lib/globular/pki/issued/services/service.crt

# Check that the key matches the certificate
openssl x509 -noout -modulus -in /var/lib/globular/pki/issued/services/service.crt | openssl md5
openssl rsa -noout -modulus -in /var/lib/globular/pki/issued/services/service.key | openssl md5
# (These should match)
```

### Lock File Stuck

**Symptoms**: Certificate provisioning fails with "lock file held"

**Cause**: A previous certificate operation crashed without releasing the lock.

**Fix**:
```bash
# Check the lock file age
ls -la /var/lib/globular/pki/.cert.lock

# If it's older than 10 minutes, it's stale and will be auto-detected
# If you need to force it:
rm /var/lib/globular/pki/.cert.lock
```

## Practical Scenarios

### Scenario 1: Pre-Expiry Certificate Rotation

Certificates are approaching expiry (30 days out):

```bash
# Doctor reports the warning
globular doctor report
# WARN: server certificate on node-2 expires in 28 days

# Rotate the certificate
# Restart the node agent to trigger re-provisioning:
ssh node-2 sudo systemctl restart globular-node-agent

# Verify
globular node certificate-status --node node-2:11000
# Days Until Expiry: 365 (new certificate)
```

### Scenario 2: CA Rotation

The cluster CA is approaching expiry:

```bash
# This is a high-impact operation — plan carefully
# 1. Generate a new CA on the gateway node
# 2. Distribute the new CA to all nodes
# 3. Re-sign all node certificates with the new CA
# 4. Restart all services to pick up new certificates

# After CA rotation:
# All nodes should show the new CA fingerprint
for node in node-1 node-2 node-3; do
  globular node certificate-status --node $node:11000
done
```

### Scenario 3: Adding a New SAN

A node's IP address changed and the certificate SAN needs updating:

```bash
# The existing certificate has SANs for the old IP
# Generate a new CSR with the new IP and submit for signing
# Restart the node agent to trigger re-provisioning:
sudo systemctl restart globular-node-agent

# The agent detects the IP change, generates a new CSR
# with the current IPs, and gets it signed

# Verify the new SANs
globular node certificate-status --node <node>:11000
```

## What's Next

- [DNS and PKI](operators/dns-and-pki.md): External certificates (Let's Encrypt), DNS zones, ACME provisioning, split-horizon DNS
- [Writing a Microservice](../developers/writing-a-microservice.md): Build services that use Globular's TLS infrastructure
- [RBAC Integration](../developers/rbac-integration.md): Add authorization to your services
