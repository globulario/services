# Build & Package Pipeline (generateCode.sh / build-all-packages.sh)

## generateCode.sh (services repo)
Purpose: regenerate code from protos, clean TS outputs, produce authz metadata, sync workflows, build services and installer.
- Generates Go gRPC stubs for all listed protos into `golang/*/*pb`.
- Generates TypeScript grpc-web bindings for selected protos into `typescript/<svc>/`; cleans `globular_auth_pb` requires for Vite compatibility and copies cleaned files to `typescript/dist/`.
- Builds a combined proto descriptor (`generated/policy/descriptor.pb`) and runs `authzgen` to produce permissions/roles (`generated/policy`).
- Updates `golang/go.mod` to latest `globular-installer` and tidies.
- Builds installer binary (if `../globular-installer` exists) to `globular-installer/bin/globular-installer`.
- Syncs workflow YAMLs from `golang/workflow/definitions/` into `generated/payload/workflow/definitions/`.
- Builds Go services via `golang/build/build-services.sh` and the CLI binary `golang/globularcli/globularcli`.

Run when: protos change, authz annotations change, workflows updated, or before packaging.

## build-all-packages.sh (services repo)
Purpose: rebuild infra + service packages and stage them for installer/repo publish.
Steps:
1) Copy staged Go binaries (gateway, xds, globularcli, mcp) from `golang/tools/stage/linux-amd64/usr/local/bin` into `packages/bin/`.
2) Read versions from specs in `packages/specs/*.yaml`; download/verify third-party binaries (envoy, etcd/etcdctl, prometheus/promtool, alertmanager/amtool, node_exporter, sidekick, restic, rclone, yt-dlp, ffmpeg, sha256sum/coreutils) into `packages/bin/`.
3) Build infrastructure packages via `packages/build.sh --out packages/dist`.
4) Generate service specs (`golang/globularcli/tools/specgen/specgen.sh`) and build service packages (`golang/globularcli/tools/pkggen/pkggen.sh`) into `packages/dist/`.
5) Publish packages to repository (`globularcli pkg publish`) if repo reachable (`GLOBULAR_REPO_ADDR`, default localhost:443).
6) Copy all packages to installer assets dir `globular-installer/internal/assets/packages/`.
Outputs:
- Packages `.tgz` in `packages/dist/` and installer assets.
- Counts of infra vs service packages; summary listing.

Run when: producing release artifacts or refreshing installer bundle after code changes.
