# Project U.1 — scylla-manager HTTPS listener enabled (HTTP retained)

## Status

**COMPLETE.** HTTPS listener bound at `10.0.0.63:5443` with Globular PKI;
HTTP listener at `10.0.0.63:5080` retained as transitional fallback.
Real scylla-manager process running, NRestarts=0 across the deploy
window, Project R/S backup state intact, doctor still 0 scylla-manager
findings.

## Config before / after

### Before (preserved at `loads/scylla_manager_yaml_pre_U1_20260529_120447.yaml`)

```yaml
# Scylla Manager configuration (managed by Globular)
http: 10.0.0.63:5080

database:
  hosts:
    - 10.0.0.63
  port: 9042
```

### After (live at `/var/lib/globular/scylla-manager/scylla-manager.yaml`)

```yaml
# Scylla Manager configuration (managed by Globular)
# Project U.1: HTTPS enabled with Globular PKI; HTTP kept transitional
http: 10.0.0.63:5080
https: 10.0.0.63:5443
tls_cert_file: /var/lib/globular/pki/issued/services/service.crt
tls_key_file: /var/lib/globular/pki/issued/services/service.key

database:
  hosts:
    - 10.0.0.63
  port: 9042
```

Three lines added: `https:`, `tls_cert_file:`, `tls_key_file:`. The
file is owned by `globular:globular` with mode 0640.

## Cert / key paths used

| Asset | Path | Owner | Mode | Verified readable by `globular` user |
|---|---|---|---|---|
| Cert | `/var/lib/globular/pki/issued/services/service.crt` | globular:globular | 0444 | ✓ |
| Key | `/var/lib/globular/pki/issued/services/service.key` | globular:globular | 0400 | ✓ |
| CA (for clients) | `/var/lib/globular/pki/ca.crt` | globular:globular | 0644 | n/a (read by clients) |

The cert SANs already cover every name an HTTPS endpoint would
advertise on this node (`IP:10.0.0.63`, `DNS:globule-ryzen`,
`DNS:*.globular.internal`, `DNS:localhost`). No new issuance was
required; this matches the Project U planning report's prediction.

## Restart result

```
systemctl restart globular-scylla-manager.service
sleep 6

ActiveState=active
SubState=running
MainPID=741700
NRestarts=0
Result=success
process: /usr/lib/globular/bin/scylla_manager --config-file
         /var/lib/globular/scylla-manager/scylla-manager.yaml
```

Real scylla-manager binary running (not the legacy `/bin/sleep
infinity` placeholder which was removed back in Project R / Project T).

## Both listeners bound

```
ss -lntp | grep -E ":5080|:5443"
LISTEN 0 4096 10.0.0.63:5080 0.0.0.0:* users:(("scylla_manager",pid=741700,fd=27))
LISTEN 0 4096 10.0.0.63:5443 0.0.0.0:* users:(("scylla_manager",pid=741700,fd=28))
```

One process (PID 741700) owns both ports — confirms scylla-manager
accepted the config and didn't silently fall back to HTTP-only.

## HTTP probe result (unchanged behavior)

```
$ curl -sf -m 3 -w "HTTP %{http_code}\n" http://10.0.0.63:5080/api/v1/version
{"version":"3.10.1"}
HTTP 200
```

## HTTPS probe result (strict CA verification)

```
$ curl -sf -m 3 --cacert /var/lib/globular/pki/ca.crt \
       -w "HTTP %{http_code} ssl_verify=%{ssl_verify_result}\n" \
       https://10.0.0.63:5443/api/v1/version
{"version":"3.10.1"}
HTTP 200 ssl_verify=0
```

`ssl_verify=0` is curl's "verification succeeded" code. The endpoint
returns the same version payload as HTTP. The cluster list returns
identically over both schemes (verified below).

## Certificate validation result

```
$ echo Q | openssl s_client -connect 10.0.0.63:5443 \
              -CAfile /var/lib/globular/pki/ca.crt \
              -verify_return_error 2>&1 | grep -E "Verify|subject=|issuer="
subject=CN = globule-ryzen, O = globular.internal
issuer=CN = Globular Root CA, O = globular.internal
Verify return code: 0 (ok)
    Verify return code: 0 (ok)
```

`-verify_return_error` makes openssl exit non-zero on validation
failure; the command succeeded. Subject CN matches this node's
hostname; issuer is the Globular Root CA. The chain validates cleanly
without `-k` / `--insecure` shortcuts.

## Cluster registration still present (via the new HTTPS probe)

```
$ curl -sf -m 3 --cacert /var/lib/globular/pki/ca.crt \
       https://10.0.0.63:5443/api/v1/clusters
[{"id":"932c01cb-8c50-4a30-b90d-e2f08c10a17c","name":"globular-internal",
  "host":"10.0.0.63","port":5612, ...}]
```

