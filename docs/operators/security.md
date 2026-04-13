# Security

Globular implements defense-in-depth security across every layer of the platform. All inter-service communication is encrypted with TLS. Every gRPC request is authenticated (JWT or mTLS) and authorized (RBAC). Every authorization decision is audited. The bootstrap process is time-bounded and loopback-restricted. Certificates are managed automatically with rotation support.

This page covers the complete security architecture: the PKI system, authentication, RBAC, the interceptor chain, bootstrap security, and certificate lifecycle.

## Public Key Infrastructure (PKI)

### Ed25519 Keystore

Globular uses Ed25519 (EdDSA) for all cryptographic signing. Each node generates its own Ed25519 key pair during initialization. Keys are stored locally:

| Path | Purpose | Permissions |
|------|---------|-------------|
| `/var/lib/globular/keys/<id>_private` | Signing key (PEM, PKCS#8) | 0600 |
| `/var/lib/globular/keys/<id>_public` | Verification key (PEM, SubjectPublicKeyInfo) | 0644 |

**Key IDs (KID)**: Each key pair has a Key ID derived from the SHA256 hash of the public key (first 16 characters, base64url-encoded). The KID is embedded in JWT headers, allowing token validators to look up the correct public key.

**Key rotation**: The keystore supports multiple keys per node. During rotation, the new key is written alongside the old one with a KID-qualified filename (`<id>_<kid>_private`). Old keys remain available for validating tokens signed before rotation. The lookup order is: rotated key (by KID) → legacy key (by node ID) → legacy config directory.

### TLS Certificates

All gRPC communication uses TLS. Certificate management is integrated into the platform:

**Certificate storage**:
```
/var/lib/globular/pki/
├── ca.crt                              # Cluster CA certificate
├── ca.key                              # CA signing key (bootstrap node)
├── ca.pem                              # CA in PEM format
├── issued/services/
│   ├── service.crt                     # Node service certificate (server + client)
│   └── service.key                     # Node service private key
├── xds/current/
│   ├── tls.crt                         # xDS server certificate
│   └── tls.key                         # xDS server key
└── envoy-xds-client/current/
    ├── tls.crt                         # Envoy xDS client certificate
    └── tls.key                         # Envoy xDS client key
```

**CA certificate distribution**: Each node fetches the cluster CA from the Gateway via HTTP/HTTPS:
```
GET /get_ca_certificate
```

The protocol is auto-detected based on port conventions:
- Ports 443, 8443, 9443, 10443 → HTTPS preferred
- Ports 80, 8080, 9080 → HTTP preferred
- Port ending in `43` → HTTPS preferred

For self-signed CAs during bootstrap, the client first tries HTTPS with system trust, then retries with certificate verification disabled (bootstrap only), then falls back to HTTP.

**Certificate signing**: Nodes generate a CSR and submit it to the Gateway's signing endpoint:
```
GET /sign_ca_certificate?csr=<base64-encoded-csr>
```

The gateway signs the CSR with the cluster CA and returns the certificate. This is used for initial certificate provisioning and for rotation.

**Atomic certificate writes**: Certificate installation uses a safe write pattern:
1. Write new certificate to a temporary file
2. Back up the existing certificate
3. Atomically rename the temporary file to the target path
4. If the rename fails, restore from backup

A `.cert.lock` file (with 10-minute stale detection) prevents concurrent certificate operations on the same node.

**CA drift detection**: The platform computes SPKI (Subject Public Key Info) fingerprints for CA certificates. If the fingerprint changes (indicating a CA rotation), all service certificates are re-provisioned.

### Certificate Status

Query certificate status on any node:
```bash
globular node certificate-status --node node-1:11000
```

The Node Agent's `GetCertificateStatus` RPC returns:
- Server certificate: subject, issuer, SANs, not_before, not_after, days_until_expiry, SHA256 fingerprint
- CA certificate: same fields
- Chain validity: whether the certificate chain is complete and valid

## Authentication

Globular supports two authentication methods: JWT tokens and mTLS certificates.

### JWT Authentication

**Token structure**: JWTs use Ed25519 (EdDSA) signatures with these claims:

| Claim | Purpose |
|-------|---------|
| `PrincipalID` | Opaque, stable, domain-independent identity (e.g., `usr_7f9a3b2c`). This is the canonical identity used for authorization. |
| `ClusterID` | Prevents cross-cluster token replay. Derived from the cluster domain. |
| `Scopes` | OAuth-style scopes (e.g., `["read:files", "write:config"]`) |
| `ID` | Legacy username (display only, NOT used for authorization) |
| `Email` | Contact information only |
| Standard claims | `exp`, `iat`, `nbf`, `jti`, `sub`, `iss`, `aud` |

**Token generation**: The Authentication service (port 10101) handles authentication:

```bash
# Authenticate with username and password
# Returns: JWT token
globular auth login --username admin --password <password>
```

Internally, the `Authenticate` RPC:
1. Validates the password against the bcrypt hash stored in the resource service
2. Generates a JWT signed with the node's Ed25519 private key
3. Sets the issuer to the node's MAC address (for key lookup during validation)
4. Returns the token to the client

**Token validation**: When any gRPC request arrives with a token (via `Authorization: Bearer <token>` header or custom `token` metadata):
1. Extract the KID from the JWT header
2. Look up the issuer's Ed25519 public key (via the KID or issuer claim)
3. Verify the signature
4. Check expiration (with 60-second leeway for clock skew)
5. Validate the ClusterID against the local cluster domain
6. Extract the PrincipalID as the authenticated identity

**Token refresh**: Tokens can be refreshed up to 7 days after expiry. The Authentication service issues a new token with a fresh expiration if the original token is within the refresh window.

**Service tokens**: For internal service-to-service calls, services generate short-lived tokens (5-minute TTL) signed with their own Ed25519 key. The audience field is set to the peer's MAC address to prevent replay.

### Password Policy

The Authentication service enforces a password policy:
- Minimum 12 characters
- At least 3 of 4 character classes: lowercase, uppercase, digit, special character
- No spaces or control characters
- Passwords are hashed with bcrypt before storage

### mTLS Authentication

When a client presents a TLS client certificate, the platform extracts identity from it:
1. Read the Common Name (CN) from the peer certificate
2. Strip any `@domain` suffix for backward compatibility
3. Read Organization[0] as a ClusterID hint (fallback to default domain)
4. Create an AuthContext with `AuthMethod: "mtls"`

mTLS is used primarily for node-to-node communication (Node Agent ↔ Controller, Controller ↔ Workflow Service). Human users typically use JWT authentication.

### AuthContext

Every authenticated request produces an `AuthContext` that flows through the handler chain:

```go
type AuthContext struct {
    ClusterID     string  // Cluster identifier
    Subject       string  // Canonical identity (PrincipalID or CN)
    PrincipalType string  // "user", "application", "node", "anonymous"
    AuthMethod    string  // "jwt", "mtls", "apikey", "anonymous"
    IsBootstrap   bool    // Day-0 bootstrap mode
    IsLoopback    bool    // Request from 127.0.0.1/::1
    GRPCMethod    string  // Full RPC method name
}
```

The identity extraction order:
1. Try JWT token from metadata → extract PrincipalID
2. If no JWT, try mTLS certificate → extract CN
3. If neither, mark as anonymous

## Role-Based Access Control (RBAC)

### Permission Model

Globular's RBAC system controls access to resources through a hierarchical permission model:

**Resources**: Identified by hierarchical paths (e.g., `/catalog/connections/{id}/items/{item_id}`)

**Permissions**: Four levels — `read`, `write`, `delete`, `admin`

**Subjects**: Six types — account, application, group, organization, node_identity, role

### Built-in Roles

Globular defines built-in roles for common operational patterns:

| Role | Permissions | Use Case |
|------|------------|----------|
| `globular-admin` | `/*` (full access) | Cluster administrators |
| `globular-publisher` | Publish artifacts | CI/CD pipelines |
| `globular-operator` | Manage releases, domains | Day-2 operators |
| `globular-controller-sa` | Read/apply state (no publish) | Cluster Controller service account |
| `globular-node-agent-sa` | Report status, execute plans | Node Agent service account |
| `globular-node-executor` | Per-node scoped operations | Node-specific actions |

### Role Bindings

Role bindings associate subjects with roles:

```bash
# Assign the operator role to a user
globular rbac bind --subject usr_abc123 --role globular-operator

# List role bindings for a subject
globular rbac bindings --subject usr_abc123
```

Role bindings are stored in etcd at `ROLE_BINDINGS/<subject>` as JSON arrays of role names.

### Permission Annotations

Every gRPC RPC is annotated in the proto file with its required permissions:

```protobuf
rpc DeleteBackup(DeleteBackupRequest) returns (DeleteBackupResponse) {
    option (globular.auth.authz) = {
        action: "backup.delete"
        permission: "delete"
        resource_template: "/backup/{backup_id}"
        default_role_hint: "admin"
    };
}
```

The `authzgen` tool extracts these annotations during code generation and produces:
- `permissions.generated.json`: Maps each RPC to its required action and resource template
- `cluster-roles.generated.json`: Maps built-in roles to their permitted actions

### Permission Matching

When checking permissions, the system supports wildcards:
- `/*` — grants access to all resources (admin role)
- `/pkg.Service/*` — grants access to all methods of a service
- `file.*` — grants all file-related actions (`file.read`, `file.write`, etc.)

### Ownership

Resources have an ownership chain. The owner of a resource automatically has all permissions on it:
1. Check if the subject is in the resource's owners list
2. If not, walk up the path hierarchy (parent resources)
3. Support group and organization membership inheritance
4. Owner check uses both exact match and bare ID match (strips `@domain`)

### Deny Overrides Allow

If a subject has both an `allow` and a `deny` for the same resource, the deny wins. This allows fine-grained exceptions:
- Role `globular-operator` grants `write` on `/services/*`
- Explicit deny on `/services/authentication` prevents modifying the auth service
- Result: operator can manage all services except authentication

## The Interceptor Chain

Every gRPC request passes through a multi-layer interceptor chain before reaching the handler. This chain implements the security model:

### Step 1: Call Depth Check

The interceptor reads the `x-call-depth` metadata header. If the depth exceeds 10, the request is rejected. This prevents infinite loops where Service A calls Service B, which calls Service A, etc.

Each outgoing service-to-service call increments the depth counter. If a chain of calls reaches 10 hops, something is wrong and the circuit is broken.

### Step 2: Authentication

The interceptor creates an `AuthContext` by extracting identity from the request:
1. Check for JWT token in metadata → validate signature, extract PrincipalID
2. Check for mTLS peer certificate → extract CN
3. Fall back to anonymous

### Step 3: Bootstrap Check

If the cluster is in bootstrap mode (Day-0 initialization), the `BootstrapGate` applies four levels of security:

1. **Explicit enablement**: The file `/var/lib/globular/bootstrap.enabled` must exist
2. **Time-bounded**: Bootstrap expires 30 minutes after the flag was created
3. **Loopback-only**: The request must originate from 127.0.0.1 or ::1
4. **Method allowlist**: Only essential Day-0 methods are permitted (health checks, RBAC setup, authentication, DNS zone creation, repository upload, event publish/subscribe)

If all four checks pass, the request proceeds without RBAC enforcement. This allows the cluster to initialize itself before the RBAC system is fully operational.

### Step 4: Cluster ID Validation

Post-bootstrap, the interceptor validates the ClusterID from the AuthContext:
- If the ClusterID is empty → reject (unless exempt: mTLS, JWT with matching cluster, loopback, or allowlisted method)
- If the ClusterID doesn't match the local cluster → reject

This prevents cross-cluster attacks where a token from cluster A is used against cluster B.

### Step 5: Allowlist Check

Some methods are explicitly allowlisted for unauthenticated access:
- Health checks (`grpc.health.v1.Health/Check`)
- gRPC reflection
- Authentication endpoints (must be callable before having a token)

If the method is on the allowlist, it proceeds without RBAC.

### Step 6: RBAC Enforcement

For all other methods:
1. Look up the RBAC resource mapping for this method (from generated permission files)
2. Extract the resource path by substituting request fields into the resource template
3. Call the RBAC service to check if the subject has the required permission on the resource
4. If the RBAC service is unreachable, fall back to local cluster-roles.json (prevents bootstrap deadlock)
5. Allow or deny based on the result

### Step 7: Audit Logging

Every authorization decision is logged:

```json
{
    "timestamp": "2025-04-12T10:30:00Z",
    "subject": "usr_abc123",
    "principal_type": "user",
    "auth_method": "jwt",
    "grpc_method": "/backup.BackupManager/DeleteBackup",
    "resource_path": "/backup/bk-001",
    "permission": "delete",
    "allowed": true,
    "reason": "rbac_granted",
    "decision_latency_ms": 2,
    "remote_addr": "192.168.1.50:43210"
}
```

**Allowed decisions**: Logged at DEBUG level (to reduce volume in normal operation).
**Denied decisions**: Logged at WARN level and **never sampled** — every denial is recorded. Raw tokens are never included in audit logs.

## Node Identity and Scoping

### Node Principals

Each node has a principal identity in the format `node_<uuid>`. Node principals are scoped — they can only operate on their own node:

- `node_abc123` can call `SetInstalledPackage` for `node_id=abc123`
- `node_abc123` **cannot** call `SetInstalledPackage` for `node_id=def456`
- Admin principals are exempt from node scoping

This prevents a compromised node from modifying the state of other nodes.

### Service Account Deprecation

The legacy `sa` (service account) principal was previously used for node operations. Globular is transitioning to per-node identity:

- `DeprecateSANodeAuth=true`: Warn when `sa` is used for node operations
- `RequireNodeIdentity=true`: Reject `sa` for node operations (strict mode)

## Bootstrap Security

Day-0 cluster initialization is a security-sensitive operation. The cluster must be able to initialize services (including RBAC) before the security system is fully operational. The bootstrap gate provides a controlled window:

### Enabling Bootstrap

```bash
# On the node being bootstrapped:
# This creates /var/lib/globular/bootstrap.enabled with a 30-minute TTL
globular cluster bootstrap --node localhost:11000 --domain mycluster.local
```

The bootstrap flag file contains:
```json
{
    "enabled_at_unix": 1712937600,
    "expires_at_unix": 1712939400,
    "nonce": "random-uuid",
    "created_by": "globularcli",
    "version": 1
}
```

### Bootstrap Window

During the 30-minute window, requests from localhost can access essential methods without RBAC:
- Health checks (all services)
- RBAC role binding setup
- Authentication initialization
- DNS zone creation
- Repository artifact upload
- Event publish/subscribe

After 30 minutes, the flag expires automatically. If bootstrap hasn't completed, the operator must re-enable it.

### Post-Bootstrap

Once the cluster is operational (RBAC configured, roles bound, services running), bootstrap mode is no longer needed. All requests must authenticate and pass RBAC checks. The bootstrap flag file can be deleted:

```bash
rm /var/lib/globular/bootstrap.enabled
```

## Cross-Cluster Security

Globular prevents cross-cluster attacks through ClusterID validation:

1. **Token binding**: Every JWT includes a `ClusterID` claim derived from the cluster domain
2. **Validation**: The interceptor compares the token's ClusterID against the local cluster
3. **Rejection**: Tokens from a different cluster are rejected, even if the signature is valid

This is critical in environments where multiple Globular clusters share network connectivity. A token from `cluster-a.local` cannot be used to access services on `cluster-b.local`.

## Practical Scenarios

### Scenario 1: Service-to-Service Authentication

The monitoring service needs to query the authentication service for token validation:

1. Monitoring service generates a service token (5-minute TTL, audience=auth-service MAC)
2. Monitoring calls `ValidateToken` on the authentication service with the service token in metadata
3. Auth service interceptor validates the service token (correct signature, correct audience, not expired)
4. Auth service interceptor checks RBAC — monitoring's role includes `auth.validate_token.read`
5. Request proceeds

### Scenario 2: Operator Deploying a Service

An operator wants to deploy a new service version:

1. Operator authenticates: `globular auth login --username admin`
2. Receives JWT with `PrincipalID=usr_admin`, `ClusterID=mycluster.local`
3. Runs: `globular services desired set postgresql 0.0.4`
4. CLI sends `UpsertDesiredService` to the controller with the JWT token
5. Controller interceptor validates JWT → subject=`usr_admin`
6. Controller interceptor checks RBAC: `usr_admin` has `globular-operator` role → `services.desired.write` → allowed
7. Controller processes the request

### Scenario 3: Compromised Node Attempt

If an attacker compromises node-3 and tries to modify node-1's packages:

1. Attacker uses node-3's credentials (`node_node3`)
2. Calls `SetInstalledPackage` on the controller with `node_id=node1`
3. Interceptor authenticates the request — valid JWT for `node_node3`
4. Node scoping check: `node_node3` can only modify `node_id=node3`
5. Request rejected — `node_node3` cannot operate on `node_id=node1`
6. Audit log records: denied, reason=`node_scope_violation`

## What's Next

- [Installation](installation.md): Day-0 bootstrap walkthrough
- [Adding Nodes](adding-nodes.md): Day-1 cluster expansion
