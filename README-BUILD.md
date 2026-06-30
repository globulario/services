# Building All Packages

## Quick Start

To rebuild all infrastructure and service packages with updated binaries:

```bash
./build-all-packages.sh
```

This script will:
1. Prepare infrastructure binaries (gateway, xds, globularcli)
2. Download/verify envoy 1.35.3 and etcd 3.5.14
3. Build all infrastructure packages (envoy, etcd, gateway, xds, minio)
4. Build all 24 service packages with smart TLS discovery
5. Copy package artifacts to the installer's embedded package mirror/output inputs

## What Gets Built

### Infrastructure (7 packages)
- envoy 1.35.3
- etcd 3.5.14
- gateway
- xds
- minio
- globular-cli
- mc-cmd

### Services (24 packages)
All services with smart TLS certificate discovery:
- authentication, blog, catalog, cluster-controller, conversation
- discovery, dns, echo, event, file, ldap, log
- media, monitoring, node-agent, persistence
- rbac, repository, resource, search, sql
- storage, title, torrent

## Prerequisites

- All service binaries built (in golang/tools/stage/linux-amd64/usr/local/bin/)
- gateway_server and xds_server in Globular/.bin/
- Internet connection (for downloading envoy/etcd if needed)

## Manual Steps

If you only want to build specific components:

### Build Service Binaries
```bash
cd golang/<service>/<service>_server
go build -o ../../../tools/stage/linux-amd64/usr/local/bin/<service>_server .
```

### Generate Service Specs
```bash
bash golang/globularcli/tools/specgen/specgen.sh \
    golang/tools/stage/linux-amd64/usr/local/bin \
    generated
```

### Build Service Packages
```bash
bash golang/globularcli/tools/pkggen/pkggen.sh \
    --globular golang/tools/stage/linux-amd64/usr/local/bin/globularcli \
    --bin-dir golang/tools/stage/linux-amd64/usr/local/bin \
    --gen-root generated \
    --out generated/packages \
    --version 0.0.1
```

### Full Regenerate-And-Release Inputs
```bash
bash scripts/regenerate-release-inputs.sh --version 1.2.257
```

This wipes only the known release-input subtrees under `services/generated`,
rebuilds generated policy/spec/template inputs, and writes a freshness manifest
used by `scripts/build-release.sh --full-regenerate`.

### Build Infrastructure Packages
```bash
cd ../packages
./build.sh
```

## Output

Artifacts and staging locations:
- Package artifact output: `packages/dist/*.tgz`
- Generated service workspace output: `services/generated/*.tgz`
- Installer embedded package mirror/input: `globular-installer/internal/assets/packages/*.tgz`

## Versions

Default versions (can be changed in build-all-packages.sh):
- Envoy: 1.35.3
- etcd: 3.5.14
- All packages: 0.0.1
