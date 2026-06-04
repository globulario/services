# Incident closure — scylla-manager TLS trust (2026-06-03)

**Status**: closed.
**Final verification**: cluster `overall_status = PASS` with one
unrelated `INFO` housekeeping item remaining
(`artifact.layout_drift_local`).

---

## Doctor finding

| Field | Value |
|---|---|
| invariant_id | `scylla_manager.cluster_registered` |
| severity | WARN |
| entity_ref | `globular-scylla-manager.service` |
| summary | "scylla-manager HTTPS endpoint reachable but TLS trust failure blocks safe verification (refusing to fall back to HTTP; cluster registration state cannot be confirmed)" |
| endpoint | `https://10.0.0.63:5443` |
| ca_path | `/var/lib/globular/pki/ca.crt` |
| tls_error | `x509: certificate signed by unknown authority` |

## Root cause

scylla-manager served a self-signed `O=Scylla` cert on its HTTPS
endpoint because `/var/lib/globular/scylla-manager/scylla-manager.yaml`
lacked `tls_cert_file` and `tls_key_file`. With no TLS material
configured, scylla-manager auto-generated its own self-signed cert
under an `O=Scylla` root every startup.

The cluster-doctor rule
`golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go`
GETs `https://<host>:5443/api/v1/clusters` with `CAFile =
/var/lib/globular/pki/ca.crt` and does not fall back to HTTP. The
self-signed `O=Scylla` cert always failed verify, so the finding
persisted across every install.

The Globular service cert was present, valid, and contained the
right SAN (`IP:10.0.0.63`) — scylla-manager simply wasn't pointed
at it.

## Live operational fix

Edited `/var/lib/globular/scylla-manager/scylla-manager.yaml` (backup
preserved at `scylla-manager.yaml.bak-1780538413`), adding two lines:

```yaml
tls_cert_file: /var/lib/globular/pki/issued/services/service.crt
tls_key_file: /var/lib/globular/pki/issued/services/service.key
```

Then restarted **only** `globular-scylla-manager.service`. No other
mutation: no etcd write, no CA change, no doctor-trust-path change,
no cert regeneration, no apply-desired.

## Permanent fix

`packages` repo commit `ea647b7` — _scylla-manager: configure script
must emit TLS material for HTTPS_. Three files changed:

- `metadata/scylla-manager/package.json` — version bump
  `3.10.1` → `3.10.1+1` (Globular-internal suffix; upstream binary
  unchanged)
- `metadata/scylla-manager/specs/scylla_manager_service.yaml` — the
  install-time `scylla-manager-configure` script now writes
  `https: ${CQL_HOST}:5443`, `tls_cert_file: {{.StateDir}}/pki/issued/services/service.crt`,
  and `tls_key_file: {{.StateDir}}/pki/issued/services/service.key`
- `metadata/scylla-manager/scripts/configure_test.sh` — new, 115 lines,
  12 test cases (static contract on the spec yaml + simulated install
  with mocked `CQL_HOST` that asserts the generated yaml has all
  required keys and parses as valid yaml)

Artifact `scylla-manager_3.10.1+1_linux_amd64.tgz` is published to
the cluster repository.

**Desired version is intentionally still `3.10.1`.** The new artifact
is in the repo for future installs; the existing node's yaml was
already brought into compliance by the operational fix above. Fresh
installs on new nodes will pick up the corrected configure script
automatically. Promoting `3.10.1+1` to desired would trigger a
reinstall whose configure script's "skip if config exists" guard
makes it a no-op on this node — safe, but not required to clear
the finding.

## Verification

| Check | Before | After |
|---|---|---|
| served cert issuer on `:5443` | `O = Scylla` (self-signed) | `CN = Globular Root CA, O = globular.internal` |
| `openssl verify -CAfile /var/lib/globular/pki/ca.crt` against served cert | fail (self-signed) | **OK** |
| `curl --cacert /var/lib/globular/pki/ca.crt https://10.0.0.63:5443/api/v1/clusters` | TLS failure | **HTTP 200** with valid cluster JSON |
| doctor `scylla_manager.cluster_registered` | WARN | **cleared (0 findings)** |
| cluster `overall_status` | WARN | **PASS** |
| `configure_test.sh` | n/a | **12 / 12 PASS** |

## Remaining findings

| Severity | Invariant | Entity |
|---|---|---|
| INFO | `artifact.layout_drift_local` | `/var/lib/globular` |

This is pre-existing housekeeping debt (legacy empty alias directories
under `/var/lib/globular/`) and is explicitly scoped out of this
incident. No other findings remain.
