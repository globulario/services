# Config Ownership Audit

Date: 2026-06-28
Package repair: `globulario/packages#11`

Scope:
- Services enforcement: `golang/globularcli/pkgpack/spec_seed_guard_test.go`
- Package source: adjacent `packages` repository, both legacy `specs/` and current `metadata/*/specs/` layouts
- Guardrail: `config_ownership`

## Contract

Package install may seed cluster-owned configuration only when the target file is
absent. A reinstall or upgrade must not overwrite live configuration owned by an
operator, workflow, identity/join flow, controller, or node-agent runtime policy.

The services guard now fails closed:
- Missing package specs are test failures, not skipped cases.
- Duplicate current-layout specs are test failures.
- The current metadata layout is audited directly; legacy `packages/specs/`
  files are only a fallback when metadata specs are absent.

## Audited Seeds

| Spec | Path | Owner mode | Required protection |
| --- | --- | --- | --- |
| `etcd_service.yaml` | `{{.StateDir}}/config/etcd.yaml` | contract-rendered | `skip_if_exists: true` |
| `minio_service.yaml` | `{{.StateDir}}/minio/minio.env` | contract-rendered | `skip_if_exists: true` |
| `minio_service.yaml` | `{{.StateDir}}/minio/credentials` | identity-owned | `skip_if_exists: true` |
| `prometheus_service.yaml` | `{{.StateDir}}/prometheus/prometheus.yml` | seed-only | `skip_if_exists: true` |
| `alertmanager_service.yaml` | `{{.StateDir}}/alertmanager/alertmanager.yml` | seed-only | `skip_if_exists: true` |
| `xds_service.yaml` | `{{.StateDir}}/xds/xds.yaml` | seed-only | `skip_if_exists: true` |
| `xds_service.yaml` | `{{.StateDir}}/xds/config.json` | seed-only | `skip_if_exists: true` |
| `mcp_service.yaml` | `{{.StateDir}}/mcp/config.json` | seed-only | `skip_if_exists: true` |
| `sidekick_service.yaml` | `{{.StateDir}}/sidekick/sidekick.env` | seed-only | `skip_if_exists: true` |
| `scylla_manager_agent_service.yaml` | `{{.StateDir}}/scylla-manager-agent/scylla-manager-agent.yaml` | seed-only | `skip_if_exists: true` |

## Repair Notes

The audit found stale services coverage and package-side misses:
- The services test still read `packages/specs/*.yaml`, so it skipped the current
  `packages/metadata/*/specs/*.yaml` layout.
- `xds_service.yaml` seeded `xds.yaml` and `config.json` without
  `skip_if_exists: true`.
- `xds_service.yaml` also set `install_config: true`, allowing bundled config to
  be copied by the payload installer after the explicit seed step.
- `sidekick_service.yaml` seeded `sidekick.env` without `skip_if_exists: true`.

After repair, the package specs preserve all audited live config seeds and the
services-side guard enforces the current package metadata layout.
