# Docker Cluster Simulation

The `globulario/globular-quickstart` repository provides a Docker-based 5-node Globular cluster for development and testing. It runs unmodified production binaries inside systemd-in-Docker containers, simulating a real cluster from cold boot with zero prior state.

## Why This Exists

Bare-metal clusters mask cold-boot bugs because services are already registered in etcd from prior runs. The Docker simulation starts from zero every time, which exposed and helped fix:

- Hardcoded `127.0.0.1` defaults in service constructors that poisoned the etcd registry
- One-shot service resolution at startup that permanently failed on cold boot
- Mesh routing (Envoy) stripping auth context from inter-service gRPC calls
- FQDN-to-IP mismatch in DNS A record generation

These were all latent production bugs — the Docker simulation is a pressure chamber that surfaces issues the bare-metal cluster hides.

## Running It

```bash
cd globular-quickstart
make up      # builds image (~2.8GB) + starts 5-node cluster + ScyllaDB
make status  # check health
make shell N=1  # exec into a node
```

Full convergence takes ~3 minutes: etcd → ScyllaDB → services → profiles → DNS → workflow.

## What Gets Tested

The simulation validates the entire stack end-to-end:

| Layer | What's proven |
|-------|---------------|
| Transport | etcd mTLS, gRPC mTLS, MinIO HTTPS, ScyllaDB — all using cluster CA |
| Identity | CA → certs → Ed25519 keys → JWT tokens → interceptor chain |
| Storage | etcd (config), ScyllaDB (8 keyspaces), MinIO (workflow definitions) |
| Discovery | Service registration, DNS reconciler (11 A + 3 SRV), ClusterResolver |
| Control-plane | Leader election, 5-node heartbeats, profile auto-derivation |
| Workflow | Full `cluster.reconcile` execution: 7 steps, persisted in ScyllaDB |

## Architecture

```
5 Globular nodes (systemd-in-Docker, privileged)
  node-1 (control-plane, gateway)  — 15 services
  node-2 (control-plane, storage)  — 20 services (includes MinIO)
  node-3 (control-plane, ai)       — 19 services
  node-4 (compute)                 — node-agent only
  node-5 (compute)                 — node-agent only

1 ScyllaDB container (sidecar)     — shared database
```

All containers on a Docker bridge network (10.10.0.0/24). Docker provides only networking — DNS, discovery, TLS, and routing go through Globular's own stack.

## Cold-Boot Sequence

Understanding the boot order helps when debugging:

1. **entrypoint.sh** (before systemd): generates PKI, writes etcd config, renders unit templates, enables services based on profile
2. **systemd starts**: etcd first (no dependencies), then services in dependency order
3. **seed-etcd.service**: writes Tier-0 keys (ScyllaDB/MinIO/DNS hosts) to etcd
4. **Services register**: each service writes its config to etcd during `InitService()`
5. **Node agents heartbeat**: controller receives node status, auto-derives profiles
6. **DNS reconciler**: generates A/SRV records from profiles, pushes to DNS daemon
7. **configure-dns.sh**: rewrites `/etc/resolv.conf` to use Globular DNS
8. **Workflow execution**: controller dispatches `cluster.reconcile`, workflow service loads definition from MinIO, executes steps, persists to ScyllaDB

## Key Lessons for Developers

### Never hardcode 127.0.0.1 as a default
If etcd can't provide an infrastructure address, the service must error out — not silently fall back to localhost. A loopback default gets saved to etcd and becomes "truth" that persists across restarts.

### Service resolution must handle cold boot
Don't resolve peer addresses once at startup. Use lazy retry goroutines or re-resolve on each call. The service registry is empty during the first few seconds of a cold boot.

### Direct gRPC vs. mesh routing
When a service needs mTLS + token auth on an inter-service call, use the direct service port — not the Envoy mesh port (443). Envoy may strip auth metadata that the target service's interceptor requires.

### DNS A records need IPs, not hostnames
Any code that generates DNS A records must resolve hostnames to IPv4 addresses first. `net.ParseIP()` on a hostname returns nil, producing malformed DNS wire-format packets.

### Profile auto-derivation
Nodes auto-register with empty profiles on cold boot. The controller now derives profiles from installed packages (dns → control-plane, minio → storage, etc.). This is critical for DNS record generation and MinIO pool discovery.

## Modifying the Simulation

### Adding a service
1. Add the binary to the `Makefile` `ALL_BINS` list
2. The unit file is auto-collected from `/etc/systemd/system/globular-*.service`
3. If it needs a profile, add the enable logic in `entrypoint.sh`

### Changing the topology
Edit `docker-compose.yml` to add/remove nodes. Update `GLOBULAR_PROFILES` to change what each node runs. The profile auto-derivation handles the rest.

### Debugging a service
```bash
make shell N=2                              # exec into node-2
systemctl status globular-workflow          # check service status
journalctl -u globular-workflow -f          # follow logs
systemctl restart globular-workflow         # restart
etcdctl get /globular/services/ --prefix    # inspect service registry
```

## Relationship to Production

The simulation is **not** production. Key differences:

- No VIP/keepalived (Docker networking is flat)
- MinIO user created manually (Day-0 bootstrap not automated)
- Workflow definitions uploaded manually (not packaged)
- ScyllaDB runs as a single-node sidecar (not replicated)
- No ACME/Let's Encrypt (internal CA only)

But the service binaries, configuration model, auth chain, and convergence logic are identical to production. A bug found here is a real bug.
