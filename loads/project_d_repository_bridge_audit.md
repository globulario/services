# Project D Repository Bridge Audit

## Bridge events

Two package-produced bridges executed during Project D validation.

### Bridge 1 — v1.2.120

| Field | Value |
|---|---|
| Version | 1.2.120 |
| Package path | `/var/lib/globular/packages/pinned/repository_1.2.120_linux_amd64.tgz` |
| Package sha256 | `0f72f6a3dc0f1f3ee03310a33bcbddeac6444cd9d8ac7c69de8398df4dc0c34d` |
| Manifest entrypointChecksum | `sha256:e8948d6d4b1f8a79754858d007fb64770a298b6bd257e5b0940a61d15f9f48a7` |
| Extracted binary sha256 | `e8948d6d4b1f8a79754858d007fb64770a298b6bd257e5b0940a61d15f9f48a7` |
| Installed binary sha256 | `e8948d6d4b1f8a79754858d007fb64770a298b6bd257e5b0940a61d15f9f48a7` |
| Previous binary backup | `/var/lib/globular/recovery/backups/repository-bridge-1.2.120-20260528-215644/repository_server.before-1.2.120` |
| Restart time | 2026-05-28 21:56:44 EDT |
| Reason | Activate Project D RepairArtifact CAS-fallback fix (commit `931318db`) |
| Approved scope | Bounded package-produced recovery bridge |

### Bridge 2 — v1.2.121

| Field | Value |
|---|---|
| Version | 1.2.121 |
| Package path | `/var/lib/globular/packages/pinned/repository_1.2.121_linux_amd64.tgz` |
| Package sha256 | `23f89c183ef60df7d1deeadd0f5f50bc6a650976550e0855ff4c975d56fbaf04` |
| Manifest entrypointChecksum | `sha256:56b4462eeffddf5975f0f70afd5f8f9d8e7df5dccc8eb03ef716843a501ff9d4` |
| Extracted binary sha256 | `56b4462eeffddf5975f0f70afd5f8f9d8e7df5dccc8eb03ef716843a501ff9d4` |
| Installed binary sha256 | `56b4462eeffddf5975f0f70afd5f8f9d8e7df5dccc8eb03ef716843a501ff9d4` |
| Previous binary backup | `/var/lib/globular/recovery/backups/repository-bridge-1.2.121-…` |
| Reason | Activate direct-write backfill fix (commit `6a5bd635`) after live test of v1.2.120 revealed state-machine rejection of DOWNLOADING→PUBLISHED |
| Approved scope | Bounded package-produced recovery bridge |

### Bridge 3 — v1.2.122

| Field | Value |
|---|---|
| Version | 1.2.122 |
| Package path | `/var/lib/globular/packages/pinned/repository_1.2.122_linux_amd64.tgz` |
| Manifest entrypointChecksum | `sha256:879e841827e74446259b878c354de4f92d9a48859546ee4ae0021b618f00a79a` |
| Extracted binary sha256 | `879e841827e74446259b878c354de4f92d9a48859546ee4ae0021b618f00a79a` |
| Installed binary sha256 | `879e841827e74446259b878c354de4f92d9a48859546ee4ae0021b618f00a79a` |
| Reason | Activate broadened trigger predicate (commit `248857dd`) after live test of v1.2.121 revealed `gateway`/`globular-cli` had artifact_state=PUBLISHED but manifest_json=NULL — repair skipped because the precondition only checked artifact_state |
| Approved scope | Bounded package-produced recovery bridge |

## Forbidden actions explicitly NOT taken

- No `cp` of a locally-built binary (every bridge sourced from
  `/var/lib/globular/packages/pinned/repository_*.tgz` produced by
  `globular deploy repository --bump patch`).
- No identity bypass — every bridge confirmed
  `extracted_binary_sha256 == manifest.entrypointChecksum` before
  install.
- No `etcdctl put` on desired-state or installed-state keys.
- No `cqlsh INSERT/UPDATE` on `repository.manifests`.
- No `event` hash_drift clearing by manual mutation.
- No widening of allowed pipeline transitions in the artifact-state
  machine.
- No fabricated manifest_json values — every row's manifest_json was
  derived from the validated local CAS `.manifest.json` file.

## Why bridges were needed

The normal repository install was gated by
`dependency_not_ready: [event]` because the `event` service is in
hash_drift state. The Project B runtime proof writer covers the 3
self-hosted control-plane components but not ordinary services like
`event`. The fixed repository binary was therefore unable to become
active through normal reconciliation.

Per the user's handoff (`claude_project_d_repository_bridge_validation_instructions.md`):

> Proceed with Option 1 only as a bounded package-produced bridge.
> This is approved only if:
>   repository v1.2.120 comes from the globular deploy/package pipeline
>   and extracted binary sha256 matches manifest entrypoint_checksum

Three iterations were required because each bridge revealed a
deeper-layer issue once the previous fix was live:
1. v1.2.120 — discovered the verifier needed CAS-file fallback for
   NULL manifest_json (commit `931318db`).
2. v1.2.121 — discovered `completePublish` rejects the
   DOWNLOADING→PUBLISHED state transition (commit `6a5bd635`).
3. v1.2.122 — discovered the trigger predicate also needed to fire
   when artifact_state=PUBLISHED but manifest_json=NULL (commit
   `248857dd`).

Each bridge restored the next-layer fix; all 15 target artifacts now
have `publish_state=PUBLISHED` and `manifest_json` populated.
