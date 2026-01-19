# Service Spec Configuration Path Fix

## Issue Summary

The service package specs were creating individual directories for each service in `/var/lib/globular/`, which caused:

1. Empty directories like `/var/lib/globular/authentication`, `/var/lib/globular/blog`, etc.
2. Inconsistent configuration paths across services
3. Some services using `/etc/globular` instead of `/var/lib/globular/services`
4. Confusion about where service configurations are stored

## Root Cause

The `specgen.sh` script was generating specs that:
- Created individual working directories: `{{.StateDir}}/servicename`
- Used those directories as `WorkingDirectory` in systemd units
- Didn't follow the standard pattern of storing all configs in `/var/lib/globular/services`

## Solution

### 1. Updated specgen.sh (services/golang/globularcli/tools/specgen/)

**Changed:**
- Removed creation of individual service directories
- Set `WorkingDirectory={{.StateDir}}/services` for all services
- Ensured `GLOBULAR_SERVICES_DIR={{.StateDir}}/services` environment variable

**Before:**
```yaml
dirs:
  - path: "{{.StateDir}}"
  - path: "{{.StateDir}}/services"
  - path: "{{.StateDir}}/authentication"  # ❌ Creates individual dir

[Service]
WorkingDirectory={{.StateDir}}/authentication  # ❌ Wrong path
```

**After:**
```yaml
dirs:
  - path: "{{.StateDir}}"
  - path: "{{.StateDir}}/services"
  # No individual service directory ✓

[Service]
WorkingDirectory={{.StateDir}}/services  # ✓ Correct path
```

### 2. Updated Infrastructure Package Specs

Fixed specs in `/home/dave/Documents/github.com/globulario/packages/specs/`:

**Services that NO LONGER create individual directories:**
- `gateway` - Uses `/var/lib/globular/services`
- `envoy` - Uses `/var/lib/globular/services` (config in /run/globular/envoy)
- `node-agent` - Uses `/var/lib/globular/services`

**Services that STILL keep their directories (legitimate data storage):**
- `cluster-controller` - Needs `/var/lib/globular/cluster-controller/` for state.json
- `etcd` - Needs `/var/lib/globular/etcd/` for data and config
- `minio` - Needs `/var/lib/globular/minio/data/` for object storage
- `xds` - Needs `/var/lib/globular/xds/` for xds.yaml config

### 3. Rebuilt All Packages

Rebuilt packages with corrected specs:

**Infrastructure packages** (from `/packages/`):
- service.envoy_1.35.3_linux_amd64.tgz ✓
- service.gateway_0.0.1_linux_amd64.tgz ✓
- service.node-agent_0.0.1_linux_amd64.tgz ✓

**Application services** (from `/services/golang/globularcli/generated/`):
- All 21 service packages rebuilt with corrected specs ✓

## Directory Structure

### Correct Structure (After Fix)

```
/var/lib/globular/
├── services/                    # All service configs (*.json)
├── cluster-controller/          # Infrastructure: state storage
├── etcd/                        # Infrastructure: data storage
├── minio/                       # Infrastructure: object storage
│   └── data/
└── xds/                         # Infrastructure: config storage
```

### Incorrect Structure (Before Fix)

```
/var/lib/globular/
├── services/
├── authentication/              # ❌ Empty
├── blog/                        # ❌ Empty
├── catalog/                     # ❌ Empty
├── conversation/                # ❌ Empty
├── gateway/                     # ❌ Empty
├── node-agent/                  # ❌ Empty
├── envoy/                       # ❌ Empty
├── ... (many more empty dirs)
```

## Service Configuration Storage

### Application Services (Store config in /var/lib/globular/services/)

All application services store their configuration as JSON files in `/var/lib/globular/services/`:
- Format: `<service-uuid>.json`
- Example: `/var/lib/globular/services/12345678-1234-1234-1234-123456789abc.json`

Services:
- authentication, blog, catalog, conversation, discovery
- dns, echo, event, file, log, media, monitoring
- persistence, rbac, repository, resource, search
- sql, storage, title, torrent

### Infrastructure Services (May have own directories)

Infrastructure services that need persistent data storage keep their own directories:

1. **cluster-controller** `/var/lib/globular/cluster-controller/`
   - `state.json` - Cluster state

2. **etcd** `/var/lib/globular/etcd/`
   - `etcd.yaml` - Configuration
   - Data directory for key-value storage

3. **minio** `/var/lib/globular/minio/`
   - `data/` - Object storage

4. **xds** `/var/lib/globular/xds/`
   - `xds.yaml` - Configuration

## Migration / Cleanup

### For Existing Installations

If you have an existing installation with empty service directories, use the cleanup script:

```bash
cd /home/dave/Documents/github.com/globulario/globular-installer/scripts
sudo ./cleanup-service-dirs.sh
```

This script:
- Removes empty service directories
- Preserves infrastructure directories (cluster-controller, etcd, minio, xds)
- Preserves any non-empty directories (with warning)
- Shows summary of what was cleaned

### Manual Cleanup

If preferred, manually remove empty directories:

```bash
cd /var/lib/globular
sudo rmdir authentication blog catalog conversation discovery dns echo event \
  file log media monitoring persistence rbac repository resource search sql \
  storage title torrent gateway node-agent envoy 2>/dev/null || true
```

## Testing

After applying the fix and reinstalling services:

1. **Check directory structure:**
   ```bash
   ls -la /var/lib/globular/
   ```
   Should only show: `services`, `cluster-controller`, `etcd`, `minio`, `xds`

2. **Check service configs:**
   ```bash
   ls -la /var/lib/globular/services/
   ```
   Should show `*.json` config files

3. **Verify services are running:**
   ```bash
   systemctl status globular-*.service
   ```

4. **Check working directories:**
   ```bash
   systemctl show -p WorkingDirectory globular-authentication.service
   ```
   Should show: `WorkingDirectory=/var/lib/globular/services`

## Impact

✓ **No breaking changes** - Services read configs from `/var/lib/globular/services/` as before
✓ **Cleaner filesystem** - No unnecessary empty directories
✓ **Consistent paths** - All services use the same standard locations
✓ **Future-proof** - New services will follow the correct pattern

## Files Modified

1. `/services/golang/globularcli/tools/specgen/specgen.sh` - Spec generator
2. `/packages/specs/gateway_service.yaml` - Gateway spec
3. `/packages/specs/envoy_service.yaml` - Envoy spec
4. `/packages/specs/node_agent_service.yaml` - Node agent spec
5. All generated service packages in `/services/golang/globularcli/generated/packages/`
6. Infrastructure packages in `/packages/out/`

## Date

Fixed: 2026-01-18
