# Envoy LDS-wedge: CDS progresses, LDS stays at zero

**Phase 28 investigation — 2026-06-03 — globule-ryzen (single-node observation, 5-node cluster)**

Anchored failure_mode: `envoy.lds_update_attempt_zero_despite_cds_progress`
Anchored invariant:    `envoy.lds_progress_required_for_http_mesh_readiness`
Status of those anchors: still valid; this report extends them with the *real* upstream cause.

## TL;DR

The symptom Phase 24 anchored (`listener_manager.lds.update_attempt = 0`
while `cluster_manager.cds.update_success > 0`, port 443 refused) is real and
reproducible on the live cluster. The cause is **not** an xDS protocol bug,
**not** a snapshot-emission bug, and **not** a bootstrap subscription bug.
The root cause is an **upstream restart-storm**: the workflow service
re-dispatches `node.maybe_restart_package` against the `envoy` package multiple
times per second, SIGTERM'ing Envoy before it has time to complete the LDS
half of the ADS handshake. CDS gets through because it fetches first in
init; LDS never gets a chance.

The fix landscape is layered, not single-source:
1. Doctor rule (Phase 28, this PR) — surfaces the wedge as CRITICAL.
2. Phase 27 (`951baeb8`) — install-skip path now verifies the running PID's
   binary matches expected. Closes the half where the skip path treated a
   checksum-mismatched-but-version-matching install as SUCCEEDED, leaving the
   controller stuck re-trying maybe_restart.
3. Workflow debounce on `node.maybe_restart_package` — **NOT in scope for this
   phase**; tracked as follow-up because it lives in the workflow engine and
   needs its own awareness pass.

## Live evidence

Probed on globule-ryzen (single node, 2026-06-03 ~00:46 EDT):

### Envoy admin stats (PID 1992227, ran for ~30s before being killed)

```
cluster_manager.cds.update_attempt:    5
cluster_manager.cds.update_success:    4
cluster_manager.cds.update_failure:    0
cluster_manager.cds.update_rejected:   0
cluster_manager.cds.version_text:      "1780461885739778298"

listener_manager.lds.update_attempt:   0
listener_manager.lds.update_success:   0
listener_manager.lds.update_failure:   0
listener_manager.lds.update_rejected:  0
listener_manager.lds.version_text:     ""
```

### Bootstrap (correct — not the bug)

`/run/globular/envoy/envoy-bootstrap.json` declares both CDS and LDS via ADS:

```json
"dynamic_resources": {
  "ads_config": {"api_type": "GRPC", "grpc_services": [{"envoy_grpc": {"cluster_name": "xds_cluster"}}], "transport_api_version": "V3"},
  "cds_config": {"ads": {}, "resource_api_version": "V3"},
  "lds_config": {"ads": {}, "resource_api_version": "V3"}
}
```

Node id `globular-xds`, cluster `globular-cluster`. Matches what the xDS
server's IDHash cache key expects.

### xDS server snapshot (correct — not the bug)

`globular-xds.service` logs every 5 s:

```
[DEBUG] === BuildSnapshot ENTERED === version=…, len(Routes)=27
[DEBUG] BuildSnapshot: Creating snapshot with resources:
[DEBUG]   - type.googleapis.com/envoy.config.route.v3.RouteConfiguration: 2 resources
[DEBUG]   - type.googleapis.com/envoy.config.listener.v3.Listener: 2 resources
[DEBUG]   - type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret: 3 resources
[DEBUG]   - type.googleapis.com/envoy.config.cluster.v3.Cluster: 26 resources
xDS snapshot pushed node_id=globular-xds version=…
```

The snapshot contains 2 listeners (`ingress_listener_443` on 0.0.0.0:443 and
`ingress_listener_443_http_80` on 0.0.0.0:80) every push.
`Consistent()` passes — SetSnapshot returns nil.

### Port 443 reality

```
$ sudo ss -ltnp | grep -E ':(443|80) '
(empty — nothing bound)

$ curl -sk https://127.0.0.1:443/
* connect to 127.0.0.1 port 443 from 127.0.0.1 port 45992 failed: Connection refused
```

### Envoy lifecycle — the smoking gun

