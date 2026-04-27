# Website Publishing

This page explains how Globular serves static websites, how to publish a site, and how the full pipeline from file upload to HTTPS delivery works.

---

## How It Works

Globular serves static websites through a layered pipeline:

```
Browser
  └─► Envoy (port 443)
         └─► SNI filter chain (matches domain, selects TLS cert)
                └─► Gateway service (port 8443)
                       └─► MinIO (object storage) or local disk
```

Every piece is driven by configuration in etcd — no static config files, no manual Envoy restarts.

### Object Storage Layout

When MinIO is configured, all website files are stored as objects. The object key prefix depends on whether the request comes from an **internal cluster subdomain** or an **external domain**.

| Host | Object key prefix | Example |
|------|-------------------|---------|
| `*.globular.internal` | `webroot/` | `webroot/index.html` |
| `app.example.com` (subdomain of cluster domain) | `webroot/` | `webroot/app/index.html` |
| `globular.io` (external domain) | `globular.io/webroot/` | `globular.io/webroot/index.html` |
| `docs.globular.io` (external subdomain) | `globular.io/webroot/` | `globular.io/webroot/docs/index.html` |

This domain-aware routing means a single MinIO bucket can host multiple websites with full isolation.

### User Files vs Webroot

The gateway distinguishes two upload targets:

- **`/users/<uid>/...`** — personal file storage, goes under `files/users/<uid>/` in MinIO
- **`/` or any non-users path** — treated as webroot content, goes under `<domain>/webroot/` in MinIO

### TLS Certificate Chain

For external domains, Globular uses Let's Encrypt certificates through the following chain:

1. **Domain registration** — operator registers the domain via the CLI (`globular domain add`)
2. **ACME challenge** — the cluster performs an HTTP-01 or DNS-01 challenge automatically
3. **Certificate issuance** — cert stored at `/var/lib/globular/domains/<domain>/fullchain.pem`
4. **Domain status** — once issued, etcd key `/globular/domains/v1/<domain>/status` is set to `phase: "Ready"`
5. **xDS snapshot** — the xDS service watches this key and, when `Ready`, pushes a new snapshot to Envoy containing an SNI filter chain for `[domain, *.domain]` backed by an SDS secret `ext-cert/<domain>`
6. **SDS secret** — Envoy fetches the cert from the xDS SDS endpoint; the key maps to the Let's Encrypt cert on disk
7. **Live** — HTTPS requests for the domain now use the LE cert, not the internal cluster cert

Internal domains always use the cluster mTLS cert (`internal-cert`). External domains get their own LE cert via SDS.

---

## Prerequisites

