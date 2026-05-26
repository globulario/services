---
name: ScyllaDB wipe loop — INC-2026-0007
description: Root cause and fix for the ScyllaDB wipe loop that destroyed all keyspaces every ~90s on ryzen
type: project
---

**INC-2026-0007 RESOLVED** (2026-05-25)

## Symptoms
- All ScyllaDB keyspaces (dns, ai_memory, globular_projections, workflow, local_resource, globular_events, repository) wiped every ~90s
- DDL preflight blocked: `raft_group0_members_table_missing` (reason=unknown)
- Services failing: ai-memory inactive, workflow circuit breaker OPEN, DNS degraded
- Controller log: "wiping stale Raft data and restarting (attempt N)" every ~90s

## Root Causes

**RC-1: Probe false-negative in infra_health_probe.go**
`validateScyllaRuntimePrereqs()` checked that `service.key` is readable by scylla user (UID 123).
File is `0400 owner=997:986` (globular service account). ScyllaDB's rendered `scylla.yaml` uses
plain CQL — NO TLS — and never reads `service.key`. The check was a leftover for a future
TLS feature that was never implemented. Probe always returned FAILED → `ScyllaJoinPhase` never
reached Verified → `ScyllaWasEverVerified` stayed false → wipe fired every ~90s.

**RC-2: DDL preflight doesn't know about system.raft_state (ScyllaDB 2025.x+)**
ScyllaDB 2025.3.x renamed `system.raft_group0_members` → `system.raft_state` with different
schema. Preflight always returned `DDLPreflightUnknown` → schema guard couldn't auto-create
missing keyspaces. This meant manually created keyspaces couldn't be recreated by the guard.

## Resolution

**Immediate (2026-05-25):**
1. `chmod o+r /var/lib/globular/pki/issued/services/service.key` via globular CLI (file is
   owned by globular user 997 who can chmod without sudo). Probe passed at 17:10:53, wipe stopped.
2. Manually created 7 missing keyspaces via cqlsh (RF=1 SimpleStrategy, single-node cluster).
3. Restarted affected services: ai-memory, workflow, dns, resource, event, authentication,
   rbac, log, cluster-doctor, backup-manager.

**Code fixes (committed, need CI deploy):**
- commit `6ca288e6`: node-agent `infra_health_probe.go` — remove service.crt/key from prereq
  check. ScyllaDB doesn't use them; only ca.crt readability retained.
- commit `c2e258a4`: cluster-controller `scylla_schema_preflight.go` — add `queryRaftStateFallback`
  to try `system.raft_state` when `system.raft_group0_members` is absent.

**After deploy:**
- Revert chmod: `chmod 0400 /var/lib/globular/pki/issued/services/service.key`
- Schema guard will auto-manage keyspace RF as cluster scales

## Key Invariant
The `validateScyllaRuntimePrereqs()` must ONLY check files that ScyllaDB actually reads.
Check the rendered scylla.yaml (via `service_config.go::renderScyllaConfig`) before adding
any file to the prereq list. Globular gRPC certs (service.crt/key) are NOT ScyllaDB files.

**Why:** ScyllaWasEverVerified=false + failing probe = infinite wipe loop destroying all data.
This is the worst possible failure mode for a storage node.