`journalctl -u globular-envoy.service` shows the restart-storm pattern over
seconds, not minutes:

```
00:47:48.117 envoy[2000701]: caught ENVOY_SIGTERM    ← only ~400ms uptime
00:47:48.269 envoy[2000726]: starting
00:47:48.587 envoy[2000726]: caught ENVOY_SIGTERM    ← ~300ms uptime
00:47:48.741 envoy[2000740]: starting
00:47:49.046 envoy[2000740]: caught ENVOY_SIGTERM    ← ~300ms uptime
00:47:49.200 envoy[2000755]: starting
00:47:49.549 envoy[2000755]: caught ENVOY_SIGTERM    ← ~300ms uptime
00:47:49.702 envoy[2000769]: starting
00:47:49.991 envoy[2000769]: caught ENVOY_SIGTERM    ← ~300ms uptime
00:47:50 systemd: globular-envoy.service: Start request repeated too quickly.
00:47:50 systemd: globular-envoy.service: Failed with result 'start-limit-hit'.
```

The same node_agent journal shows the SIGTERM source — fanned-out
`install-package envoy` dispatches arriving at the same wall-clock window:

```
00:47:47 grpc-workflow: install-package envoy: checksum 241c1702… != desired cfd6de59…
00:47:47 apply-package: INFRASTRUCTURE/envoy@1.35.3 (build 0, build_id=019e88ce-…) already installed, skipping
00:47:47 grpc-workflow: install-package envoy SUCCEEDED (6.62274ms, status=skipped)
00:47:48 grpc-workflow: install-package envoy: checksum 241c1702… != desired cfd6de59…
00:47:48 apply-package: already installed, skipping
00:47:48 grpc-workflow: install-package envoy SUCCEEDED (2.84ms, status=skipped)
…  (4 more cycles in the same second)
```

And earlier, at 00:35, the controller-driven loop:

```
00:35:47 workflow_server: dispatching action node-agent action=node.maybe_restart_package
                          run_id=InfrastructureRelease/core@globular.io/envoy[0] step_id=maybe_restart
00:35:47 envoy: caught ENVOY_SIGTERM
00:35:47 systemd: Stopping globular-envoy.service
00:35:47 envoy: initializing epoch 0
00:35:48 workflow_server: dispatching action node.maybe_restart_package (same run_id, again)
00:35:48 envoy: caught ENVOY_SIGTERM
00:35:48 systemd: Stopping globular-envoy.service
00:35:48 envoy: initializing epoch 0
…
```

## Why CDS progresses but LDS does not

Envoy init sequence in ADS/SOTW (verbatim from Envoy source, relevant subset):

1. Bootstrap parsed; ADS gRPC stream opens to `xds_cluster`.
2. **CDS DiscoveryRequest** sent first (clusters are init-blocking).
3. Cluster init: every dynamic cluster must reach "initialized" (DNS / endpoint
   resolution / health-check warm-up).
4. **LDS DiscoveryRequest** sent only after CDS reaches steady-state.
5. Listeners parsed → sockets bound → port 443 live.

Steps 1–2 finish in low-100s of ms. Step 3 takes seconds (26 clusters with
SDS-wrapped mTLS, per-cluster health-check). Step 4 onwards is what
`listener_manager.lds.update_attempt` measures.

If Envoy is SIGTERM'd between step 2 (CDS pushed) and step 4 (LDS requested),
the CDS counter ticks and the LDS counter stays at 0 forever. That is exactly
what the 300-millisecond-uptime restart cycles produce.

