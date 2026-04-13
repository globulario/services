# Globular CLI (globular / globularcli) — Quickstart

Purpose: how to talk to the cluster from the CLI with correct endpoints, TLS, and token usage. For full guide see `golang/globularcli/README.md`; this summarizes essentials for operators/automation.

## Endpoints & auth
- Controller: dynamic port from etcd; discover via MCP `cluster_get_health` or `globular services list` against control plane; defaults in README (e.g., 10000) may not match runtime.
- Node agent: per-node port from allocator; discover via MCP/etcd.
- Token: pass with `--token <jwt>`; fetch from existing `~/.config/globular/token` or generate via your auth flow.
- TLS: use `--ca /var/lib/globular/pki/ca.crt` and client certs if needed; avoid `--insecure` in prod.

## Basic patterns
List nodes (JSON):
```bash
globular --controller <host:port> --output json cluster nodes list
```

Set desired service version (rollout):
```bash
globular services desired set <service-name> <version>
```

Publish a package:
```bash
globular pkg publish --repository <repo-host:port> --file <pkg.tgz> --force
```

Node inventory:
```bash
globular --node <node-host:port> --output json node inventory
```

Logs (service on a node):
```bash
globular --node <node-host:port> logs service --unit globular-ai-memory.service --tail 200
```

Doctor report via controller:
```bash
globular --controller <host:port> doctor report --output json
```

## Useful commands by area
- Cluster: `cluster health`, `cluster nodes list`, `cluster join-token create`
- Services: `services desired set/get`, `services list`, `services releases watch`
- Packages: `pkg build`, `pkg publish`, `pkg info`
- Releases: `release create/apply/status`, `release watch`
- DNS: `dns records list`, `dns resolve`
- Backup: `backup jobs list`, `backup run`, `backup restore`
- AI: `ai executor status`, `ai memory query` (when available via CLI bindings)
- Support bundle: `support-bundle create` (collects diagnostics)

## Tips
- Ports are dynamic; always resolve from etcd/MCP before calling.
- Reflection is enabled on most services; `globular` CLI can often auto-discover gRPC surfaces, but raw grpcurl needs the right host/port.
- Scripts in `globular-installer/scripts` handle Day-0 setup; use the CLI for day-1+ operations and rollouts.