Same cluster ID, same name, same host. No duplicate, no drift.

## Healthcheck evidence

```
$ sctool --api-url http://10.0.0.63:5080/api/v1 tasks -c globular-internal
healthcheck/cql        | Success=81 | Error=0 | Last 12:04:00 EDT | DONE | Next 12:06:00
healthcheck/alternator | Success=81 | Error=0 | Last 12:04:00 EDT | DONE | Next 12:06:00
healthcheck/rest       | Success=81 | Error=0 | Last 12:04:00 EDT | DONE | Next 12:06:00
repair/all-weekly      | Success=0  | Error=0 |                    | NEW  | Next 30 May 23:00
```

Success counts continue to climb (was 56 at end of Project S deploy →
60 → 81 now). Zero errors. The Project R backups still listed and
still in DONE state.

`sctool` was invoked over HTTP per the Project U plan; upstream
sctool 3.10.1 lacks `--ca-file` for the API URL, so the HTTPS
migration leaves sctool on HTTP until upstream support lands or U.2
introduces a curl-based fallback.

## Backup state evidence

| Check | State |
|---|---|
| `backup/3b966c52-056e-47ca-9c2b-55e313b8b689` | `DONE`, success=1, error=0 (unchanged from Project R) |
| `backup/105a3d1f-8625-4298-9374-dfc4ee4cf664` | `DONE`, success=1, error=0 (unchanged from Project R) |
| MinIO `scylla-manager-backup` object count | **378** (unchanged from Project R/S/Q post-deploy snapshots) |

Backup readiness preserved end-to-end. No restore test was run; the
Project R dry-run remains valid (the manifest is unchanged in MinIO).

## Doctor before / after delta

| Metric | Pre-U.1 (post-Project-Q) | Post-U.1 |
|---|---|---|
| Total findings | 25 | 24 |
| scylla-manager findings | **0** | **0** |

The 25→24 delta is unrelated to U.1 — the Project Q deploy left a
transient `cluster.services.drift` finding for `cluster-controller`
during convergence; the controller has now converged so that finding
cleared. No scylla-manager regression.

The doctor's `scylla_manager.cluster_registered` invariant continues
to probe the manager via the configured `scyllaManagerEndpoint` (which
is HTTP today — U.3 will migrate this to HTTPS).

## Stability check

```
60-second stability check:
  start PID: 741700
  end PID:   741700
  NRestarts: 0
✓ STABLE
```

No mid-flight restart, no JSON-error-driven exit. The single
"unexpected end of JSON input" log line observed during the first 30s
window is the upstream scylla-manager 3.10.1 orphan-row class
documented since Project R — non-fatal, doesn't trigger a restart.

## Recommendation for U.2

**Authorized to proceed with U.2 when ready.**

U.2 is the registration script update: prefer HTTPS, fall back to
HTTP. The probe path `https://10.0.0.63:5443/api/v1/version` is now
proven reachable with strict CA validation; the script can use
`curl --cacert /var/lib/globular/pki/ca.crt` against that URL and
fall through to `http://10.0.0.63:5080/api/v1/version` only if the
HTTPS probe fails.

Key constraints for U.2 (from the planning report):

- `sctool cluster add` keeps calling `http://...:5080/api/v1` until
  upstream sctool ships `--ca-file`. The script's *probe* path uses
  HTTPS; the *write* path stays on HTTP for now. This is a
  deliberate hybrid documented in the Project U plan.
- Script must remain idempotent (proven 3× during Project S deploy).
- Script must exit 0 on probe failure so the unit stays up; doctor's
  `scylla_manager.cluster_registered` rule surfaces the unregistered
  state.

U.3 (doctor invariant) becomes the natural next step after U.2 since
both observe the same endpoint.

## Rollback (if needed)

```
1. sudo cp loads/scylla_manager_yaml_pre_U1_20260529_120447.yaml \
            /var/lib/globular/scylla-manager/scylla-manager.yaml
2. sudo chown globular:globular /var/lib/globular/scylla-manager/scylla-manager.yaml
3. sudo systemctl restart globular-scylla-manager.service
```

The HTTP listener has never been disabled in this ticket, so even
without the rollback the cluster remains functional through the HTTP
path. The rollback only removes the HTTPS listener.

## Stop condition honored

- ✅ Did not modify scylla_manager keyspace state.
- ✅ Did not touch backup tasks or MinIO artifacts.
- ✅ Did not rebuild any package (this was a config-file edit only).
- ✅ Did not disable HTTP.
- ✅ Did not implement U.2 or U.3.

## Evidence files

- `loads/scylla_manager_yaml_pre_U1_20260529_120447.yaml` (rollback artifact)
- `/tmp/dr_U1.json` (doctor snapshot used for the delta — not preserved long-term)