The Phase 24 anchor description ("Envoy consumes CDS updates but never
attempts LDS") is correct at the symptom layer; what it could not see — and
what this report adds — is that the *cause* is restart-storm, not protocol
divergence.

## What the doctor rule (Phase 28) adds

`golang/cluster_doctor/cluster_doctor_server/rules/envoy_lds_wedge.go`
fires CRITICAL with `InvariantStatus_INVARIANT_FAIL` and
`InvariantID = envoy.lds_progress_required_for_http_mesh_readiness` when:

- `envoy_cluster_manager_cds_update_success > 0`, AND
- `envoy_listener_manager_lds_update_attempt == 0`.

It stays silent during cold init (CDS still 0), stays silent when LDS has
attempted at least once (handshake working, rejection diagnosis owned
elsewhere), and emits an INFO + PASS finding when LDS is healthy so the
ledger records the healthy state.

The rule is **diagnostic-only**. It does NOT restart Envoy, does NOT touch
the xDS snapshot, does NOT mutate desired state. That is deliberate: every
restart we trigger here would deepen the wedge while the upstream
`maybe_restart_package` storm is still running.

## What the Phase 27 fix (`951baeb8`) already addresses

The skip-without-runtime-proof path (which let the storm continue forever
because every `install-package envoy` returned SUCCEEDED status=skipped
despite the checksum mismatch) is now fixed. On a deployed cluster running
the Phase 27 fix, the skip path will return FAILED when the running PID's
binary doesn't match expected, the controller will dispatch a real reinstall,
and the binary on disk will be corrected. That stops the *checksum-mismatch
half* of the loop.

## What is NOT in scope here

- **Workflow-engine debounce on `node.maybe_restart_package`.** The action is
  re-dispatched on every workflow tick. There is no "we just restarted this
  unit X seconds ago, skip" guard. Adding one needs its own awareness pass
  (touches the workflow engine, has its own incident-pattern surface area)
  and is tracked as a follow-up.
- **Readiness classifier change** ("don't mark envoy healthy unless
  `lds.update_success > 0`"). That belongs in node-agent's
  envoy-readiness path; doctor classifying the data-plane is sufficient
  for this phase.

## Safe operator workaround

When the doctor rule fires `envoy.lds_wedge`:

1. Look at the workflow journal for repeated dispatches against
   `globular-envoy.service`:
   ```bash
   journalctl -u globular-workflow.service --since '10min ago' | grep envoy | head -40
   ```
2. If the loop is active, **stop the storm first**: pause the offending
   workflow run via the typed API (`workflow_stop`); do **not** kill envoy
   manually — the storm will just restart it.
3. Verify the on-disk envoy binary checksum matches the desired manifest:
   ```bash
   sha256sum /usr/lib/globular/bin/envoy
   ```
   If it doesn't match, the binary is the old version and needs a real
   reinstall, not a `maybe_restart`. Force a real reinstall via the typed
   install RPC (Phase 27 fix will route this correctly once deployed).
4. Once the storm is silent for ≥30 s, `systemctl restart globular-envoy.service`
   once. With no SIGTERM stomping on it, Envoy reaches step 4 of init,
   LDS updates apply, port 443 binds.

## Forbidden shortcuts

- **Do NOT** add a localhost fallback in the gateway or in service clients
  to "work around" 443 being down. The fallback hides the wedge and
  silently changes the trust boundary (no mTLS termination).
- **Do NOT** classify systemd-active Envoy as healthy. `systemctl is-active`
  returns `active` for a process that's been alive for 300 ms — that is the
  same process that's about to be SIGTERM'd. Health must include
  `lds.update_success > 0`.
- **Do NOT** infinite-restart Envoy as a remediation. It IS the loop.
  The doctor rule is diagnostic-only for exactly this reason.
- **Do NOT** bypass Envoy in code paths as a permanent workaround. Tactical
  bypass during incident recovery is fine; permanent bypass erodes the mesh.
- **Do NOT** edit etcd to fake convergence of `envoy` so the workflow stops
  re-dispatching. The convergence record must reflect reality.

## Remaining risks after Phase 28

- The doctor rule fires on the symptom. If Prometheus stops scraping Envoy
  (Prometheus down, scrape misconfigured, Envoy unreachable from Prom), the
  rule silently produces no finding. Belt-and-braces: the existing
  `node_units_running` invariant covers the systemd path; a future
  enhancement could cross-check.
- The Phase 27 fix is not yet deployed. Until v1.2.144+ is on every node, the
  skip-without-runtime-proof path keeps the loop primed.
- The workflow-engine debounce is the structural fix and is still owing.
  Without it, even with Phase 27, a transient checksum mismatch (or any
  other reason the controller decides to re-dispatch) can re-enter the same
  300-ms restart cycle.
