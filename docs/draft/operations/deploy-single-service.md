# Deploying a Single Service (deploy-service.sh)

Script: `deploy-service.sh <service_name> [--comment "..."] [--version X.Y.Z] [--repository host:port]`

Flow:
1) Build Go binary for the service (injects `Version` and `BuildNumberStr` via ldflags).
2) Stage binary into payload (`generated/payload/<service>/bin/`).
3) Build a package `.tgz` with `globularcli pkg build` using the generated spec.
4) Publish package to the repository (auto-resolves repo address from etcd if not provided; uses cached token if present).
5) Set desired state via `globular services desired set <service> <version>`.
6) If service is `cluster_controller`, also set `/globular/system/controller-target-build` in etcd with version/build/checksum.
7) Record build number in `.build-numbers` and append to `.deploy-log`.

Inputs/assumptions:
- Specs are in `generated/specs/<service>_service.yaml`.
- Go package dir auto-detected under `golang/<service>/...`; binary name from spec `exec:` (defaults to `<service>_server`).
- Repository address auto-resolved from etcd `/globular/services/*/config` entries matching `repository.PackageRepository` (TLS certs under `/var/lib/globular/pki/...`).
- Uses `globularcli` from stage (`golang/tools/stage/...`) or system `globular`.

Common flags:
- `--comment "..."` annotate deploy log.
- `--version` override version (default 0.0.2).
- `--repository` override repo host:port.

Quick example:
```bash
./deploy-service.sh cluster_controller --comment "etcd state persistence"
# repo auto-resolves; build number increments; desired state set
```

Notes:
- Ports and desired state are dynamic; controller rollout picks up new artifact automatically once published + desired set.
- If publish succeeds but verify warns (mesh auth/manifest), script treats it as success if bundle_id present. Check logs if unsure.
