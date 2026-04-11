# Day-0 Checklist (Globular)

Purpose: shortest path from bare node to a running Globular cluster with correct DNS, TLS, storage, and MCP/xDS/gateway wired up. Scripts live in `~/Documents/github.com/globulario/globular-installer/scripts`.

## Order of operations
1) **DNS on port 53**  
   - Script: `bootstrap-dns.sh` (frees port 53, configures resolver).  
   - Verify: `dig globular.internal @127.0.0.1` (or cluster DNS IP) returns A records.

2) **TLS prep**  
   - Script: `setup-tls.sh` (or `verify-cert-setup.sh` to validate).  
   - Ensure `/var/lib/globular/config/tls/{fullchain.pem,privkey.pem,ca.pem}` readable by services; `fix-tls-permissions.sh` if needed.

3) **MinIO**  
   - Scripts: `setup-minio.sh`, `ensure-minio-buckets.sh`, `setup-minio-contract.sh`, `provision-minio-token.sh`.  
   - Verify: `mc ls minio/` lists buckets; `mc ls minio/globular-config/workflows/` accessible.

4) **Scylla TLS (for AI memory)**  
   - Scripts: `scylladb/setup-scylla-tls.sh`, `fix-client-cert-ownership.sh`.  
   - Verify: `cqlsh <scylla-host> 9042 --ssl -e "DESC KEYSPACE ai_memory;"`.

5) **Bootstrap artifacts**  
   - Script: `ensure-bootstrap-artifacts.sh` (packages, specs, workflow defs).  
   - Optional: `setup-config.sh` to stage configs.

6) **Install Day-0 packages** (as root)  
   - Script: `install-day0.sh`  
   - Env vars: `PKG_DIR` (defaults to `internal/assets/packages`), `FORCE_REINSTALL=1` if needed.  
   - This drives `globular-installer` to lay down services, writes Day-0 trace log at `/var/lib/globular/day0-install.jsonl`.

7) **Validate cluster**  
   - Script: `validate-cluster-health.sh`  
   - Also run MCP doctor: `cluster_get_doctor_report` via MCP to confirm services healthy.

8) **Publish workflows to MinIO**  
   - See `workflow-publish.md` (mc cp `golang/workflow/definitions/*.yaml` → `minio/globular-config/workflows/`).

9) **Bring up xDS + Envoy + gateway**  
   - Ensure xDS (tls) running; Envoy bootstrapped with ADS/SDS to xDS; gateway registered in etcd.  
   - Test MCP through Envoy: `curl -vk https://mcp.<domain>/mcp` (with correct Host/SNI).

## Quick verifications
- DNS: `dig globule-ryzen.globular.internal`.
- Ports: MCP `cluster_get_health` for live service ports.
- etcd: `etcdctl get /globular/system/config`.
- MinIO: `mc ls minio/globular-config/workflows/`.
- Scylla: `cqlsh ... SELECT id,title FROM ai_memory.memories LIMIT 1;`.
- MCP: `curl -s -D - http://127.0.0.1:10250/mcp -d '{"'"'"'jsonrpc'"'"'":"'"'"'2.0'"'"',"'"'"'id'"'"'":1,"'"'"'method'"'"'":"'"'"'initialize'"'"',"'"'"'params'"'"'":{"'"'"'protocolVersion'"'"'":"'"'"'2025-03-26'"'"',"'"'"'capabilities'"'"'":{"'"'"'tools'"'"':{}}}}'`.

## Notes
- Ports are dynamic; rely on etcd/MCP, not static numbers.
- Run scripts as root where noted (e.g., install-day0.sh).
- Keep TLS/mTLS enabled for xDS/SDS; `GLOBULAR_XDS_INSECURE=1` is dev-only.
