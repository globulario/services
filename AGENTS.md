# Globular Agent Instructions

## Prime Rules

1. Walk the 4 layers before debugging: Repository → Desired → Installed → Runtime.
2. Never confuse platform release with package version. release-index.json is the platform truth.
3. Never confuse build_id (UUID) with build_number (integer).
4. Never duplicate package kind classification. Use the canonical package registry.
5. Know the encoding before changing a type: proto, JSON, or internal-only.
6. Know the route before calling an RPC: mesh-routable or direct-only.
7. GitHub is a provider, not the architecture. Controller and node-agent stay provider-neutral.
8. MinIO is a cache for packages, not package authority.
9. Day-0 reads release-index.json. Day-1 joins from active BOM, not latest.
10. Do not infer truth from filenames when a manifest exists.
11. Keep scripts phase-oriented and idempotent.
12. Every fix should add or update a regression test.

## Hard Rules

- **etcd is the sole source of truth** — no env vars, no hardcoded addresses or gRPC ports.
- **NO localhost/127.0.0.1** for remote addresses — resolve from etcd. Bind/listen uses `0.0.0.0`.
- **4-layer state model is sacred** — Repository → Desired → Installed → Runtime are independent. Never collapse.
- **All state changes flow through workflows** — idempotent, reach terminal state (SUCCEEDED or FAILED).
- **Founding quorum** — etcd on ALL nodes, ScyllaDB≥3, MinIO≥3. First 3 nodes MUST have core+control-plane+storage.
- **NO automatic rollback** — crash → incident. Rollback = explicit Force=true via CLI only.
- **cluster_controller MUST NOT use os/exec** — node_agent can only use os/exec within internal/supervisor/.
- **No tokens in codebase** — tokens are ephemeral, generated at runtime.

Full rules: CLAUDE.md and docs/ai/ai-operating-rules.md