1. A Globular cluster with MinIO on at least 3 nodes (founding quorum)
2. A public domain pointed at your VIP (A record → your VIP's public IP)
3. Port 443 forwarded from your router to the VIP (`10.0.0.100` in the reference cluster)
4. An account with write access to the gateway

---

## Step 1 — Register Your Domain

```bash
# Register the domain with ACME enabled
globular domain add --domain globular.io --acme

# Check that the certificate was issued and domain is Ready
globular domain status globular.io
```

Wait for `phase: "Ready"` and `cert_expiry` to appear. This means the Let's Encrypt cert has been issued and the xDS snapshot has been updated.

To verify Envoy is serving the correct cert:

```bash
openssl s_client -connect 10.0.0.100:443 -servername globular.io </dev/null 2>&1 \
  | grep -E "issuer|subject"
# Expected:
# subject=CN = globular.io
# issuer=C = US, O = Let's Encrypt, CN = E8
```

---

## Step 2 — Upload Your Website

### Via the Gateway Upload API

The gateway exposes a multipart upload endpoint at `POST /file-upload`. Any file uploaded to path `/` (or any non-`/users` path) is written to the webroot for the request's `Host` domain.

```bash
# Upload index.html to the webroot of globular.io
curl -X POST https://globular.io/file-upload \
  -H "token: <your-token>" \
  -F "dir=/" \
  -F "multiplefiles=@index.html"
```

The response is JSON with the stored paths:

```json
{ "paths": ["/index.html"] }
```

### Uploading Multiple Files

```bash
# Upload all files from a dist/ directory
for f in dist/*; do
  filename=$(basename "$f")
  curl -X POST https://globular.io/file-upload \
    -H "token: <your-token>" \
    -F "dir=/" \
    -F "multiplefiles=@${f};filename=${filename}"
done
```

### From a Build Pipeline

For CI/CD integration, use the same endpoint with an API token:

```bash
TOKEN=$(globular auth token --service my-publisher)

upload_dir() {
  local src_dir="$1"
  local target_dir="$2"
  find "$src_dir" -type f | while read -r file; do
    rel="${file#$src_dir/}"
    dir="$target_dir/$(dirname "$rel")"
    curl -s -X POST https://globular.io/file-upload \
      -H "token: $TOKEN" \
      -F "dir=$dir" \
      -F "multiplefiles=@$file;filename=$(basename $file)"
  done
}

upload_dir ./dist /
```

---

## Step 3 — Verify the Site

```bash
# Check the object is in MinIO
globular storage ls globular.io/webroot/

# Make an HTTP request
curl -I https://globular.io/
# Expected: HTTP/2 200
```

---

## How the Gateway Serves Files

When a GET request arrives at the gateway for a file path:

1. Gateway checks if the path is under `/users/` — if so, it's a personal file
2. Otherwise, it resolves the MinIO key: `<domain>/webroot/<path>` (or `webroot/<path>` for internal hosts)
3. If MinIO is configured and the object exists, it streams the object back with the appropriate `Content-Type`
4. If MinIO is not configured (local mode), it reads from `<data-root>/files/<path>`
5. Directory paths (e.g. `/about/`) are resolved to `/about/index.html`

### Content-Type Detection

Content-Type is derived from the file extension. Common mappings:

| Extension | Content-Type |
|-----------|--------------|
| `.html` | `text/html` |
| `.css` | `text/css` |
| `.js` | `application/javascript` |
| `.json` | `application/json` |
| `.png`, `.jpg`, `.gif` | `image/*` |
| `.svg` | `image/svg+xml` |
| `.woff2` | `font/woff2` |

---

## Multi-Domain Hosting

A single cluster can host multiple websites. Each external domain gets:
- Its own Let's Encrypt cert (via `ext-cert/<domain>` SDS secret)
- Its own MinIO prefix (`<domain>/webroot/`)
- Its own SNI filter chain in Envoy

To add a second domain:

```bash
globular domain add --domain myapp.io --acme

# Upload to that domain's webroot
curl -X POST https://myapp.io/file-upload \
  -H "token: <your-token>" \
  -F "dir=/" \
  -F "multiplefiles=@index.html"
```

Objects land at `myapp.io/webroot/index.html` in MinIO, completely separate from `globular.io/webroot/`.

---

## Permissions

By default, all file uploads require a valid auth token. The gateway checks write access via RBAC before accepting any upload.

**Exception — home directory shortcut**: A user can always upload to `/users/<their-uid>/` without explicit RBAC write permission. The gateway uses the token's identity directly, bypassing the RBAC check. This prevents breakage when identity lookups fail after domain or account changes.

To grant a service account write access to the webroot:

```bash
globular rbac grant --subject publisher@myapp.io --resource /webroot --action write
```

---

## Keepalived and VIP Stability

The VIP (`10.0.0.100` in the reference cluster) is managed by keepalived using VRRP. Both gateway nodes (`globule-ryzen` at priority 120, `globule-nuc` at priority 110) participate. The VIP floats to the highest-priority healthy node.

Keepalived health check (`/usr/lib/globular/bin/check-ingress.sh`) verifies that `globular-gateway.service` is active. If the gateway process dies, keepalived detects this within 4 seconds (2 × 2s interval) and moves the VIP to the backup node. Site availability resumes in under 5 seconds during a gateway node failure.

The keepalived spec lives in etcd at `/globular/ingress/v1/spec` and is reconciled by the node agent on each heartbeat cycle. Changing the spec propagates to all participant nodes automatically.

---

## Troubleshooting

### Site returns the wrong TLS certificate

**Symptom**: `openssl s_client` shows the internal cluster cert (`O=globular.internal`) instead of the Let's Encrypt cert.

**Cause**: The xDS service hasn't yet pushed a snapshot containing the SNI filter chain for the domain. This happens when:
- The domain status hasn't reached `phase: "Ready"` in etcd
- The xDS service was restarted before the ACME cert was issued
- The domain was registered after the last xDS snapshot

**Fix**:
```bash
# Check domain status
globular domain status globular.io

# If Ready but cert still wrong, restart xDS to force a new snapshot
sudo systemctl restart globular-xds
# Wait ~10s, then restart Envoy to pick it up
sudo systemctl restart globular-envoy
```

### 404 on all pages

**Symptom**: All requests return 404 or "file not found".

**Cause**: Files were uploaded with the wrong path, or MinIO key prefix mismatch.

**Fix**:
```bash
# List what's actually in MinIO
globular storage ls globular.io/webroot/
# or for internal host
globular storage ls webroot/

# Re-upload with correct path
curl -X POST https://globular.io/file-upload -F "dir=/" -F "multiplefiles=@index.html"
```

### VIP keeps moving between nodes

**Symptom**: Site goes down briefly every few minutes; keepalived logs show MASTER/BACKUP transitions.

**Cause**: The health check script is failing on the current MASTER node. This is usually because:
- `globular-gateway.service` is not running
- The health script `/usr/lib/globular/bin/check-ingress.sh` is missing

**Fix**:
```bash
# On the MASTER node
sudo systemctl status globular-gateway.service
sudo cat /usr/lib/globular/bin/check-ingress.sh

# If health script is missing, it will be written on the next node-agent heartbeat
# Force a reconcile by restarting the node agent
sudo systemctl restart globular-node-agent
```

### Upload returns 401 Unauthorized

**Symptom**: File upload returns `401`.

**Cause**: Token is missing or expired, or the account lacks write permission on the target path.

**Fix**:
```bash
# Refresh your token
globular auth login

# Check RBAC permissions
globular rbac check --subject me@domain.io --resource /webroot --action write
```

---

## Reference: etcd Keys

| Key | Purpose |
|-----|---------|
| `/globular/domains/v1/<domain>` | Domain spec (ACME config, ingress settings) |
| `/globular/domains/v1/<domain>/status` | Domain status (`phase`, `cert_expiry`) |
| `/globular/ingress/v1/spec` | Keepalived VIP spec (participants, priorities, VIP) |
| `/globular/ingress/v1/status/<node_id>` | Per-node VRRP state and VIP presence |

---

## Reference: MinIO Object Key Mapping

| Upload path | Host | Object key |
|-------------|------|------------|
| `/index.html` | `app.example.com` (internal) | `webroot/index.html` |
| `/about/team.html` | `app.example.com` (internal) | `webroot/about/team.html` |
| `/index.html` | `globular.io` (external) | `globular.io/webroot/index.html` |
| `/css/main.css` | `docs.globular.io` (external) | `globular.io/webroot/css/main.css` |
| `/users/alice/photo.jpg` | any | `files/users/alice/photo.jpg` |
